package state

import (
	"sort"

	"github.com/fdatoo/switchyard-driverkit/driver"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/z2m"
)

// Action is what Reconcile emits. main switches on the concrete type
// to apply each (subscribe + driver.AddEntity, etc.).
type Action interface{ isAction() }

// AddEntity instructs main to register the entity, install capability
// handlers (if any), and subscribe to the device's state +
// availability topics. FriendlyName is the Z2M friendly_name (used
// to compute /set topic); IEEE is carried so command handlers can
// look up the device address-stable.
type AddEntity struct {
	EntityID     string
	Spec         driver.EntitySpec
	IEEE         string
	FriendlyName string
	Property     string // "" for lights; the Z2M property name for sensors
}

func (AddEntity) isAction() {}

// UnregisterEntity instructs main to unsubscribe from the device's
// topics and call driver.UnregisterEntity.
type UnregisterEntity struct {
	EntityID     string
	FriendlyName string // for unsubscribe; main rebuilds topics
}

func (UnregisterEntity) isAction() {}

// UpdateAttrs is emitted on friendly_name change so the entity can be
// relabeled without re-registration. Property is "" for lights and the
// Z2M property name for sensors (used to build the new friendly name).
type UpdateAttrs struct {
	EntityID        string
	NewFriendlyName string
	Property        string
}

func (UpdateAttrs) isAction() {}

// Reconcile diffs prev → next at the entity level. The returned slice
// is ordered: all AddEntity first, then UnregisterEntity, then
// UpdateAttrs. This ordering matters for the v0.1 race window between
// retained-state delivery and entity registration (subscribe ahead of
// registration is the safer direction).
func Reconcile(prev, next []z2m.Device) []Action {
	prevByIEEE := indexByIEEE(prev)
	nextByIEEE := indexByIEEE(next)

	var adds, removes, updates []Action

	// Walk next: anything missing from prev is an add; anything with a
	// different friendly_name is an update.
	for ieee, ndev := range nextByIEEE {
		entries := EntitiesFor(ndev)
		pdev, existed := prevByIEEE[ieee]
		if !existed {
			for _, r := range entries {
				adds = append(adds, AddEntity{
					EntityID:     r.EntityID,
					Spec:         r.Spec,
					IEEE:         ieee,
					FriendlyName: ndev.FriendlyName,
					Property:     r.Property,
				})
			}
			continue
		}
		if pdev.FriendlyName != ndev.FriendlyName {
			for _, r := range entries {
				updates = append(updates, UpdateAttrs{
					EntityID:        r.EntityID,
					NewFriendlyName: r.Spec.FriendlyName,
					Property:        r.Property,
				})
			}
		}
		// Composition changes (a device's exposes tree gains/loses a
		// property without a rename) are not handled in v0.1: real Z2M
		// firmware updates are rare, and the user can restart the driver
		// to pick them up. Documented as a caveat in the README.
	}

	// Walk prev: anything missing from next is a remove.
	for ieee, pdev := range prevByIEEE {
		if _, stillThere := nextByIEEE[ieee]; stillThere {
			continue
		}
		for _, r := range EntitiesFor(pdev) {
			removes = append(removes, UnregisterEntity{
				EntityID:     r.EntityID,
				FriendlyName: pdev.FriendlyName,
			})
		}
	}

	sortByEntityID(adds)
	sortByEntityID(removes)
	sortByEntityID(updates)

	out := make([]Action, 0, len(adds)+len(removes)+len(updates))
	out = append(out, adds...)
	out = append(out, removes...)
	out = append(out, updates...)
	return out
}

func indexByIEEE(devices []z2m.Device) map[string]z2m.Device {
	out := make(map[string]z2m.Device, len(devices))
	for _, d := range devices {
		out[d.IEEEAddress] = d
	}
	return out
}

func sortByEntityID(actions []Action) {
	sort.SliceStable(actions, func(i, j int) bool {
		return entityIDOf(actions[i]) < entityIDOf(actions[j])
	})
}

func entityIDOf(a Action) string {
	switch v := a.(type) {
	case AddEntity:
		return v.EntityID
	case UnregisterEntity:
		return v.EntityID
	case UpdateAttrs:
		return v.EntityID
	}
	return ""
}
