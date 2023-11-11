package stableinterfaces

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"time"
)

type (
	InterfaceContext struct {
		Context context.Context

		Shard uint32

		interfaceManager   *InterfaceManager
		InternalInstanceID string
	}

	InterfaceContextWithAttempt struct {
		InterfaceContext
		Attempt int
	}
)

func (ic *InterfaceContext) SetAlarm(ctx context.Context, id string, meta map[string]any, at time.Time) error {
	if ic.interfaceManager.alarmManager == nil {
		return ErrInterfaceManagerNotWithAlarm
	}

	stored := StoredAlarm{
		ID:                          id,
		Meta:                        meta,
		Created:                     time.Now(),
		Fires:                       at,
		InterfaceInstanceInternalID: ic.InternalInstanceID,
	}
	err := ic.interfaceManager.alarmManager.SetAlarm(ctx, ic.Shard, stored)
	if err != nil {
		return fmt.Errorf("error in SetAlarm: %w", err)
	}
	iam, found := ic.interfaceManager.internalAlarmManagers.Load(ic.Shard)
	if !found {
		return ErrInternalAlarmManagerNotFound
	}

	iam.SetAlarm(wrappedStoredAlarm{
		StoredAlarm: stored,
		Attempt:     0,
	})

	return nil
}

// TODO
// func (ic *InterfaceContext) ListAlarms(ctx context.Context, alarm StoredAlarm, offset string) error {}

func (ic *InterfaceContext) CancelAlarm(ctx context.Context, alarmID string) error {
	if ic.interfaceManager.alarmManager == nil {
		return ErrInterfaceManagerNotWithAlarm
	}

	// Run concurrently because we don't want in-mem to fire while we are editing
	// The AlarmManager can choose to implement retries/background,
	// and the interface can decide to panic
	g := errgroup.Group{}
	g.Go(func() error {
		// Delete from memory
		iam, found := ic.interfaceManager.internalAlarmManagers.Load(ic.Shard)
		if !found {
			return ErrInternalAlarmManagerNotFound
		}

		iam.DeleteAlarm(alarmID)
		return nil
	})
	g.Go(func() error {
		// Delete from storage
		err := ic.interfaceManager.alarmManager.MarkAlarmDone(ctx, ic.Shard, alarmID, AlarmCanceled)
		if err != nil {
			return fmt.Errorf("error in MarkAlarmDone: %w", err)
		}
		return nil
	})

	return g.Wait()
}
