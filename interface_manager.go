package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"stableinterfaces/syncx"
	"time"
)

type (
	InterfaceManager struct {
		hostID    string
		hosts     []string
		numShards uint32
		myShards  []uint32

		// shardsToHost is a mapping of shards to hosts
		shardsToHost syncx.Map[uint32, string]

		// Default GetInstanceID
		HashingFunction func(string) (string, error)

		instances syncx.Map[string, *StableInterface]

		interfaceSpawner InterfaceSpawner

		alarmManager          AlarmManager
		alarmCheckInterval    *time.Duration
		internalAlarmManagers syncx.Map[uint32, *internalAlarmManager]
		getAlarmsTimeout      *time.Duration
		onAlarmTimeout        *time.Duration
		modifyAlarmTimeout    *time.Duration
		alarmRetryBackoff     *time.Duration
		maxAlarmAttempts      int

		logger Logger
	}

	// InterfaceSpawner should return a pointer to a StableInterface
	InterfaceSpawner func(internalID string) StableInterface
)

const (
	DefaultAlarmCheckInterval = time.Millisecond * 150
	DefaultAlarmCheckTimeout  = time.Second * 5
	DefaultOnAlarmTimeout     = time.Second * 5
	DefaultModifyAlarmTimeout = time.Second * 5
	DefaultMaxAlarmAttempt    = 5
	DefaultMaxAlarmBackoff    = time.Millisecond * 100
)

var (
	ErrTooFewShards        = errors.New("too few shards, the number of shards must be >= number of hosts")
	ErrHostDoesNotOwnShard = errors.New("this host does not own the shard that instance belongs to, try checking InterfaceManager.GetHostForID() and whether that equals InterfaceManager.GetHostID()")
	ErrReturnedNilInstance = errors.New("InterfaceSpawner returned a nil instance")
	ErrShardNotFound       = errors.New("shard not found, there must be a bug")
	ErrInstanceNotFound    = errors.New("instance not found")
	ErrInstanceOnConnect   = errors.New("instance returned error during OnConnect")
)

// NewInterfaceManager makes a new interface.
// InterfaceSpawner should return a pointer to a StableInterface
func NewInterfaceManager(hostID string, hostExpansion string, numShards uint32, interfaceSpawner InterfaceSpawner, opts ...InterfaceManagerOption) (*InterfaceManager, error) {
	im := &InterfaceManager{
		hostID:           hostID,
		shardsToHost:     syncx.NewMap[uint32, string](),
		HashingFunction:  TruncatedSHA256, // default function
		interfaceSpawner: interfaceSpawner,
		numShards:        numShards,
		myShards:         []uint32{},
		logger:           &DefaultLogger{},
		maxAlarmAttempts: DefaultMaxAlarmAttempt,
	}

	var err error
	if hostID == hostExpansion {
		// Single host
		im.hosts = []string{hostID}
	} else {
		im.hosts, err = expandRangePattern(hostExpansion)
	}
	if err != nil {
		return nil, fmt.Errorf("error expanding hosts to list, did the notation look like `host-{x..y}`?: %w", err)
	}

	if numShards < uint32(len(im.hosts)) {
		return nil, ErrTooFewShards
	}

	// Iterate over number of shards and map hosts
	for i := 0; i < int(numShards); i++ {
		modHost := im.hosts[i%len(im.hosts)]
		im.shardsToHost.Store(uint32(i), modHost)
		if modHost == hostID {
			im.myShards = append(im.myShards, uint32(i))
		}
	}

	// Handle options
	for _, opt := range opts {
		err = opt(im)
		if err != nil {
			return nil, fmt.Errorf("error setting option: %w", err)
		}
	}

	// If we are using an alarm, do that
	if im.alarmManager != nil {
		for _, shard := range im.myShards {
			alarmManager := newInternalAlarmManager(shard, im)
			im.internalAlarmManagers.Store(shard, alarmManager)
			go alarmManager.launchPollAlarms()
		}
	}

	return im, nil
}

// GetHostID returns the host ID
func (im *InterfaceManager) GetHostID() string {
	return im.hostID
}

// GetHostForID returns the owning host ID of the instance ID
func (im *InterfaceManager) GetHostForID(id string) (string, error) {
	internalID, err := im.HashingFunction(id)
	if err != nil {
		return "", fmt.Errorf("error in HashingFunction: %w", err)
	}

	shard := instanceInternalIDToShard(internalID, int(im.numShards))
	host, found := im.shardsToHost.Load(shard)
	if !found {
		return "", ErrShardNotFound
	}

	return host, nil
}

func (im *InterfaceManager) verifyHostOwnership(id string) error {
	// Hash ID and make sure we own the shard
	hostForID, err := im.GetHostForID(id)
	if err != nil {
		return fmt.Errorf("error in GetHostForID: %w", err)
	}

	if hostForID != im.hostID {
		return ErrHostDoesNotOwnShard
	}
	return nil
}

func (im *InterfaceManager) getOrMakeInstance(internalID string) (*StableInterface, error) {
	instance, exists := im.instances.Load(internalID)
	if !exists {
		instance = ptr(im.interfaceSpawner(internalID))
		if instance == nil {
			return nil, ErrReturnedNilInstance
		}
		im.instances.Store(internalID, instance)
	}

	return instance, nil
}

func (im *InterfaceManager) destroyInstanceIfExists(internalID string) {
	im.instances.Delete(internalID)
}

func (im *InterfaceManager) GetInternalID(id string) (string, error) {
	internalID, err := im.HashingFunction(id)
	if err != nil {
		return "", fmt.Errorf("error in HashingFunction: %w", err)
	}

	return internalID, nil
}

// Request invokes a request-response like interaction with an instance of a stable interface.
// It will create the interface if it is not started or has yet to exist.
func (im *InterfaceManager) Request(ctx context.Context, id string, payload any) (any, error) {
	// Hash ID and make sure we own the shard
	if err := im.verifyHostOwnership(id); err != nil {
		return nil, fmt.Errorf("error in verifyHostOwnership: %w", err)
	}

	internalID, err := im.GetInternalID(id)
	if err != nil {
		return nil, fmt.Errorf("error in GetInternalID: %w", err)
	}

	instance, err := im.getOrMakeInstance(internalID)
	if err != nil {
		return nil, fmt.Errorf("error in getOrMakeInstance: %w", err)
	}

	response, err := (*instance).OnRequest(im.makeInterfaceContext(internalID, ctx), payload)
	if err != nil {
		return nil, wrapStableInterfaceHandlerError(err)
	}

	return response, nil
}

func (im *InterfaceManager) makeInterfaceContext(internalID string, ctx context.Context) InterfaceContext {
	return InterfaceContext{
		Context:            ctx,
		Shard:              instanceInternalIDToShard(internalID, int(im.numShards)),
		interfaceManager:   im,
		InternalInstanceID: internalID,
	}
}

// ShutdownInstance turns off an instance of it is running.
// Returns ErrInstanceNotFound if not running.
// func (im *InterfaceManager) ShutdownInstance(ctx context.Context, id string) error {
// 	// TODO
// }

// Shutdown shuts down the entire interface manager
// func (im *InterfaceManager) Shutdown(ctx context.Context) error {
// 	// TODO
//  // TODO: Shutdown all alarm managers
//  // TODO: Shutdown all connections on all instances
// }

// Connect connects to an instance for persistent duplex communication.
// The recvHandler parameter will receive (blocking) messages when an instance sends a message to that, so you probably want to launch a goroutine for concurrency.
// The returned MsgHandler can be invoked when you want to send a message to the instance
func (im *InterfaceManager) Connect(ctx context.Context, id string, meta map[string]any) (*InterfaceConnection, error) {
	if err := im.verifyHostOwnership(id); err != nil {
		return nil, fmt.Errorf("error in verifyHostOwnership: %w", err)
	}

	internalID, err := im.GetInternalID(id)
	if err != nil {
		return nil, fmt.Errorf("error in GetInternalID: %w", err)
	}

	instance, err := im.getOrMakeInstance(internalID)
	if err != nil {
		return nil, fmt.Errorf("error in getOrMakeInstance: %w", err)
	}

	connID := genRandomID("")
	incoming := newIncomingConnection(internalID, connID, meta)
	doneChan := make(chan any, 1)
	go func() {
		(*instance).OnConnect(im.makeInterfaceContext(internalID, ctx), *incoming)
		doneChan <- nil
	}()

	var connPair *connectionPair
	select {
	case connPair = <-incoming.acceptChan:
		break
	case <-doneChan:
		break
	case reason := <-incoming.rejectChan:
		return nil, fmt.Errorf("%w, reason: %w", ErrIncomingConnectionRejected, reason)
	case <-ctx.Done():
		return nil, fmt.Errorf("context done: %w", ctx.Err())
	}

	if connPair == nil {
		// they did not accept or reject
		return nil, ErrIncomingConnectionNotHandled
	}

	return &connPair.ManagerSide, nil
}

func (im *InterfaceManager) getStoredAlarms(shard uint32) ([]StoredAlarm, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deref(im.getAlarmsTimeout, DefaultAlarmCheckTimeout))
	defer cancel()
	return im.alarmManager.GetNextAlarms(ctx, shard)
}

func (im *InterfaceManager) onAlarm(ctx context.Context, alarm StoredAlarm) error {
	instance, err := im.getOrMakeInstance(alarm.InterfaceInstanceInternalID)
	if err != nil {
		return fmt.Errorf("error in getOrMakeInstance: %w", err)
	}

	alarmInstance, ok := (*instance).(StableInterfaceWithAlarm)
	if !ok {
		return fmt.Errorf("%w -- this is a bug, please report", ErrInterfaceNotWithAlarm)
	}

	err = alarmInstance.OnAlarm(im.makeInterfaceContext(alarm.InterfaceInstanceInternalID, ctx), alarm.ID, alarm.Meta)
	if err != nil {
		return fmt.Errorf("error in OnAlarm: %w", err)
	}

	return nil
}
