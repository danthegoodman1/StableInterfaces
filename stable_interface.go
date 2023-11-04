package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
)

var (
	StableInterfaceHandlerErr = errors.New("stable interface returned error")
)

// StableInterface is implemented as a class to
type StableInterface interface {
	OnRequest(context.Context, any) (any, error)
	// OnConnect(context.Context, any) error
}

func wrapStableInterfaceHandlerError(err error) error {
	return fmt.Errorf("%w: %w", StableInterfaceHandlerErr, err)
}
