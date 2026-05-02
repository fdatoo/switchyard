package carport_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/carport/fakedriver"
)

func serveFake(t *testing.T, d *fakedriver.Double) string {
	t.Helper()
	sock, _ := d.Serve(fakeDriverTB{t})
	return sock
}

func TestInstance_SendCommand_ResultDelivered(t *testing.T) {
	d := &fakedriver.Double{
		OnCommand: func(_ context.Context, c *carportpb.Command) *carportpb.CommandResult {
			return &carportpb.CommandResult{
				CommandId: c.CommandId,
				Ok:        true,
				Code:      carportpb.CarportErrorCode_CARPORT_OK,
			}
		},
	}
	sock := serveFake(t, d)

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatalf("DialInstance: %v", err)
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := inst.SendCommand(ctx, &carportpb.Command{
		CommandId:  "cmd-1",
		EntityId:   "light.x",
		Capability: "turn_on",
	})
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if res.CommandId != "cmd-1" || !res.Ok {
		t.Errorf("result = %+v", res)
	}
}

func TestInstance_SendCommand_TimeoutFailsFast(t *testing.T) {
	d := &fakedriver.Double{
		OnCommand: func(_ context.Context, c *carportpb.Command) *carportpb.CommandResult {
			time.Sleep(200 * time.Millisecond)
			return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
		},
	}
	sock := serveFake(t, d)

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = inst.SendCommand(ctx, &carportpb.Command{CommandId: "cmd-2", EntityId: "x", Capability: "y"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	// Accept either context.DeadlineExceeded (unwrapped) or ErrDispatchTimeout (mapped).
	if err != context.DeadlineExceeded && err != carport.ErrDispatchTimeout {
		t.Errorf("expected timeout-ish error, got %v", err)
	}
}

func TestInstance_ConcurrentCommandsMatchResults(t *testing.T) {
	d := &fakedriver.Double{
		OnCommand: func(_ context.Context, c *carportpb.Command) *carportpb.CommandResult {
			return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
		},
	}
	sock := serveFake(t, d)

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "cmd-" + strconv.Itoa(i)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := inst.SendCommand(ctx, &carportpb.Command{CommandId: id, EntityId: "x", Capability: "y"})
			if err != nil {
				t.Errorf("cmd %s: %v", id, err)
				return
			}
			if res.CommandId != id {
				t.Errorf("cmd %s: got %s", id, res.CommandId)
			}
		}(i)
	}
	wg.Wait()
}
