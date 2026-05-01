package z2m

import "testing"

func TestBridgeTopics(t *testing.T) {
	const base = "zigbee2mqtt"
	cases := []struct {
		fn   string
		got  string
		want string
	}{
		{"BridgeDevices", BridgeDevices(base), "zigbee2mqtt/bridge/devices"},
		{"BridgeState", BridgeState(base), "zigbee2mqtt/bridge/state"},
		{"BridgeEvent", BridgeEvent(base), "zigbee2mqtt/bridge/event"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.fn, tc.got, tc.want)
		}
	}
}

func TestDeviceTopics(t *testing.T) {
	got := DeviceTopics("zigbee2mqtt", "kitchen_light")
	want := Topics{
		State:        "zigbee2mqtt/kitchen_light",
		Set:          "zigbee2mqtt/kitchen_light/set",
		Availability: "zigbee2mqtt/kitchen_light/availability",
	}
	if got != want {
		t.Errorf("DeviceTopics: got %+v, want %+v", got, want)
	}
}

func TestDeviceTopicsCustomBase(t *testing.T) {
	got := DeviceTopics("home/zigbee", "office")
	if got.Set != "home/zigbee/office/set" {
		t.Errorf("custom base: got %q", got.Set)
	}
}
