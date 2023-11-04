package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
)

type (
	InterfaceManager struct {
		hostID string
		hosts  []string

		// shardsToHost is a mapping of shards to hosts
		shardsToHost map[uint32]string

		// Default GetInstanceID
		HashingFunction func(string) ([]byte, error)

		instances map[string]*StableInterface

		interfaceSpawner InterfaceSpawner
	}

	// InterfaceSpawner should return a pointer to a StableInterface
	InterfaceSpawner func(internalID string) StableInterface
)

var (
	ErrTooFewShards        = errors.New("too few shards, the number of shards must be >= number of hosts")
	ErrHostDoesNotOwnShard = errors.New("this host does not own the shard that instance belongs to, try checking InterfaceManager.GetHostForID() and whether that equals InterfaceManager.GetHostID()")
	ErrReturnedNilInstance = errors.New("InterfaceSpawner returned a nil instance")
)

// NewInterfaceManager makes a new interface.
// InterfaceSpawner should return a pointer to a StableInterface
func NewInterfaceManager(hostID string, hostExpansion string, numShards uint32, interfaceSpawner InterfaceSpawner) (*InterfaceManager, error) {
	im := &InterfaceManager{
		hostID:           hostID,
		shardsToHost:     map[uint32]string{},
		HashingFunction:  TruncatedSHA256, // default function
		interfaceSpawner: interfaceSpawner,
	}

	var err error
	im.hosts, err = expandRangePattern(hostExpansion)
	if err != nil {
		return nil, fmt.Errorf("error expanding hosts to list, did the notation look like `host-{x..y}`?: %w", err)
	}

	if numShards < uint32(len(im.hosts)) {
		return nil, ErrTooFewShards
	}

	// Iterate over number of shards and map hosts
	for i := 0; i < int(numShards); i++ {
		im.shardsToHost[uint32(i)] = im.hosts[i%len(im.hosts)]
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

	shard := instanceInternalIDToShard(internalID, len(im.shardsToHost))

	return im.shardsToHost[shard], nil
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
	instance, exists := im.instances[internalID]
	if !exists {
		instance = ptr(im.interfaceSpawner(internalID))
		if instance == nil {
			return nil, ErrReturnedNilInstance
		}
	}

	return instance, nil
}

func (im *InterfaceManager) GetInternalID(id string) (string, error) {
	internalID, err := im.HashingFunction(id)
	if err != nil {
		return "", fmt.Errorf("error in HashingFunction: %w", err)
	}

	return string(internalID), nil
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

	response, err := (*instance).OnRequest(ctx, payload)
	if err != nil {
		return nil, wrapStableInterfaceHandlerError(err)
	}

	return response, nil
}
