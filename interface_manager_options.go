package stableinterfaces

import (
	"errors"
	"fmt"
)

type (
	InterfaceManagerOption func(manager *InterfaceManager) error
)

var (
	ErrInterfaceNotWithAlarm = errors.New("the interface did not implement StableInterfaceWithAlarm, check if you are missing the OnAlarm handler")
)

const (
	withAlarmTestInstanceID = "withAlarmTestInstanceID"
)

func WithAlarm(alarmManager *AlarmManager) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		// Spawn a test interface (this doesn't even belong on this host)
		testInterface, err := manager.getOrMakeInstance(withAlarmTestInstanceID)
		if err != nil {
			return fmt.Errorf("error in getOrMakeInstance: %w", err)
		}
		defer manager.destroyInstanceIfExists(withAlarmTestInstanceID)
		if _, ok := (*testInterface).(StableInterfaceWithAlarm); !ok {
			return ErrInterfaceNotWithAlarm
		}
		// We are good otherwise
		manager.withAlarm = true
		return nil
	}
}
