package state

import "strings"

// EntityID returns the gohome entity ID for a Z2M (device, property)
// pair. The last 8 hex chars of the IEEE address are used as the
// stable identifier — short enough to scan in logs, immune to
// friendly_name changes, and unambiguous within one Z2M instance.
//
// Lights collapse all light properties (state, brightness, color_temp,
// color) into a single light.* entity, so prop is empty for lights.
// Sensors append _<prop> so a multi-sensor's properties get distinct
// IDs.
func EntityID(ieee, kind, prop string) string {
	last8 := lastHex8(ieee)
	if prop == "" {
		return kind + ".z2m_" + last8
	}
	return kind + ".z2m_" + last8 + "_" + prop
}

func lastHex8(ieee string) string {
	id := strings.TrimPrefix(ieee, "0x")
	if len(id) > 8 {
		id = id[len(id)-8:]
	}
	return id
}
