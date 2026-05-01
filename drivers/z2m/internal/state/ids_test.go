package state

import "testing"

func TestEntityIDLight(t *testing.T) {
	got := EntityID("0x00158d0001234abc", "light", "")
	want := "light.z2m_01234abc"
	if got != want {
		t.Errorf("EntityID light: got %q, want %q", got, want)
	}
}

func TestEntityIDNumericSensor(t *testing.T) {
	got := EntityID("0x00158d0009876543", "numeric_sensor", "temperature")
	want := "numeric_sensor.z2m_09876543_temperature"
	if got != want {
		t.Errorf("EntityID numeric_sensor: got %q, want %q", got, want)
	}
}

func TestEntityIDBinarySensor(t *testing.T) {
	got := EntityID("0x00158d0002468ace", "binary_sensor", "contact")
	want := "binary_sensor.z2m_02468ace_contact"
	if got != want {
		t.Errorf("EntityID binary_sensor: got %q, want %q", got, want)
	}
}

func TestEntityIDShortIEEE(t *testing.T) {
	// IEEEs shorter than 8 hex chars (post-prefix-strip) are passed through.
	got := EntityID("0x12", "light", "")
	want := "light.z2m_12"
	if got != want {
		t.Errorf("short IEEE: got %q, want %q", got, want)
	}
}

func TestEntityIDNoOxPrefix(t *testing.T) {
	// Already without "0x" prefix.
	got := EntityID("00158d0001234abc", "light", "")
	want := "light.z2m_01234abc"
	if got != want {
		t.Errorf("no-prefix: got %q, want %q", got, want)
	}
}

func TestEntityIDCollisionFreeAcrossFixture(t *testing.T) {
	// Sanity check: the fixture's IEEEs all yield distinct IDs even
	// with property suffixes.
	cases := []string{
		EntityID("0x00158d0001234abc", "light", ""),
		EntityID("0x00158d0009876543", "binary_sensor", "occupancy"),
		EntityID("0x00158d0009876543", "numeric_sensor", "temperature"),
		EntityID("0x00158d0009876543", "numeric_sensor", "humidity"),
		EntityID("0x00158d0009876543", "numeric_sensor", "battery"),
		EntityID("0x00158d0002468ace", "binary_sensor", "contact"),
		EntityID("0x00158d0002468ace", "numeric_sensor", "battery"),
		EntityID("0x00158d0011223344", "numeric_sensor", "power"),
	}
	seen := map[string]bool{}
	for _, id := range cases {
		if seen[id] {
			t.Errorf("collision on %q", id)
		}
		seen[id] = true
	}
}
