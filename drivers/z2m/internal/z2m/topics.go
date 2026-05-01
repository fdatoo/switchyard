package z2m

import "fmt"

// Topics holds the four topics belonging to a single Z2M device, used
// by the reconciler so the caller can subscribe/unsubscribe in one
// pass.
type Topics struct {
	State        string // <base>/<friendly>
	Set          string // <base>/<friendly>/set
	Availability string // <base>/<friendly>/availability
}

// BridgeDevices returns <base>/bridge/devices — the retained topic
// listing every paired device. Subscribing replays the current list.
func BridgeDevices(base string) string { return base + "/bridge/devices" }

// BridgeState returns <base>/bridge/state — "online"/"offline" for the
// Z2M bridge process itself.
func BridgeState(base string) string { return base + "/bridge/state" }

// BridgeEvent returns <base>/bridge/event — device-level lifecycle
// events (paired, removed, interview started/finished). Logged only
// in v0.1; the bridge/devices retained topic drives reconciliation.
func BridgeEvent(base string) string { return base + "/bridge/event" }

// DeviceTopics returns the per-device topic bundle for friendlyName.
func DeviceTopics(base, friendlyName string) Topics {
	prefix := fmt.Sprintf("%s/%s", base, friendlyName)
	return Topics{
		State:        prefix,
		Set:          prefix + "/set",
		Availability: prefix + "/availability",
	}
}
