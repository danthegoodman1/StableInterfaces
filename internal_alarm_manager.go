package stableinterfaces

import (
	"errors"
	"fmt"
	"github.com/tidwall/btree"
	"sync"
	"time"
)

type (
	internalAlarmManager struct {
		InterfaceManager        *InterfaceManager
		Shard                   uint32
		StopChan                chan any
		idIndex, alarmIndex     btree.Map[string, *StoredAlarm]
		idIndexMu, alarmIndexMu *sync.Mutex
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
		idIndex:   btree.Map[string, *StoredAlarm]{},
		idIndexMu: &sync.Mutex{},
		// Ordered by Fires, ID
		alarmIndex:   btree.Map[string, *StoredAlarm]{},
		alarmIndexMu: &sync.Mutex{},
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
		iam.idIndex.Set(storedAlarm.ID, &storedAlarm)
		iam.alarmIndex.Set(formatAlarmIndexKey(storedAlarm.Fires, storedAlarm.ID), &storedAlarm)
	}

	ticker := time.NewTicker(deref(iam.InterfaceManager.alarmCheckInterval, DefaultAlarmCheckInterval))
	select {
	case <-ticker.C:
		iam.checkAlarms()

	case <-iam.StopChan:
		ticker.Stop()
		return
	}
}

func formatAlarmIndexKey(firesAt time.Time, id string) string {
	return fmt.Sprintf("%s::%s", string(firesAt.UnixMilli()), id)
}

func (iam *internalAlarmManager) checkAlarms() {
	// TODO: Check for the next alarm
	// TODO: Remove from indexes
}

func (iam *internalAlarmManager) SetAlarm(alarm StoredAlarm) {
	iam.setIDIndex(alarm)
	iam.setAlarmIndex(alarm)
}

func (iam *internalAlarmManager) setIDIndex(alarm StoredAlarm) {
	iam.idIndexMu.Lock()
	defer iam.idIndexMu.Unlock()
	iam.idIndex.Set(alarm.ID, &alarm)
}

func (iam *internalAlarmManager) setAlarmIndex(alarm StoredAlarm) {
	iam.alarmIndexMu.Lock()
	defer iam.alarmIndexMu.Unlock()
	iam.alarmIndex.Set(formatAlarmIndexKey(alarm.Fires, alarm.ID), &alarm)
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
	iam.idIndex.Delete(alarm.ID)
	return &alarm.Fires
}

func (iam *internalAlarmManager) deleteAlarmIndex(alarmID string, fires time.Time) {
	iam.alarmIndexMu.Lock()
	defer iam.alarmIndexMu.Unlock()
	iam.alarmIndex.Delete(formatAlarmIndexKey(fires, alarmID))
}
