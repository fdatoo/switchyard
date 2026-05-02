package state

import (
	"log/slog"

	"github.com/fdatoo/switchyard-driverkit/driver"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/z2m"
)

// blockedProperties never become entities. linkquality/voltage are
// noise; update_available/last_seen are housekeeping.
var blockedProperties = map[string]bool{
	"linkquality":      true,
	"voltage":          true,
	"update_available": true,
	"last_seen":        true,
}

// numericSensorProperties are the read-only numeric leaves we surface.
// Anything not in this set is ignored (debug log).
var numericSensorProperties = map[string]bool{
	"temperature": true,
	"humidity":    true,
	"illuminance": true,
	"battery":     true,
	"pressure":    true,
	"power":       true,
	"energy":      true,
	"current":     true,
}

// binarySensorProperties are the read-only binary leaves we surface.
var binarySensorProperties = map[string]bool{
	"occupancy":  true,
	"contact":    true,
	"water_leak": true,
	"smoke":      true,
	"tamper":     true,
	"vibration":  true,
}

// EntityResult is one entity to register, plus the Z2M property name
// (if any) the caller should listen for on the device's state topic.
// Property is empty for lights (a light entity merges multiple
// properties — state, brightness, color_temp, color — under one ID;
// the caller iterates the StatePayload itself).
type EntityResult struct {
	EntityID string
	Spec     driver.EntitySpec
	Property string
}

// EntitiesFor walks the device's exposes tree and returns one entity
// per supported (read-only) leaf, plus one collapsed light entity for
// any "light" composite. Unknown leaf types are skipped silently
// (debug-logged at the caller level if log verbosity is up). Writable
// non-light properties (smart plug state) are skipped with one INFO
// log line so users can see what they're missing.
func EntitiesFor(dev z2m.Device) []EntityResult {
	out := make([]EntityResult, 0, len(dev.Definition.Exposes))
	for _, e := range dev.Definition.Exposes {
		out = append(out, mapExpose(dev, e)...)
	}
	return out
}

func mapExpose(dev z2m.Device, e z2m.Expose) []EntityResult {
	switch e.Type {
	case "light":
		return []EntityResult{lightEntity(dev, e)}
	case "switch":
		// v0.1: writable Switch is out of scope. Surface any read-only
		// child properties (e.g. power on smart plugs) but skip the
		// settable state child with an INFO log.
		var out []EntityResult
		for _, f := range e.Features {
			out = append(out, mapExpose(dev, f)...)
		}
		return out
	case "numeric":
		if blockedProperties[e.Property] {
			return nil
		}
		if e.AccessSettable() && !blockedProperties[e.Property] {
			// Writable numeric — out of scope (no actuator class for
			// numerics in v0.1). Skip silently.
			return nil
		}
		if !numericSensorProperties[e.Property] {
			slog.Debug("z2m: unrecognised numeric property; skipping",
				"device", dev.FriendlyName, "property", e.Property)
			return nil
		}
		return []EntityResult{numericSensorEntity(dev, e)}
	case "binary":
		if blockedProperties[e.Property] {
			return nil
		}
		if e.AccessSettable() {
			// Writable binary outside a `light` composite — typically a
			// smart plug's `state`. Skip in v0.1 with a one-shot INFO so
			// the user sees what's not surfaced.
			slog.Info("z2m: writable binary property skipped (Switch class out of scope in v0.1)",
				"device", dev.FriendlyName, "property", e.Property)
			return nil
		}
		if !binarySensorProperties[e.Property] {
			slog.Debug("z2m: unrecognised binary property; skipping",
				"device", dev.FriendlyName, "property", e.Property)
			return nil
		}
		return []EntityResult{binarySensorEntity(dev, e)}
	default:
		// composite / climate / cover / lock / fan / enum / text / list —
		// out of scope in v0.1. Composite is handled inside light/switch
		// above; everything else falls through silently.
		slog.Debug("z2m: unsupported expose type; skipping",
			"device", dev.FriendlyName, "type", e.Type, "name", e.Name)
		return nil
	}
}

func lightEntity(dev z2m.Device, e z2m.Expose) EntityResult {
	caps := []string{"turn_on", "turn_off"}
	for _, f := range e.Features {
		switch f.Property {
		case "brightness":
			caps = append(caps, "set_brightness")
		case "color_temp":
			caps = append(caps, "set_color_temp")
		case "color":
			caps = append(caps, "set_color")
		}
	}
	return EntityResult{
		EntityID: EntityID(dev.IEEEAddress, "light", ""),
		Spec: driver.EntitySpec{
			EntityType:   "light",
			FriendlyName: dev.FriendlyName,
			Capabilities: caps,
			InitialState: &entityv1.Attributes{
				Available: true,
				Kind:      &entityv1.Attributes_Light{Light: &entityv1.Light{}},
			},
		},
		Property: "",
	}
}

func numericSensorEntity(dev z2m.Device, e z2m.Expose) EntityResult {
	return EntityResult{
		EntityID: EntityID(dev.IEEEAddress, "numeric_sensor", e.Property),
		Spec: driver.EntitySpec{
			EntityType:   "numeric_sensor",
			FriendlyName: dev.FriendlyName + " " + e.Property,
			Capabilities: nil, // read-only
			InitialState: &entityv1.Attributes{
				Available: true,
				Kind: &entityv1.Attributes_NumericSensor{
					NumericSensor: &entityv1.NumericSensor{Unit: e.Unit},
				},
			},
		},
		Property: e.Property,
	}
}

func binarySensorEntity(dev z2m.Device, e z2m.Expose) EntityResult {
	return EntityResult{
		EntityID: EntityID(dev.IEEEAddress, "binary_sensor", e.Property),
		Spec: driver.EntitySpec{
			EntityType:   "binary_sensor",
			FriendlyName: dev.FriendlyName + " " + e.Property,
			Capabilities: nil, // read-only
			InitialState: &entityv1.Attributes{
				Available: true,
				Kind: &entityv1.Attributes_BinarySensor{
					BinarySensor: &entityv1.BinarySensor{},
				},
			},
		},
		Property: e.Property,
	}
}
