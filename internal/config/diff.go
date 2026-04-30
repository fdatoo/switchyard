package config

import (
	"bytes"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
)

// ConfigDiff describes what changed between two snapshots.
type ConfigDiff struct {
	DriverInstancesAdded   []string
	DriverInstancesRemoved []string
	DriverInstancesChanged []string
	AutomationsChanged     int
	DashboardsChanged      int
}

// Diff computes the minimal changeset between old and next snapshots.
// Nil old is treated as empty (first-ever apply).
func Diff(old, next *configpb.ConfigSnapshot) *ConfigDiff {
	d := &ConfigDiff{}

	oldInsts := map[string][]byte{}
	if old != nil {
		for _, di := range old.GetDriverInstances() {
			oldInsts[di.GetId()] = di.GetConfigHash()
		}
	}

	nextIDs := map[string]bool{}
	for _, di := range next.GetDriverInstances() {
		nextIDs[di.GetId()] = true
		oldHash, existed := oldInsts[di.GetId()]
		if !existed {
			d.DriverInstancesAdded = append(d.DriverInstancesAdded, di.GetId())
		} else if !bytes.Equal(oldHash, di.GetConfigHash()) {
			d.DriverInstancesChanged = append(d.DriverInstancesChanged, di.GetId())
		}
	}
	for id := range oldInsts {
		if !nextIDs[id] {
			d.DriverInstancesRemoved = append(d.DriverInstancesRemoved, id)
		}
	}

	oldAutoCount := 0
	if old != nil {
		oldAutoCount = len(old.GetAutomations())
	}
	if len(next.GetAutomations()) != oldAutoCount {
		d.AutomationsChanged = len(next.GetAutomations()) - oldAutoCount
	}

	oldDashCount := 0
	if old != nil {
		oldDashCount = len(old.GetDashboards())
	}
	if len(next.GetDashboards()) != oldDashCount {
		d.DashboardsChanged = len(next.GetDashboards()) - oldDashCount
	}

	return d
}
