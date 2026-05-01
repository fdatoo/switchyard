# Entity Proto — Binary Sensor + Numeric Sensor Rename — Design

**Status:** draft
**Date:** 2026-04-30
**Branch:** `feat/entity-binary-sensor`

## Summary

Rename `gohome.entity.v1.Sensor` to `NumericSensor` and add a sibling
`BinarySensor` variant in the `Attributes` oneof. This unblocks drivers (the
forthcoming Z2M driver, in particular) from surfacing motion / contact / leak
devices without smuggling booleans through the numeric `value`/`unit` fields.

## Motivation

`Sensor{string unit, double value}` only fits numeric quantities. Z2M's most
useful device classes — occupancy, contact, water leak, smoke, tamper — are
binary. Two paths existed:

1. Map booleans to `0.0`/`1.0` with `unit="bool"`. Works on the wire, but every
   downstream consumer (UI, automations, MCP descriptions) has to special-case
   the convention to know "render as on/off, don't average, don't graph as a
   line." The convention leaks indefinitely.
2. Add a proper `BinarySensor` proto variant. One-time cost; type system stays
   honest.

We are early enough that the rename cost is negligible (one non-generated call
site at `internal/daemon/daemon.go:585`), so we take path 2.

While we are touching the proto, we also rename `Sensor` → `NumericSensor` for
symmetry with `BinarySensor`. Leaving the bare name `Sensor` for the numeric
variant only would be a "default kind" trap for new contributors.

## Goals

- Rename the message type `Sensor` → `NumericSensor` and the oneof field name
  `sensor` → `numeric_sensor`. Field number stays `12`.
- Add a new message `BinarySensor { bool on = 1; }` and oneof field
  `binary_sensor = 13`.
- Update the daemon's only switch case (`internal/daemon/daemon.go:585`) to use
  the new generated symbol.
- Regenerate `attributes.pb.go` and verify the tree builds.

## Non-goals

- Wider semantics beyond `bool on` for binary sensors (e.g. `last_changed`,
  `inverted`, device class). These accrete later if needed.
- Migration of any existing emitted data. Nothing in production yet emits
  `Sensor` events; no on-disk event log compatibility concern.
- Generic "sensor" wrapper with an inner oneof (`Sensor.reading`). Considered
  and rejected: breaks the established pattern where every entity kind is a
  sibling at `Attributes.kind`.

## Proto change

`proto/gohome/entity/v1/attributes.proto`:

```proto
message Attributes {
  // Old name retired in favor of `numeric_sensor` for symmetry with
  // `binary_sensor`. Field number 12 stays (wire-compatible). Per
  // dev/proto-hygiene.md rule 3, we reserve the old name to prevent
  // accidental reuse.
  reserved "sensor";

  oneof kind {
    Light          light           = 10;
    Switch         switch_device   = 11;
    NumericSensor  numeric_sensor  = 12;  // was: Sensor sensor; field number stays
    BinarySensor   binary_sensor   = 13;  // new
  }
  bool available = 90;
}

message NumericSensor {  // renamed from `Sensor`
  string unit  = 1;
  double value = 2;
}

message BinarySensor {
  // on encodes the active reading: motion detected, contact closed,
  // water present, etc. Drivers may invert at the source if a device
  // reports the inverse polarity natively (the inversion choice is the
  // driver's, not the proto's).
  bool on = 1;
}
```

Field number 12 is preserved for `numeric_sensor`, so the wire format is
backwards-compatible for any in-flight events. Field rename from `sensor` to
`numeric_sensor` is a Go API break (`GetSensor()` becomes
`GetNumericSensor()`); fixed up in the same change.

## Code changes

`internal/daemon/daemon.go` line 585 — the only non-generated reference:

```go
// before
case *entityv1.Attributes_Sensor:
// after
case *entityv1.Attributes_NumericSensor:
```

`gen/gohome/entity/v1/attributes.pb.go` regenerated via `buf generate` (the
repo's proto tooling, configured by `buf.gen.yaml` and `buf.yaml` at the
repo root).

## Proto version note

The `gohome.entity.v1` package is at `v1`, which per `dev/proto-hygiene.md`
rule 5 is a one-way door for **wire-breaking** changes. This spec is
wire-compatible: field number 12 stays, only its name changes
(`sensor` → `numeric_sensor`), and field 13 is a new addition. No wire-format
contract is broken; downstream Go code does break and is fixed in the same
change.

## Testing

No new tests. Existing daemon tests cover the switch case; if `go test ./...`
passes, the rename is correct. The new `BinarySensor` variant is a
declaration-only change with no behavior — its first real test arrives with
the Z2M driver spec, which depends on this one.

## Rollout

Single PR: proto edit + regenerate + daemon fix-up + tree builds. Lands before
the Z2M driver spec begins implementation.

## Open items

None.
