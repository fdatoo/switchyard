package z2m

import "encoding/json"

// Device is one element of <base>/bridge/devices.
//
// Only the fields the driver consumes are modelled. Z2M sends many
// more (manufacturer, model_id, network_address, power_source, ...);
// they decode into nothing harmful and are ignored.
type Device struct {
	IEEEAddress  string     `json:"ieee_address"`
	FriendlyName string     `json:"friendly_name"`
	Type         string     `json:"type"` // "Coordinator" | "EndDevice" | "Router"
	Definition   Definition `json:"definition"`
}

// Definition wraps the device's exposes tree.
type Definition struct {
	Vendor      string   `json:"vendor"`
	Model       string   `json:"model"`
	Description string   `json:"description"`
	Exposes     []Expose `json:"exposes"`
}

// Expose is the recursive node type that describes one capability or
// composite. Z2M's exposes tree mixes leaf types ("numeric", "binary",
// "enum", "text") with composites ("light", "switch", "climate", "lock",
// "fan", "cover") whose Features hold the real leaves.
//
// The fields decoded here are the union of what leaves and composites
// use. Empty fields are ignored at read time.
type Expose struct {
	Type        string   `json:"type"`
	Name        string   `json:"name,omitempty"`
	Property    string   `json:"property,omitempty"`
	Description string   `json:"description,omitempty"`
	Access      uint8    `json:"access,omitempty"` // bitmask: 1=published, 2=settable, 4=gettable
	Unit        string   `json:"unit,omitempty"`
	ValueMin    *float64 `json:"value_min,omitempty"`
	ValueMax    *float64 `json:"value_max,omitempty"`
	ValueOn     any      `json:"value_on,omitempty"`  // string or bool
	ValueOff    any      `json:"value_off,omitempty"` // string or bool
	Features    []Expose `json:"features,omitempty"`
}

// AccessPublished reports whether bit 0 of Access is set (Z2M
// publishes the property on the state topic). Effectively "is this
// readable in our context".
func (e Expose) AccessPublished() bool { return e.Access&0x01 != 0 }

// AccessSettable reports whether bit 1 of Access is set (settable via
// /set). Used to skip writable non-light properties (smart-plug state)
// in v0.1.
func (e Expose) AccessSettable() bool { return e.Access&0x02 != 0 }

// BridgeStatePayload is the payload of <base>/bridge/state.
type BridgeStatePayload struct {
	State string `json:"state"` // "online" | "offline"
}

// AvailabilityState is the payload of <base>/<friendly>/availability.
type AvailabilityState struct {
	State string `json:"state"` // "online" | "offline"
}

// StatePayload is the per-device state-push payload. Each device's
// shape differs, so the values are captured raw. Use
// state.MergeState to interpret each property.
type StatePayload map[string]json.RawMessage
