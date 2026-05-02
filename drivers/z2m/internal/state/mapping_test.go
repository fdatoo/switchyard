package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard-driverkit/driver"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/z2m"
)

// loadFixture is shared with reconcile_test; keep it in this file.
func loadFixture(t *testing.T) []z2m.Device {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "z2m", "testdata", "bridge_devices.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var devices []z2m.Device
	if err := json.Unmarshal(raw, &devices); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return devices
}

func deviceByName(t *testing.T, devices []z2m.Device, name string) z2m.Device {
	t.Helper()
	for _, d := range devices {
		if d.FriendlyName == name {
			return d
		}
	}
	t.Fatalf("device %q not found", name)
	return z2m.Device{}
}

func entityIDs(out []EntityResult) []string {
	ids := make([]string, len(out))
	for i, r := range out {
		ids[i] = r.EntityID
	}
	sort.Strings(ids)
	return ids
}

func TestEntitiesForColorLight(t *testing.T) {
	dev := deviceByName(t, loadFixture(t), "kitchen_light")
	got := EntitiesFor(dev)
	if len(got) != 1 {
		t.Fatalf("kitchen_light entities: got %d, want 1", len(got))
	}
	r := got[0]
	if r.EntityID != "light.z2m_01234abc" {
		t.Errorf("entityID: got %q, want %q", r.EntityID, "light.z2m_01234abc")
	}
	if r.Spec.EntityType != "light" {
		t.Errorf("entity type: got %q, want %q", r.Spec.EntityType, "light")
	}
	wantCaps := []string{"set_brightness", "set_color", "set_color_temp", "turn_off", "turn_on"}
	gotCaps := append([]string(nil), r.Spec.Capabilities...)
	sort.Strings(gotCaps)
	if !reflect.DeepEqual(gotCaps, wantCaps) {
		t.Errorf("capabilities: got %v, want %v", gotCaps, wantCaps)
	}
}

func TestEntitiesForMultiSensor(t *testing.T) {
	dev := deviceByName(t, loadFixture(t), "hallway_motion")
	got := EntitiesFor(dev)
	want := []string{
		"binary_sensor.z2m_09876543_occupancy",
		"numeric_sensor.z2m_09876543_battery",
		"numeric_sensor.z2m_09876543_humidity",
		"numeric_sensor.z2m_09876543_temperature",
	}
	if !reflect.DeepEqual(entityIDs(got), want) {
		t.Errorf("entity ids: got %v, want %v", entityIDs(got), want)
	}
	// linkquality and voltage are blocked.
	for _, r := range got {
		if strings.Contains(r.EntityID, "linkquality") || strings.Contains(r.EntityID, "voltage") {
			t.Errorf("blocked property surfaced: %q", r.EntityID)
		}
	}
}

func TestEntitiesForContactSensor(t *testing.T) {
	dev := deviceByName(t, loadFixture(t), "front_door")
	got := EntitiesFor(dev)
	want := []string{
		"binary_sensor.z2m_02468ace_contact",
		"numeric_sensor.z2m_02468ace_battery",
	}
	if !reflect.DeepEqual(entityIDs(got), want) {
		t.Errorf("entity ids: got %v, want %v", entityIDs(got), want)
	}
}

func TestEntitiesForSmartPlugSkipsWritableState(t *testing.T) {
	dev := deviceByName(t, loadFixture(t), "office_plug")
	got := EntitiesFor(dev)
	want := []string{"numeric_sensor.z2m_11223344_power"}
	if !reflect.DeepEqual(entityIDs(got), want) {
		t.Errorf("entity ids: got %v, want %v", entityIDs(got), want)
	}
}

func TestEntitiesForCoordinator(t *testing.T) {
	dev := deviceByName(t, loadFixture(t), "Coordinator")
	got := EntitiesFor(dev)
	if len(got) != 0 {
		t.Errorf("coordinator entities: got %d, want 0 (%v)", len(got), entityIDs(got))
	}
}

func TestEntitiesForPropertyToEntityMap(t *testing.T) {
	// Each result records which Z2M property feeds it. main uses this
	// to fan a state-topic payload out to the right entity IDs.
	dev := deviceByName(t, loadFixture(t), "hallway_motion")
	got := EntitiesFor(dev)
	for _, r := range got {
		if r.Property == "" {
			t.Errorf("sensor result missing Property: %+v", r)
		}
	}
	// Light entities have a synthetic Property="" (multiple properties
	// feed one entity) — verified separately.
	light := deviceByName(t, loadFixture(t), "kitchen_light")
	for _, r := range EntitiesFor(light) {
		if r.Property != "" {
			t.Errorf("light result Property: got %q, want \"\"", r.Property)
		}
	}
}

// Round-trip sanity: an EntitySpec carries an InitialState whose Kind
// matches the entity type.
func TestEntitiesForInitialStateKinds(t *testing.T) {
	devices := loadFixture(t)
	for _, dev := range devices {
		for _, r := range EntitiesFor(dev) {
			if r.Spec.InitialState == nil {
				continue
			}
			switch r.Spec.EntityType {
			case "light":
				if _, ok := r.Spec.InitialState.Kind.(*entityv1.Attributes_Light); !ok {
					t.Errorf("%s: InitialState.Kind not Light", r.EntityID)
				}
			case "numeric_sensor":
				if _, ok := r.Spec.InitialState.Kind.(*entityv1.Attributes_NumericSensor); !ok {
					t.Errorf("%s: InitialState.Kind not NumericSensor", r.EntityID)
				}
			case "binary_sensor":
				if _, ok := r.Spec.InitialState.Kind.(*entityv1.Attributes_BinarySensor); !ok {
					t.Errorf("%s: InitialState.Kind not BinarySensor", r.EntityID)
				}
			}
		}
	}
	_ = driver.EntitySpec{} // ensure the import is used
}
