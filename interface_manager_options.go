package stableinterfaces

import (
	"errors"
	"fmt"
	"time"
)

type (
	InterfaceManagerOption func(manager *InterfaceManager) error
)

var (
	ErrInterfaceNotWithAlarm        = errors.New("the interface did not implement StableInterfaceWithAlarm, check if you are missing the OnAlarm handler")
	ErrInterfaceManagerNotWithAlarm = errors.New("the interface manager does not have WithAlarm()")
)

const (
	withAlarmTestInstanceID = "withAlarmTestInstanceID"
)

func WithAlarm(alarmManager AlarmManager) InterfaceManagerOption {
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
		manager.alarmManager = alarmManager
		return nil
	}
}

func WithAlarmCheckInterval(duration time.Duration) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.alarmCheckInterval = &duration
		return nil
	}
}

func WithGetAlarmsTimeout(duration time.Duration) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.getAlarmsTimeout = &duration
		return nil
	}
}

func WithOnAlarmTimeout(duration time.Duration) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.onAlarmTimeout = &duration
		return nil
	}
}

func WithModifyAlarmTimeout(duration time.Duration) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.modifyAlarmTimeout = &duration
		return nil
	}
}

func WithAlarmRetryBackoff(duration time.Duration) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.alarmRetryBackoff = &duration
		return nil
	}
}

func WithMaxAlarmAttempts(attempts int) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.maxAlarmAttempts = attempts
		return nil
	}
}

func WithLogger(logger Logger) InterfaceManagerOption {
	return func(manager *InterfaceManager) error {
		manager.logger = logger
		return nil
	}
}
