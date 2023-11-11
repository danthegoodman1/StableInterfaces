package stableinterfaces

import (
	"errors"
	"fmt"
)

var (
	StableInterfaceHandlerErr = errors.New("stable interface returned error")
)

// StableInterface is implemented as a class to
type StableInterface interface {
	// OnRequest is blocking. If you want concurrency, launch a goroutine in the handler.
	OnRequest(InterfaceContext, any) (any, error)
	// OnConnect is blocking. Launch a goroutine if you want concurrency.
	// If a connection is not handled (not accepted or rejected), it is implicitly rejected.
	OnConnect(InterfaceContext, IncomingConnection)
}

type StableInterfaceWithAlarm interface {
	StableInterface
	OnAlarm(ctx InterfaceContextWithAttempt, id string, meta map[string]any) error
}

func wrapStableInterfaceHandlerError(err error) error {
	return fmt.Errorf("%w: %w", StableInterfaceHandlerErr, err)
}
