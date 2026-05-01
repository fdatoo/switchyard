package carport_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func seedRoutingFixture(t *testing.T) (*registry.Registry, func()) {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(nopWriter{}, nil))
	metrics := observability.NewMetrics()
	store, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })

	reg, err := registry.New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}

	// Seed: "started" driver_event registers the instance, then EntityRegistered.
	ctx := context.Background()
	if _, err := store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "driver_event",
		Source:    "carport:host",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
			DriverEvent: &eventv1.DriverEvent{DriverInstanceId: "hue_main", Kind: "started"},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "entity_registered",
		Entity:    "light.kitchen",
		Source:    "driver:hue_main",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "hue_main",
				EntityType:       "light",
				FriendlyName:     "Kitchen",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
					Light: &entityv1.Light{},
				}},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	return reg, func() {}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestRouting_ResolveHappyPath(t *testing.T) {
	reg, cleanup := seedRoutingFixture(t)
	defer cleanup()

	r := carport.NewRouter(reg)
	instanceID, err := r.Resolve(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if instanceID != "hue_main" {
		t.Errorf("instanceID = %q, want hue_main", instanceID)
	}
}

func TestRouting_ResolveUnknownEntityReturnsErrEntityUnknown(t *testing.T) {
	reg, cleanup := seedRoutingFixture(t)
	defer cleanup()

	r := carport.NewRouter(reg)
	_, err := r.Resolve(context.Background(), "light.nope")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, carport.ErrEntityUnknown) {
		t.Fatalf("got %v, want ErrEntityUnknown", err)
	}
}
