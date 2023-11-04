package stableinterfaces

import (
	"errors"
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
		closed  atomic.Bool

		// OnRecv is called when the interface send a message to the connection.
		// This function is blocking, so you probably want to launch a goroutine.
		OnRecv func(payload any)

		sendChan, recvChan chan any
	}

	connectionPair struct {
		InterfaceSide, ManagerSide InterfaceConnection
	}
)

func launchInterfaceConnectionListener(ic *InterfaceConnection) {
	// Manager side listener
	for {
		received, more := <-ic.recvChan
		if !more {
			// Both sides closed, mark that we closed and exit
			ic.closed.Store(true)
			return
		}
		if ic.OnRecv != nil {
			ic.OnRecv(received)
		}
	}
}

// Close closes the connection
func (ic *InterfaceConnection) Close() error {
	if ic.closed.Load() {
		return ErrConnectionClosed
	}
	close(ic.sendChan)
	close(ic.recvChan)
	return nil
}

func (ic *InterfaceConnection) Send(payload any) error {
	if ic.closed.Load() {
		return ErrConnectionClosed
	}
	ic.sendChan <- payload
	return nil
}
