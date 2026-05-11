package carport_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	eventpb "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/carport/fakedriver"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
)

func newQuietLogger() *slog.Logger {
	return observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
}

func newQuietMetrics() *observability.Metrics { return observability.NewMetrics() }

// ---- RestartInstance ----

func TestRestartInstance_UnknownID(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, nil)
	defer h.Stop(context.Background())

	err := h.RestartInstance(context.Background(), "no_such_id")
	if err == nil {
		t.Fatal("expected error for unknown instance")
	}
	if !strings.Contains(err.Error(), "unknown instance") {
		t.Errorf("error = %v, want 'unknown instance'", err)
	}
}

func TestRestartInstance_KnownInstanceEmitsRestartManual(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	// Run RestartInstance with a short-lived ctx so its relaunched lifecycle
	// goroutine exits quickly when the test ends — there is no real binary, so
	// spawn will fail, but the test only asserts the manual-restart side effects.
	rctx, rcancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer rcancel()
	if err := h.RestartInstance(rctx, "hue_main"); err != nil {
		t.Fatalf("RestartInstance: %v", err)
	}

	// driver_event with kind=restart_manual must have been appended.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{
			Filter: eventstore.Filter{Kinds: []string{"driver_event"}},
		})
		for _, e := range evs {
			if de := e.Payload.GetDriverEvent(); de != nil && de.Kind == "restart_manual" && de.Detail == "operator" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("never observed restart_manual driver_event")
}

// ---- Health on instanceConn ----

func TestInstance_Health_OK(t *testing.T) {
	d := &fakedriver.Double{}
	sock, _ := d.Serve(fakeDriverTB{t})

	inst, err := carport.DialInstance(context.Background(), sock)
	if err != nil {
		t.Fatalf("DialInstance: %v", err)
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := inst.Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !resp.GetOk() {
		t.Errorf("Health.Ok = %v, want true", resp.GetOk())
	}
}

func TestFakeDriver_HandshakeAndShutdownState(t *testing.T) {
	d := &fakedriver.Double{
		ExpectedSecret: "secret",
		Manifest: &carportpb.DriverManifest{
			Name:            "fake",
			Version:         "1.2.3",
			ProtocolVersion: "v1alpha1",
		},
		InitialEntities: []*eventpb.EntityRegistered{{DeviceId: "light.fake"}},
	}

	if _, err := d.Handshake(context.Background(), &carportpb.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		InstanceId:      "fake",
		HandshakeSecret: "wrong",
	}); err == nil {
		t.Fatal("expected bad handshake secret to fail")
	}
	resp, err := d.Handshake(context.Background(), &carportpb.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		InstanceId:      "fake",
		HandshakeSecret: "secret",
	})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if resp.GetManifest().GetVersion() != "1.2.3" {
		t.Fatalf("manifest version = %q, want 1.2.3", resp.GetManifest().GetVersion())
	}
	if len(resp.GetInitialEntities()) != 1 {
		t.Fatalf("initial entities len = %d, want 1", len(resp.GetInitialEntities()))
	}
	if d.HandshakeCount() != 1 {
		t.Fatalf("HandshakeCount = %d, want 1", d.HandshakeCount())
	}
	if d.Closed() {
		t.Fatal("Closed true before Shutdown")
	}
	if _, err := d.Shutdown(context.Background(), &carportpb.ShutdownRequest{}); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if !d.Closed() {
		t.Fatal("Closed false after Shutdown")
	}
}

// ---- IngestMessage error paths ----

func TestIngestMessage_RejectsResultKind(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{Kind: &carportpb.DriverToHost_Result{Result: &carportpb.CommandResult{}}}
	err := carport.IngestMessage(context.Background(), f.store, "drv", msg)
	if err == nil {
		t.Fatal("expected error for Result kind")
	}
	if !strings.Contains(err.Error(), "Result should be consumed") {
		t.Errorf("error = %v", err)
	}
}

func TestIngestMessage_PongIsNoop(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{TsUnixMs: 1}}}
	if err := carport.IngestMessage(context.Background(), f.store, "drv", msg); err != nil {
		t.Errorf("Pong should be no-op, got %v", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if len(evs) != 0 {
		t.Errorf("Pong appended %d events, want 0", len(evs))
	}
}

func TestIngestMessage_UnknownKind(t *testing.T) {
	f := newStoreFixtureForTest(t)
	// Empty DriverToHost has nil Kind oneof — falls through default case.
	err := carport.IngestMessage(context.Background(), f.store, "drv", &carportpb.DriverToHost{})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown message kind") {
		t.Errorf("error = %v", err)
	}
}

func TestIngestMessage_StateChangedBindsEntityFromPayload(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{Kind: &carportpb.DriverToHost_StateChanged{
		StateChanged: &eventpb.StateChanged{EntityId: "light.kitchen"},
	}}
	if err := carport.IngestMessage(context.Background(), f.store, "drv", msg); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{
		Filter: eventstore.Filter{Kinds: []string{"state_changed"}},
	})
	if len(evs) != 1 || evs[0].Entity != "light.kitchen" || evs[0].Source != "driver:drv" {
		t.Fatalf("got %+v", evs)
	}
}

func TestIngestMessage_EntityRegisteredBindsEntityFromDeviceID(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{Kind: &carportpb.DriverToHost_EntityRegistered{
		EntityRegistered: &eventpb.EntityRegistered{DeviceId: "sensor.z", EntityType: "sensor", FriendlyName: "Z"},
	}}
	if err := carport.IngestMessage(context.Background(), f.store, "drv", msg); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{
		Filter: eventstore.Filter{Kinds: []string{"entity_registered"}},
	})
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	if evs[0].Entity != "sensor.z" {
		t.Errorf("Entity = %q, want sensor.z", evs[0].Entity)
	}
	if er := evs[0].Payload.GetEntityRegistered(); er == nil || er.DriverInstanceId != "drv" {
		t.Errorf("DriverInstanceId not back-filled: %+v", er)
	}
}

func TestIngestMessage_DriverEventFillsInstanceID(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{Kind: &carportpb.DriverToHost_DriverEvent{
		DriverEvent: &eventpb.DriverEvent{Kind: "k", Detail: "d"},
	}}
	if err := carport.IngestMessage(context.Background(), f.store, "drv", msg); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{
		Filter: eventstore.Filter{Kinds: []string{"driver_event"}},
	})
	if len(evs) != 1 {
		t.Fatalf("got %d events", len(evs))
	}
	if de := evs[0].Payload.GetDriverEvent(); de == nil || de.DriverInstanceId != "drv" {
		t.Errorf("DriverInstanceId not back-filled: %+v", de)
	}
}

// ---- New / Stop / InstanceState edges ----

func TestNew_NoInstances(t *testing.T) {
	f := newStoreFixtureForTest(t)
	logger := newQuietLogger()
	metrics := newQuietMetrics()
	h, err := carport.New(carport.HostConfig{SocketDir: t.TempDir()},
		f.db, f.store, nil, logger, metrics)
	if err != nil {
		t.Fatalf("New should succeed: %v", err)
	}
	if h == nil {
		t.Fatal("Host nil")
	}
	if state := h.InstanceState("anything"); state != carport.StateDeclared {
		t.Errorf("InstanceState unknown = %s, want StateDeclared", state)
	}
}

func TestStartStop_NoInstances(t *testing.T) {
	f := newStoreFixtureForTest(t)
	logger := newQuietLogger()
	metrics := newQuietMetrics()
	h, err := carport.New(carport.HostConfig{SocketDir: t.TempDir()},
		f.db, f.store, nil, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := h.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Stop is idempotent.
	h.Stop(context.Background())
	h.Stop(context.Background())
}
