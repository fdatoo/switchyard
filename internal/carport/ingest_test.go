package carport_test

import (
	"context"
	"testing"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	entitypb "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventpb "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/carport"
	"github.com/fdatoo/gohome/internal/eventstore"
)

func TestIngestMessage_StateChanged(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_StateChanged{
			StateChanged: &eventpb.StateChanged{
				EntityId: "light.kitchen",
				Attributes: &entitypb.Attributes{
					Kind: &entitypb.Attributes_Light{Light: &entitypb.Light{On: true, Brightness: 100}},
				},
			},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", msg); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	events, err := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Kind != "state_changed" {
		t.Errorf("Kind = %q", events[0].Kind)
	}
	if events[0].Source != "driver:hue_main" {
		t.Errorf("Source = %q", events[0].Source)
	}
	if events[0].Entity != "light.kitchen" {
		t.Errorf("Entity = %q", events[0].Entity)
	}
}

func TestIngestMessage_DriverEvent(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_DriverEvent{
			DriverEvent: &eventpb.DriverEvent{
				DriverInstanceId: "hue_main",
				Kind:             "custom_button_press",
				Detail:           "button=up",
			},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", msg); err != nil {
		t.Fatal(err)
	}
	events, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(events) != 1 || events[0].Kind != "driver_event" {
		t.Fatalf("want driver_event, got %+v", events)
	}
}

func TestIngestMessage_EntityRegistered(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_EntityRegistered{
			EntityRegistered: &eventpb.EntityRegistered{
				DriverInstanceId: "hue_main",
				DeviceId:         "light.kitchen",
				EntityType:       "light",
				FriendlyName:     "Kitchen",
			},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", msg); err != nil {
		t.Fatal(err)
	}
	events, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(events) != 1 || events[0].Kind != "entity_registered" {
		t.Fatalf("want entity_registered, got %+v", events)
	}
	if events[0].Entity != "light.kitchen" {
		t.Errorf("Entity = %q, want light.kitchen (derived from EntityRegistered.DeviceId)", events[0].Entity)
	}
}

func TestIngestMessage_EntityUnregistered(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_EntityUnregistered{
			EntityUnregistered: &eventpb.EntityUnregistered{Reason: "user_removed"},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", msg); err != nil {
		t.Fatal(err)
	}
	events, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(events) != 1 || events[0].Kind != "entity_unregistered" {
		t.Fatalf("want entity_unregistered, got %+v", events)
	}
}

func TestIngestMessage_PongIsNoOp(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{}},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue_main", msg); err != nil {
		t.Fatal(err)
	}
	events, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(events) != 0 {
		t.Fatalf("pong should not append; got %d events", len(events))
	}
}

func TestIngestMessage_EntityUnregisteredBindsEntityID(t *testing.T) {
	f := newStoreFixtureForTest(t)
	msg := &carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_EntityUnregistered{
			EntityUnregistered: &eventpb.EntityUnregistered{EntityId: "light.kitchen", Reason: "removed_by_driver"},
		},
	}
	if err := carport.IngestMessage(context.Background(), f.store, "hue", msg); err != nil {
		t.Fatal(err)
	}
	evs, _ := f.store.Query(context.Background(), eventstore.QueryOptions{})
	if len(evs) != 1 || evs[0].Entity != "light.kitchen" {
		t.Fatalf("got %+v, want Entity=light.kitchen", evs)
	}
}
