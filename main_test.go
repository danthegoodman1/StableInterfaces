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
	im, err := NewInterfaceManager("host-0", "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	id1 := "1"

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

	res, err = im.Request(context.Background(), id1, TestInterfaceReturnError)
	if err != nil {
		if !errors.Is(err, TestError) || !errors.Is(err, StableInterfaceHandlerErr) {
			t.Fatal()
		}
	}
	if err == nil {
		t.Fatal("was expecting error")
	}
}
