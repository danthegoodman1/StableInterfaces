package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	TestError             = errors.New("test error")
	ErrUnknownInstruction = errors.New("test error")
)

const (
	TestInterfaceReturnError      = "err"
	TestInterfaceReturnInternalID = "return internal id"
	TestInterfaceDefault          = "default response"

	TestMetaKey           = "instruction"
	TestInstructionReject = "2"
	TestInstructionAccept = "3"
)

type TestInterface struct {
	internalID string
	conn       *InterfaceConnection
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

func (ti *TestInterface) OnConnect(ctx context.Context, ic IncomingConnection) {
	switch ic.Meta[TestMetaKey] {
	case TestInstructionReject:
		ic.Reject(TestError)
	case TestInstructionAccept:
		ti.conn = ic.Accept()
		ti.conn.OnRecv = func(payload any) {
			fmt.Printf("Test interface %s received message: %+v\n", ic.instanceID, payload)
			ti.conn.Send("Thanks for the message!")
		}
	default:
		// Do nothing by default
	}
}

func TestStableInterfaceRequest(t *testing.T) {
	host := "host-0"
	id := "wrgh9uierhguhrhgierhughe"
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check hosts
	instanceHost, err := im.GetHostForID(id)
	if err != nil {
		t.Fatal(err)
	}
	if instanceHost != host {
		t.Fatalf("got mismatched hosts %s and %s", host, instanceHost)
	}

	// Check id
	internalID, err := im.GetInternalID(id)
	if err != nil {
		t.Fatal(err)
	}

	res, err := im.Request(context.Background(), id, TestInterfaceReturnInternalID)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := res.(string); !ok || s != internalID {
		t.Fatalf("did not get matching internal ID, got: %+v", s)
	}

	// Check error handling
	res, err = im.Request(context.Background(), id, TestInterfaceReturnError)
	if err != nil {
		if !errors.Is(err, TestError) || !errors.Is(err, StableInterfaceHandlerErr) {
			t.Fatal()
		}
	}
	if err == nil {
		t.Fatal("was expecting error")
	}

	// Test wrong host
	_, err = im.Request(context.Background(), "afefe", nil)
	if !errors.Is(err, ErrHostDoesNotOwnShard) {
		t.Fatal("did not get host does not own shard")
	}
}

func TestStableInterfaceConnect(t *testing.T) {
	host := "host-0"
	id := "wrgh9uierhguhrhgierhughe"
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test do nothing
	ic, err := im.Connect(context.Background(), id, nil)
	if !errors.Is(err, ErrIncomingConnectionNotHandled) {
		t.Fatalf("did not get not handled error, got \n\tIC: %+v\n\tErr: %+v", ic, err)
	}

	// Test rejection
	ic, err = im.Connect(context.Background(), id, map[string]any{
		TestMetaKey: TestInstructionReject,
	})
	if !errors.Is(err, ErrIncomingConnectionRejected) {
		t.Fatalf("did not get rejected error, got \n\tIC: %+v\n\tErr: %+v", ic, err)
	}

	// Test handling request
	ic, err = im.Connect(context.Background(), id, map[string]any{
		TestMetaKey: TestInstructionAccept,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	ic.OnRecv = func(payload any) {
		fmt.Println("Test function got message from test interface:", payload)
		cancel()
	}

	ic.Send("Hello from test 1!")
	ic.Send("Hello from test 2!")

	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal(ctx.Err())
	}
}
