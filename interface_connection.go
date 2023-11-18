package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

		// OnClose is called when the connection is closed from the interface-side. This is non-blocking.
		onClose   []func()
		onCloseMu *sync.Mutex

		// Optimistic shuttingDown
		closed *atomic.Bool

		// OnRecv is called when the interface send a message to the connection.
		// This function is executed non-blocking (launched in a goroutine)
		OnRecv func(payload any)

		sendChan, recvChan, closedChan chan any
	}

	connectionPair struct {
		ID                         string
		InterfaceSide, ManagerSide InterfaceConnection
		// Need channels to prevent panic on sending to closed channel, but not blocking
		// because connection listener is already in goroutine
		closedChan chan any
	}
)

func launchConnectionPairListener(cp *connectionPair) {
	for {
		select {
		case received := <-cp.ManagerSide.recvChan:
			if cp.ManagerSide.OnRecv != nil {
				go cp.ManagerSide.OnRecv(received)
			}
		case received := <-cp.InterfaceSide.recvChan:
			if cp.InterfaceSide.OnRecv != nil {
				go cp.InterfaceSide.OnRecv(received)
			}
		case <-cp.closedChan:
			// Mark both sides closed
			go cp.InterfaceSide.close()
			go cp.ManagerSide.close()
			return
		}
	}
}

func (cp *connectionPair) Close(ctx context.Context) error {
	// They're the same `closed`
	if !cp.InterfaceSide.closed.CompareAndSwap(false, true) {
		return ErrConnectionClosed
	}
	select {
	case cp.closedChan <- nil:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the connection
func (ic *InterfaceConnection) Close() error {
	if !ic.closed.CompareAndSwap(false, true) {
		return ErrConnectionClosed
	}

	// Close the loop
	ic.closedChan <- nil

	return nil
}

func (ic *InterfaceConnection) close() {
	ic.closed.Store(true) // in case we haven't already done this (e.g. invoked from outside)

	ic.onCloseMu.Lock()
	defer ic.onCloseMu.Unlock()
	for _, onClose := range ic.onClose {
		onClose()
	}
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

func (ic *InterfaceConnection) AddOnCloseListener(f func()) {
	ic.onCloseMu.Lock()
	defer ic.onCloseMu.Unlock()
	ic.onClose = append(ic.onClose, f)
}
