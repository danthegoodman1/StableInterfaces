package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"github.com/danthegoodman1/stableinterfaces/syncx"
	"sync"
	"sync/atomic"
)

type (
	instanceManager struct {
		stableInterface  *StableInterface
		connections      syncx.Map[string, *connectionPair]
		closing          *atomic.Bool
		handlingWG       *sync.WaitGroup
		internalID       string
		interfaceManager *InterfaceManager
	}
)

var (
	ErrInstanceIsClosing = errors.New("instance is closing")
)

func newInstanceManager(im *InterfaceManager, internalID string, stableInterface *StableInterface) *instanceManager {
	return &instanceManager{
		stableInterface:  stableInterface,
		connections:      syncx.NewMap[string, *connectionPair](),
		closing:          &atomic.Bool{},
		handlingWG:       &sync.WaitGroup{},
		internalID:       internalID,
		interfaceManager: im,
	}
}

func (im *instanceManager) makeInterfaceContext(ctx context.Context) InterfaceContext {
	return InterfaceContext{
		Context:            ctx,
		Shard:              instanceInternalIDToShard(im.internalID, int(im.interfaceManager.numShards)),
		interfaceManager:   im.interfaceManager,
		instanceManager:    im,
		InternalInstanceID: im.internalID,
	}
}

func (im *instanceManager) Request(ctx context.Context, payload any) (any, error) {
	response, err := (*im.stableInterface).OnRequest(im.makeInterfaceContext(ctx), payload)
	if err != nil {
		return nil, wrapStableInterfaceHandlerError(err)
	}

	return response, nil
}

func (im *instanceManager) Connect(ctx context.Context, meta map[string]any) (*InterfaceConnection, error) {
	connID := genRandomID("")
	incoming := newIncomingConnection(im.internalID, connID, meta)
	doneChan := make(chan any, 1)
	go func() {
		(*im.stableInterface).OnConnect(im.makeInterfaceContext(ctx), *incoming)
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

func (im *instanceManager) Alarm(ctx context.Context, attempt int, id string, meta map[string]any) error {
	if im.closing.Load() {
		return ErrInstanceIsClosing
	}

	alarmInstance, ok := (*im.stableInterface).(StableInterfaceWithAlarm)
	if !ok {
		return fmt.Errorf("%w -- this is a bug, please report", ErrInterfaceNotWithAlarm)
	}

	err := alarmInstance.OnAlarm(InterfaceContextWithAttempt{
		InterfaceContext: im.makeInterfaceContext(ctx),
		Attempt:          attempt,
	}, id, meta)
	if err != nil {
		return fmt.Errorf("error in Alarm: %w", err)
	}
	return nil
}
