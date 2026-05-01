package config

import (
	"crypto/sha256"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

func makeInst(id, driverName string, params []byte) *configpb.DriverInstanceConfig {
	h := sha256.Sum256(params)
	return &configpb.DriverInstanceConfig{Id: id, DriverName: driverName, ConfigHash: h[:], Params: params}
}

func TestDiff_AddedInstance(t *testing.T) {
	old := &configpb.ConfigSnapshot{}
	next := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main"}`))},
	}
	d := Diff(old, next)
	if len(d.DriverInstancesAdded) != 1 || d.DriverInstancesAdded[0] != "hue-main" {
		t.Fatalf("expected 1 added, got %+v", d)
	}
}

func TestDiff_RemovedInstance(t *testing.T) {
	old := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main"}`))},
	}
	next := &configpb.ConfigSnapshot{}
	d := Diff(old, next)
	if len(d.DriverInstancesRemoved) != 1 {
		t.Fatalf("expected 1 removed, got %+v", d)
	}
}

func TestDiff_ChangedInstance(t *testing.T) {
	old := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main","bridgeIP":"1.2.3.4"}`))},
	}
	next := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", []byte(`{"id":"hue-main","bridgeIP":"5.6.7.8"}`))},
	}
	d := Diff(old, next)
	if len(d.DriverInstancesChanged) != 1 {
		t.Fatalf("expected 1 changed, got %+v", d)
	}
}

func TestDiff_NoChange(t *testing.T) {
	params := []byte(`{"id":"hue-main"}`)
	old := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", params)},
	}
	next := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{makeInst("hue-main", "hue", params)},
	}
	d := Diff(old, next)
	if len(d.DriverInstancesAdded)+len(d.DriverInstancesRemoved)+len(d.DriverInstancesChanged) != 0 {
		t.Fatalf("expected no diff, got %+v", d)
	}
}
