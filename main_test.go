package stableinterfaces

import (
	"context"
	"errors"
	"testing"
)

var (
	TestError             = errors.New("test error")
	ErrUnknownInstruction = errors.New("test error")
)

const (
	TestInterfaceReturnError      = "err"
	TestInterfaceReturnInternalID = "return internal id"
	TestInterfaceDefault          = "default response"
)

type TestInterface struct {
	internalID string
}

func (ti *TestInterface) OnRequest(ctx context.Context, payload any) (any, error) {
	if s, ok := payload.(string); ok {
		switch s {
		case TestInterfaceReturnError:
			return nil, TestError
		case TestInterfaceReturnInternalID:
			return ti.internalID, nil
		default:
			return TestInterfaceDefault, nil
		}
	}
	return nil, ErrUnknownInstruction
}

func TestStableInterfaceSimple(t *testing.T) {
	host := "host-0"
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	id1 := "1"

	// Check hosts
	instanceHost, err := im.GetHostForID(id1)
	if err != nil {
		t.Fatal(err)
	}
	if instanceHost != host {
		t.Fatalf("got mismatched hosts %s and %s", host, instanceHost)
	}

	// Check id
	internalID1, err := im.GetInternalID(id1)
	if err != nil {
		t.Fatal(err)
	}

	res, err := im.Request(context.Background(), id1, TestInterfaceReturnInternalID)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := res.(string); !ok || s != internalID1 {
		t.Fatalf("did not get matching internal ID, got: %+v", s)
	}

	// Check error handling
	res, err = im.Request(context.Background(), id1, TestInterfaceReturnError)
	if err != nil {
		if !errors.Is(err, TestError) || !errors.Is(err, StableInterfaceHandlerErr) {
			t.Fatal()
		}
	}
	if err == nil {
		t.Fatal("was expecting error")
	}

	// Test wrong host
	_, err = im.Request(context.Background(), "3", nil)
	if !errors.Is(err, ErrHostDoesNotOwnShard) {
		t.Fatal("did not get host does not own shard")
	}
}
