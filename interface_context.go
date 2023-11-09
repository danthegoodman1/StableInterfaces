package stableinterfaces

import (
	"context"
	"fmt"
	"time"
)

type (
	InterfaceContext struct {
		Context context.Context

		Shard uint32

		interfaceManager *InterfaceManager
	}
)

func (ic *InterfaceContext) SetAlarm(ctx context.Context, id string, meta map[string]any, at time.Time) error {
	if ic.interfaceManager.alarmManager == nil {
		return ErrInterfaceManagerNotWithAlarm
	}

	err := ic.interfaceManager.alarmManager.SetAlarm(ctx, ic.Shard, StoredAlarm{
		AlarmID: id,
		Meta:    meta,
		Created: time.Now(),
		Fires:   at,
	})
	if err != nil {
		return fmt.Errorf("error in SetAlarm: %w", err)
	}
	return nil
}

// TODO
// func (ic *InterfaceContext) ListAlarms(ctx context.Context, alarm StoredAlarm, offset string) error {}
// func (ic *InterfaceContext) CancelAlarm(ctx context.Context, alarm StoredAlarm) error {}
