package carport_test

import (
	"context"
	"errors"
	"testing"
	"time"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

func TestDispatch_EntityUnknown(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, nil)
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.nope", "turn_on", nil)
	if !errors.Is(err, carport.ErrEntityUnknown) {
		t.Fatalf("got %v, want ErrEntityUnknown", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if countByKind(evs, "command_issued")+countByKind(evs, "command_ack") > 0 {
		t.Fatal("no command events should be appended on pre-flight error")
	}
}

func TestDispatch_InstanceNotRunning(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.kitchen", "turn_on", nil)
	if !errors.Is(err, carport.ErrInstanceNotRunning) {
		t.Fatalf("got %v, want ErrInstanceNotRunning", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if countByKind(evs, "command_issued")+countByKind(evs, "command_ack") > 0 {
		t.Fatal("no command events should be appended on pre-flight error")
	}
}

func TestDispatch_HappyPathAppendsIssuedAndAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", map[string]string{"brightness": "60"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !res.Ok {
		t.Error("expected ok=true")
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if issued, acked := countByKind(evs, "command_issued"), countByKind(evs, "command_ack"); issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1", issued, acked)
	}
}

func TestDispatch_TimeoutAppendsAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		time.Sleep(200 * time.Millisecond)
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := h.Dispatch(ctx, "light.kitchen", "turn_on", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, carport.ErrDispatchTimeout) {
		t.Errorf("expected timeout error, got %v", err)
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if issued, acked := countByKind(evs, "command_issued"), countByKind(evs, "command_ack"); issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1 (INV-1)", issued, acked)
	}
}

func TestDispatch_DriverReportsError(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{
			CommandId:    c.CommandId,
			Ok:           false,
			Code:         carportpb.CarportErrorCode_CARPORT_DEVICE_OFFLINE,
			ErrorMessage: "bulb offline",
		}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", nil)
	if err != nil {
		t.Fatalf("Dispatch err = %v, want nil (driver-typed failure should return result, nil)", err)
	}
	if res.Ok {
		t.Error("expected ok=false")
	}
	if res.ErrorMessage != "bulb offline" {
		t.Errorf("ErrorMessage = %q", res.ErrorMessage)
	}
}

func countByKind(evs []eventstore.Event, kind string) int {
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}
