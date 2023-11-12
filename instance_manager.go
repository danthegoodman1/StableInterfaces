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
		shuttingDown     *atomic.Bool
		handlingWG       *sync.WaitGroup
		internalID       string
		interfaceManager *InterfaceManager
	}
)

var (
	ErrInstanceIsShuttingDown = errors.New("instance is shuttingDown")
)

func newInstanceManager(im *InterfaceManager, internalID string, stableInterface *StableInterface) *instanceManager {
	return &instanceManager{
		stableInterface:  stableInterface,
		connections:      syncx.NewMap[string, *connectionPair](),
		shuttingDown:     &atomic.Bool{},
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
	connPair.ManagerSide.AddOnCloseListener(func() {
		// Remove the connection from the map
		im.connections.Delete(connID)
	})

	im.connections.Store(connID, connPair)

	return &connPair.ManagerSide, nil
}

func (im *instanceManager) Alarm(ctx context.Context, attempt int, id string, meta map[string]any) error {
	if im.shuttingDown.Load() {
		return ErrInstanceIsShuttingDown
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

func (im *instanceManager) Shutdown(ctx context.Context) error {
	if !im.shuttingDown.CompareAndSwap(false, true) {
		return ErrInstanceIsShuttingDown
	}

	doneChan := make(chan error)

	go func(edc chan error) {
		im.shutdown(ctx)
		doneChan <- nil
	}(doneChan)

	select {
	case err := <-doneChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (im *instanceManager) shutdown(ctx context.Context) {
	im.interfaceManager.logger.Debug(fmt.Sprintf("shutting down instance %s", im.internalID))
	// Disconnect all connections
	im.connections.Range(func(connID string, conn *connectionPair) bool {
		err := conn.Close(ctx)
		if err != nil {
			fmt.Println(err, connID)
			im.interfaceManager.logger.Error(fmt.Sprintf("failed to shutdown connection %s", conn.ID), err)
		}
		return true
	})

	im.handlingWG.Wait()
	return
}
