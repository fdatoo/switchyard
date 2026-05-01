package eventstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestGoldenReplay(t *testing.T) {
	fixtures := []string{
		"basic_state_flow",
		"scene_apply",
		"driver_restart",
		"snapshot_roundtrip",
		"correlation_walk",
		"carport_happy_path",
		"carport_crash_recovery",
		"carport_quarantine",
	}
	for _, name := range fixtures {
		name := name
		t.Run(name, func(t *testing.T) {
			runFixture(t, name)
		})
	}
}

func runFixture(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	reg, err := registry.New(ctx, f.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	events := testutil.LoadFixture(t, name)

	if name == "correlation_walk" {
		// Wire up cause chain: each event caused by the previous one.
		// We must append one at a time to know each position.
		var prevPos uint64
		for i, e := range events {
			if i > 0 {
				e.CausePosition = prevPos
			}
			pos, err := f.store.Append(ctx, e)
			if err != nil {
				t.Fatalf("append[%d]: %v", i, err)
			}
			prevPos = pos
		}
	} else {
		for i, e := range events {
			if _, err := f.store.Append(ctx, e); err != nil {
				t.Fatalf("append[%d]: %v", i, err)
			}
		}
	}

	got := map[string]any{
		"state_cache": dumpCache(cache),
		"registry":    dumpRegistry(t, ctx, reg),
	}
	testutil.AssertGolden(t, name, got)
}

func dumpCache(c *state.Cache) map[string]any {
	out := map[string]any{}
	view := c.View()
	iter := view.Iterator()
	for !iter.Done() {
		id, s, _ := iter.Next()
		raw, _ := protojson.Marshal(s.Attributes)
		var attr any
		_ = json.Unmarshal(raw, &attr)
		out[id] = map[string]any{
			"updated_by": s.UpdatedBy,
			"attributes": attr,
		}
	}
	return out
}

func dumpRegistry(t *testing.T, ctx context.Context, r *registry.Registry) map[string]any {
	t.Helper()
	di, err := r.ListDriverInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := r.ListDevices(ctx, registry.DeviceFilter{IncludeDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ent, err := r.ListEntities(ctx, registry.EntityFilter{IncludeDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"driver_instances": summarizeDrivers(di),
		"devices":          summarizeDevices(dev),
		"entities":         summarizeEntities(ent),
	}
}

func summarizeDrivers(list []registry.DriverInstance) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{"id": d.ID, "status": d.Status})
	}
	return out
}

func summarizeDevices(list []registry.Device) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{"id": d.ID, "driver": d.DriverInstanceID, "disabled": d.Disabled})
	}
	return out
}

func summarizeEntities(list []registry.Entity) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{
			"id":       e.ID,
			"type":     e.EntityType,
			"disabled": e.Disabled,
			"driver":   e.DriverInstanceID,
		})
	}
	return out
}
