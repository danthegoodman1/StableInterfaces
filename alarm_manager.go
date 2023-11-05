package stableinterfaces

import (
	"context"
	"time"
)

const (
	AlarmSuccessful         AlarmDoneReason = "successful"
	AlarmMaxRetriesExceeded AlarmDoneReason = "max retries exceeded"
)

type (
	// AlarmManager manages alarms per-shard
	AlarmManager interface {
		// GetNextAlarms will return the next N alarms, sorted by time, whether they are firing or not.
		// StableInterfaces will cache alarms in memory for instant firing, and poll for more when it runs out.
		GetNextAlarms(ctx context.Context, shard uint32) ([]Alarm, error)
		// SetAlarm should create or update an alarm
		SetAlarm(ctx context.Context, shard uint32, alarm Alarm) error
		// MarkAlarmDone marks a handled alarm done (for any reason it should stop firing)
		MarkAlarmDone(ctx context.Context, shard uint32, alarmID string, reason AlarmDoneReason) error
	}

	Alarm struct {
		ID             string
		Meta           map[string]any
		Created, Fires time.Time
	}

	AlarmDoneReason string
)
