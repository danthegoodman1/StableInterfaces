package stableinterfaces

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	testError             = errors.New("test error")
	errUnknownInstruction = errors.New("test error")
)

const (
	testInstructionReturnError      = "err"
	testInstructionReturnInternalID = "return internal id"
	testInstructionDefault          = "default response"

	testMetaKey             = "instruction"
	testInstructionReject   = "reject"
	testInstructionAccept   = "accept"
	testInstructionClose    = "close"
	testInstructionDoAlarm  = "alarm"
	testInstructionShutdown = "shutdown"

	testAlarmChannelKey   = "alarmChan"
	testAlarmRetry        = "retry"
	testAlarmRetryForever = "retry forever"
)

type (
	TestInterface struct {
		internalID string
		conn       *InterfaceConnection
	}
)

func (ti *TestInterface) OnRequest(c InterfaceContext, payload any) (any, error) {
	if s, ok := payload.(string); ok {
		switch s {
		case testInstructionReturnError:
			return nil, testError
		case testInstructionShutdown:
			go c.Shutdown(c.Context)
			return nil, nil
		case testInstructionReturnInternalID:
			return ti.internalID, nil
		case testInstructionDoAlarm:
			responseChan1 := make(chan any)
			responseChan2 := make(chan any)
			responseChan3 := make(chan any)
			responseChan4 := make(chan any)
			alarmID := genRandomID("")
			err := c.SetAlarm(c.Context, alarmID+"_0", map[string]any{
				testAlarmChannelKey: responseChan1,
			}, time.Now().Add(time.Millisecond*300))
			if err != nil {
				return nil, fmt.Errorf("error in SetAlarm: %w", err)
			}

			//  Ensure they are sequential by ID
			err = c.SetAlarm(c.Context, alarmID+"_1", map[string]any{
				testAlarmChannelKey: responseChan2,
			}, time.Now().Add(time.Millisecond*300))
			if err != nil {
				return nil, fmt.Errorf("error in SetAlarm: %w", err)
			}

			//  Queue another for retried
			err = c.SetAlarm(c.Context, alarmID+"_2", map[string]any{
				testAlarmRetry:      true,
				testAlarmChannelKey: responseChan3,
			}, time.Now().Add(time.Millisecond*300))
			if err != nil {
				return nil, fmt.Errorf("error in SetAlarm: %w", err)
			}

			//  Queue another for timeout
			err = c.SetAlarm(c.Context, alarmID+"_3", map[string]any{
				testAlarmRetryForever: true,
				testAlarmChannelKey:   responseChan4,
			}, time.Now().Add(time.Millisecond*300))
			if err != nil {
				return nil, fmt.Errorf("error in SetAlarm: %w", err)
			}

			return []chan any{responseChan1, responseChan2, responseChan3, responseChan4}, nil
		default:
			return testInstructionDefault, nil
		}
	}
	return nil, errUnknownInstruction
}

type TestInterfaceWithConnect struct {
	TestInterface
}

func (ti *TestInterfaceWithConnect) OnConnect(c InterfaceContext, ic IncomingConnection) {
	switch ic.Meta[testMetaKey] {
	case testInstructionReject:
		ic.Reject(testError)
	case testInstructionAccept:
		conn := ic.Accept()
		conn.OnRecv = func(payload any) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			fmt.Printf("Test interface %s (%s) received message: %+v\n", ic.instanceID, ic.ConnectionID, payload)
			if s, ok := payload.(string); ok && s == testInstructionClose {
				err := conn.Close()
				if err != nil {
					fmt.Printf("Test interface %s (%s) PANICKING on message: %+v\n", ic.instanceID, ic.ConnectionID, payload)
					panic(err)
				}
				fmt.Printf("Test interface %s (%s) closed connection on message: %+v\n", ic.instanceID, ic.ConnectionID, payload)
				// Try sending, this should error
				if err := conn.Send(ctx, "blah"); !errors.Is(err, ErrConnectionClosed) {
					panic(err)
				}
				return
			}
			err := conn.Send(ctx, "Thanks for the message!")
			if errors.Is(err, ErrConnectionClosed) {
				fmt.Printf("Test interface %s (%s) tried to send back but was closed on message: %+v (this is ok if it's the second message, that's just concurrency)\n", ic.instanceID, ic.ConnectionID, payload)
			} else if err != nil {
				fmt.Printf("Test interface %s (%s) PANICKING on message: %+v\n", ic.instanceID, conn.ID, payload)
				panic(err)
			}
			if c, ok := payload.(chan any); ok {
				// The test is waiting us to verify we got it
				c <- nil
			}
		}
	default:
		// Do nothing by default
	}
}

type TestInterfaceWithAlarm struct {
	TestInterface
}

func (tia *TestInterfaceWithAlarm) OnAlarm(c InterfaceContextWithAttempt, alarmID string, alarmMeta map[string]any) error {
	fmt.Printf("Test interface %s got alarm %s with attempt %d\n", tia.internalID, alarmID, c.Attempt)
	if _, exists := alarmMeta[testAlarmRetryForever]; exists {
		return testError
	}
	if _, exists := alarmMeta[testAlarmRetry]; exists {
		if c.Attempt < 4 {
			return testError
		}
	}
	resChan := alarmMeta[testAlarmChannelKey].(chan any)
	resChan <- nil
	return nil
}

func TestRequest(t *testing.T) {
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

	im2, err := NewInterfaceManager("host-1", "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("My shards: %+v", im.myShards)

	// Check hosts
	internalID, err := im.GetInternalID(id)
	if err != nil {
		t.Fatal(err)
	}

	instanceHost, err := im.GetHostForInternalID(internalID)
	if err != nil {
		t.Fatal(err)
	}
	if instanceHost != host {
		t.Fatalf("got mismatched hosts %s and %s", host, instanceHost)
	}

	// Verify the other host says the same
	instanceHost, err = im2.GetHostForInternalID(internalID)
	if err != nil {
		t.Fatal(err)
	}
	if instanceHost != host {
		t.Fatalf("got mismatched hosts %s and %s", host, instanceHost)
	}

	res, err := im.Request(context.Background(), id, testInstructionReturnInternalID)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := res.(string); !ok || s != internalID {
		t.Fatalf("did not get matching internal ID, got: %+v", s)
	}

	// Check error handling
	res, err = im.Request(context.Background(), id, testInstructionReturnError)
	if err != nil {
		if !errors.Is(err, testError) || !errors.Is(err, StableInterfaceHandlerErr) {
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

	// Shut it down
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()
	err = im.ShutdownInstance(ctx, internalID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnect(t *testing.T) {
	host := "host-0"
	id := "wrgh9uierhguhrhgierhughe"
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterfaceWithConnect{
			TestInterface{
				internalID: internalID,
			},
		}
	}, WithConnect())
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
		testMetaKey: testInstructionReject,
	})
	if !errors.Is(err, ErrIncomingConnectionRejected) {
		t.Fatalf("did not get rejected error, got \n\tIC: %+v\n\tErr: %+v", ic, err)
	}

	// Test handling accept
	ic, err = im.Connect(context.Background(), id, map[string]any{
		testMetaKey: testInstructionAccept,
	})
	if err != nil {
		t.Fatal(err)
	}

	closeChan := make(chan any)
	ic.AddOnCloseListener(func() {
		closeChan <- nil
	})
	ic.OnRecv = func(payload any) {
		fmt.Println("Test function got message from test interface:", payload)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	c1 := make(chan any)
	c2 := make(chan any)
	err = ic.Send(ctx, c1)
	if err != nil {
		t.Fatal(err)
	}
	<-c1

	err = ic.Send(ctx, c2)
	if err != nil {
		t.Fatal(err)
	}
	<-c2

	t.Log("got messages on channels")

	// Close test
	err = ic.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Verify shuttingDown again doesn't work
	err = ic.Close()
	if !errors.Is(err, ErrConnectionClosed) {
		t.Fatal("did not get ErrConnectionClosed, got", err)
	}

	err = ic.Send(ctx, "blah")
	if !errors.Is(err, ErrConnectionClosed) {
		t.Fatal("did not get ErrConnectionClosed, got", err)
	}

	// Make another to test remote close
	ic, err = im.Connect(context.Background(), id, map[string]any{
		testMetaKey: testInstructionAccept,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ic.Send(ctx, testInstructionClose)
	if err != nil {
		t.Fatal(err)
	}

	// Let's sleep to prevent hitting the context deadline due to optimistic shuttingDown
	time.Sleep(time.Millisecond)

	// need to wait because this might happen before it's closed
	err = ic.Send(ctx, testInstructionClose)
	if errors.Is(err, context.DeadlineExceeded) {
		t.Log("Concurrency resulted in the send finding closed after deadline!")
	}
	if !errors.Is(err, ErrConnectionClosed) {
		t.Fatal("did not get ErrConnectionClosed, got", err)
	}

	select {
	case <-closeChan:
		t.Log("got close chan")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	fmt.Println("----------")

	// Test shutdown
	ic, err = im.Connect(context.Background(), id, map[string]any{
		testMetaKey: testInstructionAccept,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("got new conn", ic.ID, ic.closed.Load())

	closeChan = make(chan any)
	ic.AddOnCloseListener(func() {
		closeChan <- nil
	})
	ic.OnRecv = func(payload any) {
		fmt.Println("Test function got message from test interface:", payload)
	}

	// Shut it down
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()
	internalID, err := im.GetInternalID(id)
	if err != nil {
		t.Fatal(err)
	}

	err = im.ShutdownInstance(ctx, internalID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify already closed
	err = ic.Close()
	if !errors.Is(err, ErrConnectionClosed) {
		t.Fatal("did not get ErrConnectionClosed, got", err)
	}

	select {
	case <-closeChan:
		t.Log("got close chan")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// Verify it's gone
	_, exists := im.instanceManagers.Load(internalID)
	if exists {
		t.Fatal("exists!")
	}
}

func TestWithAlarm(t *testing.T) {
	host := "host-0"
	id := "wrgh9uierhguhrhgierhughe"

	alarmManager := NewMemAlarmManager()

	// Verify that the alarm interface creates correctly
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterfaceWithAlarm{
			TestInterface{
				internalID: internalID,
			},
		}
	}, WithAlarm(&alarmManager))
	if err != nil {
		t.Fatal(err)
	}

	// Verify that it works without alarm
	_, err = NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterfaceWithAlarm{
			TestInterface{
				internalID: internalID,
			},
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify that error throws if not alarm
	_, err = NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterface{
			internalID: internalID,
		}
	}, WithAlarm(nil))
	if !errors.Is(err, ErrInterfaceNotWithAlarm) {
		t.Fatal("did not get ErrInterfaceNotWithAlarm, got:", err)
	}

	// List the alarms
	internalID, err := im.GetInternalID(id)
	if err != nil {
		t.Fatal(err)
	}

	shard := instanceInternalIDToShard(internalID, 1024)
	iam, exists := im.internalAlarmManagers.Load(shard)
	if !exists {
		t.Fatal("alarm manager did not exist")
	}

	// verify there are no active alarms
	activeAlarms := iam.listAlarmsForInstance(internalID)
	if len(activeAlarms) != 0 {
		for _, aa := range activeAlarms {
			t.Log(aa.ID)
		}
		t.Fatalf("Got incorrect number of active alarms: %d", len(activeAlarms))
	}

	// Test alarm firing
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := im.Request(ctx, id, testInstructionDoAlarm)
	if err != nil {
		t.Fatal(err)
	}

	activeAlarms = iam.listAlarmsForInstance(internalID)
	if len(activeAlarms) != 4 {
		for _, aa := range activeAlarms {
			t.Log(aa.ID)
		}
		t.Fatalf("Got incorrect number of active alarms: %d", len(activeAlarms))
	}

	alarmChans, ok := res.([]chan any)
	if !ok {
		t.Fatal("did not get back a chan any")
	}

	select {
	case <-alarmChans[0]:
		break
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// Listen on the second one, should fire immediately because of immediate second firing for alert
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()

	select {
	case <-alarmChans[1]:
		break
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-alarmChans[2]:
		break
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// This isn't the greatest test or checking max backoff
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	select {
	case <-alarmChans[3]:
		t.Fatal("got the alarm?")
	case <-ctx.Done():
		err = ctx.Err()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("got some other error:", err)
		}
	}

	// Test if an alarm after shutdown will wake it back up
	// Verify it's running
	_, exists = im.instanceManagers.Load(internalID)
	if !exists {
		t.Fatal("instance did not exist")
	}

	// Send alarm
	_, err = im.Request(ctx, id, testInstructionDoAlarm)
	if err != nil {
		t.Fatal(err)
	}

	// Let it shut down
	_, err = im.Request(context.Background(), id, testInstructionShutdown)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)

	// Verify it's not running
	_, exists = im.instanceManagers.Load(internalID)
	if exists {
		t.Fatal("instance exists")
	}

	// Wait for alarm
	time.Sleep(time.Second)

	// Verify it's running
	_, exists = im.instanceManagers.Load(internalID)
	if !exists {
		t.Fatal("instance did not exist")
	}
}

func TestShutdown(t *testing.T) {
	host := "host-0"
	id := "wrgh9uierhguhrhgierhughe"

	alarmManager := NewMemAlarmManager()

	// Verify that the alarm interface creates correctly
	im, err := NewInterfaceManager(host, "host-{0..1}", 1024, func(internalID string) StableInterface {
		return &TestInterfaceWithAlarm{
			TestInterface{
				internalID: internalID,
			},
		}
	}, WithAlarm(&alarmManager))
	if err != nil {
		t.Fatal(err)
	}

	internalID, err := im.GetInternalID(id)
	if err != nil {
		t.Fatal(err)
	}

	_, err = im.Request(context.Background(), id, testInstructionDefault)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's running
	_, exists := im.instanceManagers.Load(internalID)
	if !exists {
		t.Fatal("instance did not exist")
	}

	_, err = im.Request(context.Background(), id, testInstructionShutdown)
	if err != nil {
		t.Fatal(err)
	}

	// Let it shut down
	time.Sleep(time.Millisecond)

	// Verify it's not running
	_, exists = im.instanceManagers.Load(internalID)
	if exists {
		t.Fatal("instance exists")
	}
}
