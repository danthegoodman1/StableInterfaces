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

}

func (iam *internalAlarmManager) DeleteAlarm(id string) {

}
