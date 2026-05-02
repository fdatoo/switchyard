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

func driverEvt(pos uint64, ts time.Time, driverID, kind, detail string) eventstore.Event {
	return eventstore.Event{
		Position: pos, Timestamp: ts, Kind: "driver_event", Source: "switchyardd",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
			DriverEvent: &eventv1.DriverEvent{
				DriverInstanceId: driverID, Kind: kind, Detail: detail,
			},
		}},
	}
}

func entityRegEvt(pos uint64, entity, driver, device, eType, name string) eventstore.Event {
	return eventstore.Event{
		Position: pos, Timestamp: time.Now(), Kind: "entity_registered",
		Entity: entity, Source: "driver:" + driver,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: driver, DeviceId: device,
				EntityType: eType, FriendlyName: name,
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{}}},
			},
		}},
	}
}

func TestRegistry_Name(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Name() != "registry" {
		t.Fatalf("Name() = %q, want registry", reg.Name())
	}
}

func TestRegistry_DriverEvent_Started(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, err := registry.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := db.BeginTx(ctx, nil)
	if err := reg.Apply(ctx, tx, driverEvt(1, time.Now(), "d1", "started", "")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	_ = tx.Commit()

	di, err := reg.GetDriverInstance(ctx, "d1")
	if err != nil {
		t.Fatalf("GetDriverInstance: %v", err)
	}
	if di.Status != "running" {
		t.Fatalf("Status = %q, want running", di.Status)
	}
}

func TestRegistry_DriverEvent_Failed(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, driverEvt(1, time.Now(), "d1", "started", ""))
	_ = tx.Commit()

	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, driverEvt(2, time.Now(), "d1", "failed", "conn refused"))
	_ = tx2.Commit()

	di, _ := reg.GetDriverInstance(ctx, "d1")
	if di.Status != "failed" {
		t.Fatalf("Status = %q, want failed", di.Status)
	}
	if di.LastError != "conn refused" {
		t.Fatalf("LastError = %q", di.LastError)
	}
}

func TestRegistry_DriverEvent_Stopped(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, driverEvt(1, time.Now(), "d1", "started", ""))
	_ = tx.Commit()

	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, driverEvt(2, time.Now(), "d1", "stopped", ""))
	_ = tx2.Commit()

	di, _ := reg.GetDriverInstance(ctx, "d1")
	if di.Status != "stopped" {
		t.Fatalf("Status = %q, want stopped", di.Status)
	}
}

func TestRegistry_DriverEvent_Heartbeat(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, driverEvt(1, time.Now(), "d1", "started", ""))
	_ = tx.Commit()

	hbTime := time.Now().Add(time.Second)
	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, driverEvt(2, hbTime, "d1", "heartbeat", ""))
	_ = tx2.Commit()

	di, _ := reg.GetDriverInstance(ctx, "d1")
	if di.LastHeartbeat.IsZero() {
		t.Fatal("LastHeartbeat should be set after heartbeat")
	}
}

func TestRegistry_ListDriverInstances(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	for i, id := range []string{"d1", "d2"} {
		tx, _ := db.BeginTx(ctx, nil)
		_ = reg.Apply(ctx, tx, driverEvt(uint64(i+1), time.Now(), id, "started", ""))
		_ = tx.Commit()
	}

	list, err := reg.ListDriverInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
}

func TestRegistry_GetDriverInstance_NotFound(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	_, err := reg.GetDriverInstance(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent driver instance")
	}
}

func TestRegistry_GetDevice_And_ListDevices(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, entityRegEvt(1, "light.a", "d1", "dev-1", "light", "Light A"))
	_ = tx.Commit()

	dev, err := reg.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if dev.DriverInstanceID != "d1" {
		t.Fatalf("DriverInstanceID = %q", dev.DriverInstanceID)
	}

	devices, err := reg.ListDevices(ctx, registry.DeviceFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("ListDevices len = %d, want 1", len(devices))
	}

	byDriver, _ := reg.ListDevices(ctx, registry.DeviceFilter{DriverInstanceID: "d1"})
	if len(byDriver) != 1 {
		t.Fatalf("filter by driver len = %d, want 1", len(byDriver))
	}

	noMatch, _ := reg.ListDevices(ctx, registry.DeviceFilter{DriverInstanceID: "other"})
	if len(noMatch) != 0 {
		t.Fatalf("wrong driver filter len = %d, want 0", len(noMatch))
	}
}

func TestRegistry_GetDevice_NotFound(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	_, err := reg.GetDevice(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent device")
	}
}

func TestRegistry_ListEntities_Filters(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	evts := []eventstore.Event{
		entityRegEvt(1, "light.a", "d1", "dev-1", "light", "Light A"),
		entityRegEvt(2, "switch.b", "d2", "", "switch", "Switch B"),
	}
	for _, e := range evts {
		tx, _ := db.BeginTx(ctx, nil)
		_ = reg.Apply(ctx, tx, e)
		_ = tx.Commit()
	}

	byDriver, _ := reg.ListEntities(ctx, registry.EntityFilter{DriverInstanceID: "d1"})
	if len(byDriver) != 1 || byDriver[0].ID != "light.a" {
		t.Fatalf("filter by driver: %v", byDriver)
	}

	byType, _ := reg.ListEntities(ctx, registry.EntityFilter{EntityType: "switch"})
	if len(byType) != 1 || byType[0].ID != "switch.b" {
		t.Fatalf("filter by type: %v", byType)
	}

	byDevice, _ := reg.ListEntities(ctx, registry.EntityFilter{DeviceID: "dev-1"})
	if len(byDevice) != 1 || byDevice[0].ID != "light.a" {
		t.Fatalf("filter by device: %v", byDevice)
	}
}

func TestRegistry_EntityUnregistered(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, entityRegEvt(1, "light.a", "d1", "dev-1", "light", "Light A"))
	_ = tx.Commit()

	tx2, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx2, eventstore.Event{
		Position: 2, Timestamp: time.Now(), Kind: "entity_unregistered",
		Entity: "light.a", Source: "driver:d1",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityUnregistered{
			EntityUnregistered: &eventv1.EntityUnregistered{},
		}},
	})
	_ = tx2.Commit()

	ents, err := reg.ListEntities(ctx, registry.EntityFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// Disabled entities are excluded from default list.
	for _, e := range ents {
		if e.ID == "light.a" {
			t.Fatal("unregistered entity still visible")
		}
	}
}

func TestRegistry_Apply_UnknownPayload(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	err := reg.Apply(ctx, tx, eventstore.Event{
		Position: 1, Timestamp: time.Now(), Kind: "other",
		Payload: nil,
	})
	_ = tx.Commit()
	if err != nil {
		t.Fatalf("unknown payload should not error: %v", err)
	}
}

func TestRegistry_DriverEvent_UnknownKind(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	reg, _ := registry.New(ctx, db)

	tx, _ := db.BeginTx(ctx, nil)
	_ = reg.Apply(ctx, tx, driverEvt(1, time.Now(), "d1", "started", ""))
	_ = tx.Commit()

	tx2, _ := db.BeginTx(ctx, nil)
	err := reg.Apply(ctx, tx2, driverEvt(2, time.Now(), "d1", "reconnecting", ""))
	_ = tx2.Commit()
	if err != nil {
		t.Fatalf("unknown driver event kind should not error: %v", err)
	}
}
