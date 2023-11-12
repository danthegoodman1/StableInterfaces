package stableinterfaces

import (
	"errors"
	"sync"
	"sync/atomic"
)

type (
	IncomingConnection struct {
		instanceID, ConnectionID string
		Meta                     map[string]any
		acceptChan               chan *connectionPair
		rejectChan               chan error
	}
)

var (
	ErrIncomingConnectionRejected   = errors.New("incoming connection rejected")
	ErrIncomingConnectionNotHandled = errors.New("incoming connection rejected due to not being handled")
)

func newIncomingConnection(instanceID, connectionID string, meta map[string]any) *IncomingConnection {
	return &IncomingConnection{
		ConnectionID: connectionID,
		instanceID:   instanceID,
		Meta:         meta,
		acceptChan:   make(chan *connectionPair, 1),
		rejectChan:   make(chan error, 1),
	}
}

// Accept binds the connection to the interface, and you can now send messages to the InterfaceConnection.
func (ic *IncomingConnection) Accept() *InterfaceConnection {
	managerToInterface := make(chan any)
	interfaceToManager := make(chan any)
	closedChan := make(chan any)
	closed := atomic.Bool{}

	connPair := connectionPair{
		ID:         ic.ConnectionID,
		closedChan: closedChan,
		InterfaceSide: InterfaceConnection{
			ID:         ic.ConnectionID,
			onClose:    nil,
			onCloseMu:  &sync.Mutex{},
			OnRecv:     nil,
			closed:     &closed,
			closedChan: closedChan,
			sendChan:   interfaceToManager,
			recvChan:   managerToInterface,
			side:       interfaceSide,
		},
		ManagerSide: InterfaceConnection{
			ID:         ic.ConnectionID,
			onClose:    nil,
			onCloseMu:  &sync.Mutex{},
			OnRecv:     nil,
			closed:     &closed,
			closedChan: closedChan,
			sendChan:   managerToInterface,
			recvChan:   interfaceToManager,
			side:       managerSide,
		},
	}

	// Setup listeners in goroutines
	go launchConnectionPairListener(&connPair)

	ic.acceptChan <- &connPair
	return &connPair.InterfaceSide
}

// Reject denies a connection with a given reason
func (ic *IncomingConnection) Reject(err error) {
	ic.rejectChan <- err
}
