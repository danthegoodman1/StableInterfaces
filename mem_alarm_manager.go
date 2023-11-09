package stableinterfaces

import (
	"context"
	"github.com/tidwall/btree"
	"stableinterfaces/syncx"
)

type (
	// MemAlarmManager is a memory-only implementation of AlarmManager.
	// Thread-safe, made for testing only!
	MemAlarmManager struct {
		alarms syncx.Map[uint32, *btree.Map[string, StoredAlarm]]
	}
)

func NewMemAlarmManager() MemAlarmManager {
	return MemAlarmManager{
		alarms: syncx.NewMap[uint32, *btree.Map[string, StoredAlarm]](),
	}
}

func (m *MemAlarmManager) GetNextAlarms(_ context.Context, shard uint32) ([]StoredAlarm, error) {
	// Verify it exists
	tree, exists := m.alarms.Load(shard)
	if !exists {
		return nil, nil
	}

	var alarms []StoredAlarm

	// Get sorted alarms
	tree.Scan(func(_ string, alarm StoredAlarm) bool {
		alarms = append(alarms, alarm)
		return true
	})

	return alarms, nil
}

func (m *MemAlarmManager) SetAlarm(_ context.Context, shard uint32, alarm StoredAlarm) error {
	tree, exists := m.alarms.Load(shard)
	if !exists {
		tree = &btree.Map[string, StoredAlarm]{}
		m.alarms.Store(shard, tree)
	}

	tree.Set(alarm.AlarmID, alarm)
	return nil
}

func (m *MemAlarmManager) MarkAlarmDone(_ context.Context, shard uint32, alarmID string, _ AlarmDoneReason) error {
	// We're just going to delete it
	tree, exists := m.alarms.Load(shard)
	if !exists {
		return nil
	}

	tree.Delete(alarmID)
	return nil
}
