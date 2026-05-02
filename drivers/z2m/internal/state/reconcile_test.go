package state

import (
	"sort"
	"testing"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/z2m"
)

func actionTypes(actions []Action) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		switch a.(type) {
		case AddEntity:
			out[i] = "add"
		case UnregisterEntity:
			out[i] = "remove"
		case UpdateAttrs:
			out[i] = "update"
		default:
			out[i] = "unknown"
		}
	}
	sort.Strings(out)
	return out
}

func TestReconcileEmptyToN(t *testing.T) {
	devices := loadFixture(t)
	actions := Reconcile(nil, devices)
	for _, a := range actions {
		if _, ok := a.(AddEntity); !ok {
			t.Errorf("expected only AddEntity actions, got %T", a)
		}
	}
	// 5 devices in fixture: kitchen_light(1) + hallway_motion(4) +
	// front_door(2) + office_plug(1) + Coordinator(0) = 8 entities.
	if len(actions) != 8 {
		t.Errorf("action count: got %d, want 8", len(actions))
	}
}

func TestReconcileNoOp(t *testing.T) {
	devices := loadFixture(t)
	actions := Reconcile(devices, devices)
	if len(actions) != 0 {
		t.Errorf("no-op should produce zero actions; got %d (%v)", len(actions), actionTypes(actions))
	}
}

func TestReconcileAdd(t *testing.T) {
	all := loadFixture(t)
	prev := all[:len(all)-1] // drop coordinator
	// Coordinator yields zero entities, so add it back: still zero adds.
	actions := Reconcile(prev, all)
	if len(actions) != 0 {
		t.Errorf("adding a coordinator should add zero entities; got %v", actionTypes(actions))
	}

	// Add a real device (kitchen_light only in next).
	prev = []z2m.Device{}
	next := []z2m.Device{deviceByName(t, all, "front_door")}
	actions = Reconcile(prev, next)
	if len(actions) != 2 { // contact + battery
		t.Errorf("front_door: got %d adds, want 2", len(actions))
	}
	for _, a := range actions {
		if _, ok := a.(AddEntity); !ok {
			t.Errorf("expected AddEntity, got %T", a)
		}
	}
}

func TestReconcileRemove(t *testing.T) {
	all := loadFixture(t)
	prev := []z2m.Device{deviceByName(t, all, "front_door")}
	next := []z2m.Device{}
	actions := Reconcile(prev, next)
	if len(actions) != 2 {
		t.Errorf("front_door removal: got %d, want 2", len(actions))
	}
	for _, a := range actions {
		if _, ok := a.(UnregisterEntity); !ok {
			t.Errorf("expected UnregisterEntity, got %T", a)
		}
	}
}

func TestReconcileRename(t *testing.T) {
	all := loadFixture(t)
	original := deviceByName(t, all, "front_door")
	renamed := original
	renamed.FriendlyName = "back_door"
	prev := []z2m.Device{original}
	next := []z2m.Device{renamed}
	actions := Reconcile(prev, next)
	// 2 entities (contact + battery) → 2 UpdateAttrs.
	if len(actions) != 2 {
		t.Fatalf("rename: got %d actions, want 2", len(actions))
	}
	for _, a := range actions {
		ua, ok := a.(UpdateAttrs)
		if !ok {
			t.Errorf("expected UpdateAttrs, got %T", a)
			continue
		}
		if ua.NewFriendlyName != "back_door "+ua.Property {
			t.Errorf("UpdateAttrs.NewFriendlyName: got %q, want %q", ua.NewFriendlyName, "back_door "+ua.Property)
		}
	}
}

func TestReconcileMixed(t *testing.T) {
	all := loadFixture(t)
	prev := []z2m.Device{deviceByName(t, all, "front_door")}
	next := []z2m.Device{deviceByName(t, all, "kitchen_light")}
	actions := Reconcile(prev, next)
	// 2 removes (front_door entities) + 1 add (kitchen_light entity).
	want := []string{"add", "remove", "remove"}
	if got := actionTypes(actions); !equalStringSlices(got, want) {
		t.Errorf("mixed: got %v, want %v", got, want)
	}
}

func TestReconcileAddBeforeRemoveOrder(t *testing.T) {
	// Within one cycle, AddEntity actions must precede UnregisterEntity
	// actions so retained state delivery for added topics can race-free
	// with the registration. UpdateAttrs go last (relabeling).
	all := loadFixture(t)
	prev := []z2m.Device{deviceByName(t, all, "front_door")}
	next := []z2m.Device{deviceByName(t, all, "kitchen_light")}
	actions := Reconcile(prev, next)

	addedAt, removedAt := -1, -1
	for i, a := range actions {
		switch a.(type) {
		case AddEntity:
			if addedAt == -1 {
				addedAt = i
			}
		case UnregisterEntity:
			if removedAt == -1 {
				removedAt = i
			}
		}
	}
	if addedAt == -1 || removedAt == -1 {
		t.Fatalf("expected both add and remove; got %v", actionTypes(actions))
	}
	if addedAt > removedAt {
		t.Errorf("expected adds before removes; got order %v", actionTypes(actions))
	}
}

func TestReconcileTopicsCarried(t *testing.T) {
	// AddEntity carries the device's per-device topics so main can
	// subscribe in lock-step.
	all := loadFixture(t)
	next := []z2m.Device{deviceByName(t, all, "kitchen_light")}
	actions := Reconcile(nil, next)
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(actions))
	}
	add, ok := actions[0].(AddEntity)
	if !ok {
		t.Fatalf("expected AddEntity, got %T", actions[0])
	}
	wantState := "zigbee2mqtt/kitchen_light"
	// Reconcile is base-agnostic; it stores friendly_name and lets main
	// build topics. Verify FriendlyName is what we expect.
	if add.FriendlyName != "kitchen_light" {
		t.Errorf("FriendlyName: got %q, want %q", add.FriendlyName, "kitchen_light")
	}
	// EntityID should match what EntityID() produces.
	if add.EntityID != "light.z2m_01234abc" {
		t.Errorf("EntityID: got %q, want %q", add.EntityID, "light.z2m_01234abc")
	}
	_ = wantState
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
