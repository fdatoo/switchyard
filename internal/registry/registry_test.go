package registry_test

import (
	"context"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestRegistry_EntityRegisteredCreatesRow(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	evt := eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered",
		Entity: "light.lr", Source: "driver:hue-1",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "hue-1",
				EntityType:       "light",
				FriendlyName:     "Living Room",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
					Light: &entityv1.Light{},
				}},
			},
		}},
	}
	if err := reg.Apply(ctx, tx, evt); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	got, err := reg.GetEntity(ctx, "light.lr")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.FriendlyName != "Living Room" {
		t.Fatalf("FriendlyName = %q", got.FriendlyName)
	}
}

func TestRegistry_EntityUnregisteredDisables(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// First register.
	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered", Entity: "light.lr", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "d", EntityType: "light", FriendlyName: "x",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{}}},
			},
		}},
	})
	_ = tx.Commit()

	// Then unregister.
	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, eventstore.Event{
		Position: 2, Timestamp: time.Now(), Kind: "entity_unregistered", Entity: "light.lr", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityUnregistered{
			EntityUnregistered: &eventv1.EntityUnregistered{Reason: "removed"},
		}},
	})
	_ = tx2.Commit()

	got, err := reg.GetEntity(ctx, "light.lr")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Disabled {
		t.Fatal("entity should be disabled")
	}

	// Default filter excludes disabled.
	list, _ := reg.ListEntities(ctx, registry.EntityFilter{})
	if len(list) != 0 {
		t.Fatalf("default list should exclude disabled, got %d", len(list))
	}
}

func TestRegistry_ApplyIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	evt := eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "entity_registered", Entity: "light.x", Source: "d",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "d", EntityType: "light", FriendlyName: "x",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{}}},
			},
		}},
	}
	for i := 0; i < 3; i++ {
		tx, _ := db.BeginTx(ctx, nil)
		if err := reg.Apply(ctx, tx, evt); err != nil {
			t.Fatalf("apply[%d]: %v", i, err)
		}
		_ = tx.Commit()
	}
	list, _ := reg.ListEntities(ctx, registry.EntityFilter{})
	if len(list) != 1 {
		t.Fatalf("idempotent apply yielded %d rows", len(list))
	}
}
