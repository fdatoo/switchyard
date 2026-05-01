package config

import (
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

type fakeQuerier struct {
	knownDrivers map[string]bool
}

func (f *fakeQuerier) DriverExists(name string) bool {
	return f.knownDrivers[name]
}

func TestCompile_DuplicateDriverInstanceID(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue"},
			{Id: "hue-main", DriverName: "hue"},
		},
	}
	errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{"hue": true}})
	if len(errs) == 0 {
		t.Fatal("expected duplicate ID error, got none")
	}
}

func TestCompile_UnknownDriver(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "nonexistent"},
		},
	}
	errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{}})
	if len(errs) == 0 {
		t.Fatal("expected unknown driver error, got none")
	}
}

func TestCompile_InvalidEntityID(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		Entities: []*configpb.EntityConfig{
			{Id: "invalid_no_dot", FriendlyName: "Bad"},
		},
	}
	errs := Compile(snap, &fakeQuerier{})
	if len(errs) == 0 {
		t.Fatal("expected entity ID error, got none")
	}
}

func TestCompile_Valid(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue"},
		},
		Entities: []*configpb.EntityConfig{
			{Id: "light.living_room", FriendlyName: "Living Room"},
		},
	}
	errs := Compile(snap, &fakeQuerier{knownDrivers: map[string]bool{"hue": true}})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}
