package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidwall/btree"
	"sync"
	"sync/atomic"
	"time"
)

type (
	internalAlarmManager struct {
		InterfaceManager        *InterfaceManager
		Shard                   uint32
		StopChan                chan any
		idIndex, alarmIndex     btree.Map[string, *wrappedStoredAlarm]
		idIndexMu, alarmIndexMu *sync.Mutex
		alarmHandlerTimeout     *time.Duration

		closed atomic.Bool
	}

	wrappedStoredAlarm struct {
		StoredAlarm StoredAlarm
		Attempt     int
	}
)

var (
	ErrInternalAlarmManagerNotFound = errors.New("internal alarm manager not found, this is a bug and should never happen, please report")
)

func newInternalAlarmManager(shard uint32, im *InterfaceManager) *internalAlarmManager {
	return &internalAlarmManager{
		InterfaceManager: im,
		Shard:            shard,
		StopChan:         make(chan any, 1),
		// Lookup by ID
		idIndex:   btree.Map[string, *wrappedStoredAlarm]{},
		idIndexMu: &sync.Mutex{},
		// Ordered by Fires, ID
		alarmIndex:   btree.Map[string, *wrappedStoredAlarm]{},
		alarmIndexMu: &sync.Mutex{},
		closed:       atomic.Bool{},
	}
}

// launchPollAlarms should be launched in a goroutine, start polling for alarms
func (iam *internalAlarmManager) launchPollAlarms() {
	// Get stored alarms
	storedAlarms, err := iam.InterfaceManager.getStoredAlarms(iam.Shard)
	if err != nil {
		iam.InterfaceManager.logger.Fatal(fmt.Sprintf("failed to get stored alarms for shard %d", iam.Shard), err)
	}

	for _, storedAlarm := range storedAlarms {
		iam.idIndex.Set(storedAlarm.ID, &wrappedStoredAlarm{
			StoredAlarm: storedAlarm,
			Attempt:     0,
		})
		iam.alarmIndex.Set(formatAlarmIndexKey(storedAlarm.Fires, storedAlarm.ID), &wrappedStoredAlarm{
			StoredAlarm: storedAlarm,
			Attempt:     0,
		})
	}

	ticker := time.NewTicker(deref(iam.InterfaceManager.alarmCheckInterval, DefaultAlarmCheckInterval))
	for {
		select {
		case <-ticker.C:
			handledAlarm := true
			for handledAlarm {
				handledAlarm = iam.checkAlarms()
			}

		case <-iam.StopChan:
			ticker.Stop()
			return
		}
	}
}

func formatAlarmIndexKey(firesAt time.Time, id string) string {
	return fmt.Sprintf("%s::%s", fmt.Sprint(firesAt.UnixMilli()), id)
}

func (iam *internalAlarmManager) checkAlarms() bool {
	// Check for the next alarm
	nextAlarm := iam.getNextAlarm()
	if nextAlarm == nil {
		return false
	}
	iam.InterfaceManager.logger.Debug("got an alarm!")

	// Execute the alarm
	ctx, cancel := context.WithTimeout(context.Background(), deref(iam.InterfaceManager.onAlarmTimeout, DefaultOnAlarmTimeout))
	defer cancel()

	doneReason := AlarmSuccessful

	err := iam.InterfaceManager.onAlarm(ctx, *nextAlarm)
	if err != nil {
		// If shuttingDown, we will just delay and retry later
		if nextAlarm.Attempt <= iam.InterfaceManager.maxAlarmAttempts {
			// Increment the attempts, update the memory fires at, and retry
			iam.InterfaceManager.logger.Warn(fmt.Sprintf("alarm '%s' Alarm errored (Attempt %d), delaying", nextAlarm.StoredAlarm.ID, nextAlarm.Attempt), err)
			nextAlarm.Attempt++
			nextAlarm.StoredAlarm.Fires = nextAlarm.StoredAlarm.Fires.Add(deref(iam.InterfaceManager.alarmRetryBackoff, DefaultMaxAlarmBackoff) * time.Duration(nextAlarm.Attempt))
			iam.ReplaceAlarm(nextAlarm)
			return true
		}

		iam.InterfaceManager.logger.Error(fmt.Sprintf("alarm '%s' Alarm reached max backoff (Attempt %d), aborting", nextAlarm.StoredAlarm.ID, nextAlarm.Attempt), err)
		// We are done
		doneReason = AlarmMaxRetriesExceeded
	}

	// Remove from indexes
	iam.DeleteAlarm(nextAlarm.StoredAlarm.ID)

	// Remove from storage
	ctx, cancel = context.WithTimeout(context.Background(), deref(iam.InterfaceManager.modifyAlarmTimeout, DefaultModifyAlarmTimeout))
	defer cancel()

	err = iam.InterfaceManager.alarmManager.MarkAlarmDone(ctx, iam.Shard, nextAlarm.StoredAlarm.ID, doneReason)
	if err != nil {
		iam.InterfaceManager.logger.Error(fmt.Sprintf("failed to marl alarm '%s' successful", nextAlarm.StoredAlarm.ID), err)
	}
	return true
}

func (iam *internalAlarmManager) getNextAlarm() (nextAlarm *wrappedStoredAlarm) {
	iam.alarmIndexMu.Lock()
	defer iam.alarmIndexMu.Unlock()
	// Current id to compare
	nowID := formatAlarmIndexKey(time.Now(), "")
	iam.alarmIndex.Scan(func(key string, value *wrappedStoredAlarm) bool {
		// Only get one
		if key <= nowID {
			nextAlarm = value
			return false
		}
		return true
	})
	return
}

func (iam *internalAlarmManager) SetAlarm(alarm wrappedStoredAlarm) {
	iam.setIDIndex(alarm)
	iam.setAlarmIndex(alarm)
}

func (iam *internalAlarmManager) setIDIndex(alarm wrappedStoredAlarm) {
	iam.idIndexMu.Lock()
	defer iam.idIndexMu.Unlock()
	iam.idIndex.Set(alarm.StoredAlarm.ID, &alarm)
}

func (iam *internalAlarmManager) setAlarmIndex(alarm wrappedStoredAlarm) {
	iam.alarmIndexMu.Lock()
	defer iam.alarmIndexMu.Unlock()
	iam.alarmIndex.Set(formatAlarmIndexKey(alarm.StoredAlarm.Fires, alarm.StoredAlarm.ID), &alarm)
}

func (iam *internalAlarmManager) DeleteAlarm(alarmID string) {
	fires := iam.deleteIDIndex(alarmID)
	if fires != nil {
		iam.deleteAlarmIndex(alarmID, *fires)
	}
}

// deleteIDIndex returns the StoredAlarm.Fires time if found
func (iam *internalAlarmManager) deleteIDIndex(alarmID string) *time.Time {
	iam.idIndexMu.Lock()
	defer iam.idIndexMu.Unlock()
	alarm, found := iam.idIndex.Get(alarmID)
	if !found {
		return nil
	}
	iam.idIndex.Delete(alarm.StoredAlarm.ID)
	return &alarm.StoredAlarm.Fires
}

func (iam *internalAlarmManager) deleteAlarmIndex(alarmID string, fires time.Time) {
	iam.alarmIndexMu.Lock()
	defer iam.alarmIndexMu.Unlock()
	iam.alarmIndex.Delete(formatAlarmIndexKey(fires, alarmID))
}

func (iam *internalAlarmManager) ReplaceAlarm(alarm *wrappedStoredAlarm) {
	iam.DeleteAlarm(alarm.StoredAlarm.ID)
	iam.SetAlarm(*alarm)
}
