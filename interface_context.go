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

	stored := StoredAlarm{
		ID:      id,
		Meta:    meta,
		Created: time.Now(),
		Fires:   at,
	}
	err := ic.interfaceManager.alarmManager.SetAlarm(ctx, ic.Shard, stored)
	if err != nil {
		return fmt.Errorf("error in SetAlarm: %w", err)
	}
	iam, found := ic.interfaceManager.internalAlarmManagers.Load(ic.Shard)
	if !found {
		return ErrInternalAlarmManagerNotFound
	}

	iam.SetAlarm(stored)

	return nil
}

// TODO
// func (ic *InterfaceContext) ListAlarms(ctx context.Context, alarm StoredAlarm, offset string) error {}
// func (ic *InterfaceContext) CancelAlarm(ctx context.Context, alarm StoredAlarm) error {}