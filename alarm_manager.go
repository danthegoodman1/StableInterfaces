package stableinterfaces

import (
	"context"
	"fmt"
	"time"
)

const (
	AlarmSuccessful         AlarmDoneReason = "successful"
	AlarmMaxRetriesExceeded AlarmDoneReason = "max retries exceeded"
)

var (
	alarmCheckTicker *time.Ticker
)

type (
	// AlarmManager manages alarms per-shard
	AlarmManager interface {
		// GetNextAlarms will return the next N alarms, sorted by time, whether they are firing or not.
		// StableInterfaces will cache alarms in memory for instant firing, and poll for more when it runs out.
		// It is expected that all persisted alarms for a shard are returned from this function.
		// Anything not returned will be ignored.
		GetNextAlarms(ctx context.Context, shard uint32) ([]StoredAlarm, error)
		// SetAlarm should create or update an alarm
		SetAlarm(ctx context.Context, shard uint32, alarm StoredAlarm) error
		// MarkAlarmDone marks a handled alarm done (for any reason it should stop firing)
		MarkAlarmDone(ctx context.Context, shard uint32, alarmID string, reason AlarmDoneReason) error
	}

	StoredAlarm struct {
		AlarmID, InterfaceInstanceInternalID string
		Meta                                 map[string]any
		Created, Fires                       time.Time
	}

	AlarmDoneReason string
)

// launchPollAlarms should be launched in a goroutine, start polling for alarms
func (im *InterfaceManager) launchPollAlarms(shard uint32, stopChan chan any) {
	// Get stored alarms
	storedAlarms, err := im.getStoredAlarms(shard)
	if err != nil {
		im.logger.Fatal(fmt.Sprintf("failed to get stored alarms for shard %d", shard), err)
	}

	ticker := time.NewTicker(deref(im.alarmCheckInterval, DefaultAlarmCheckInterval))
	select {
	case <-ticker.C:
		// Check for the next alarm

	case <-stopChan:
		ticker.Stop()
		return
	}
}

func (im *InterfaceManager) getStoredAlarms(shard uint32) ([]StoredAlarm, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deref(im.getAlarmsTimeout, DefaultAlarmCheckTimeout))
	defer cancel()
	return im.alarmManager.GetNextAlarms(ctx, shard)
}
