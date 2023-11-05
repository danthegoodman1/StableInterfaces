package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
)

const (
	managerSide interfaceConnectionSide = iota
	interfaceSide
)

type (
	interfaceConnectionSide int
	// InterfaceConnection is a persistent duplex connection to an interface.
	InterfaceConnection struct {
		ID   string
		side interfaceConnectionSide

		// OnClose is called when the connection is closed from the interface-side. This is blocking.
		OnClose func()

		// Optimistic closing
		closed *atomic.Bool

		// OnRecv is called when the interface send a message to the connection.
		// This function is blocking, so you probably want to launch a goroutine.
		OnRecv func(payload any)

		sendChan, recvChan chan any
		// Need channels to prevent panic on sending to closed channel, but not blocking
		// because connection listener is already in goroutine
		closedChan chan any
	}

	connectionPair struct {
		InterfaceSide, ManagerSide InterfaceConnection
	}
)

func launchInterfaceConnectionListener(ic *InterfaceConnection) {
	// Manager side listener
	for {
		select {
		case received, more := <-ic.recvChan:
			if !more {
				return
			}
			if ic.OnRecv != nil {
				ic.OnRecv(received)
			}
		case <-ic.closedChan:
			return
		}
	}
}

// Close closes the connection
func (ic *InterfaceConnection) Close() error {
	if !ic.closed.CompareAndSwap(false, true) {
		return ErrConnectionClosed
	}
	close(ic.sendChan)
	ic.closedChan <- nil
	return nil
}

// Send sends a message to a Stable Interface instance for processing via it's OnRecv handler (if it exists).
// The context is only for queueing sends to the channel, not actual OnRecv processing, as sending is blocking.
// So best to keep this relatively short and launch goroutines in OnRecv.
// In the event that the channel is closes after we've checked (and it was found open), the ctx will time out with
// the error fmt.Errorf("%w: %w", context.DeadlineExceeded, ErrConnectionClosed).
// if there are queued sends when Close() is called, some may make it through due to Go's select.
func (ic *InterfaceConnection) Send(ctx context.Context, payload any) error {
	if ic.closed.Load() {
		return ErrConnectionClosed
	}
	select {
	case ic.sendChan <- payload:
		break
	case <-ctx.Done():
		// check if we closed between check and send
		if ic.closed.Load() {
			return fmt.Errorf("%w: %w", ctx.Err(), ErrConnectionClosed)
		}
		return ctx.Err()
	}
	return nil
}
