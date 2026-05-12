package config

import (
	"fmt"
	"path/filepath"
	"strings"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

// mergeDiscovered folds discovered per-file configs into snap. Returns the
// merged snapshot, a slice of non-fatal ValidationErrors, and a non-nil
// error iff any duplicate-id conflicts were detected. On hard error, the
// returned snapshot is the partial merge — callers should treat it as
// untrustworthy.
func mergeDiscovered(snap *configpb.ConfigSnapshot, disc discoveryResult) (*configpb.ConfigSnapshot, []ValidationError, error) {
	var errs []ValidationError
	hardErr := false

	inlineAutos := map[string]bool{}
	for _, a := range snap.GetAutomations() {
		inlineAutos[a.GetId()] = true
	}
	for _, d := range disc.Automations {
		expectedID := strings.TrimSuffix(filepath.Base(d.Path), ".pkl")
		if d.Config.GetId() != expectedID {
			errs = append(errs, ValidationError{
				Code:    "filename_id_mismatch",
				File:    d.Path,
				Field:   fmt.Sprintf("automations[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("filename %q does not match id %q", expectedID, d.Config.GetId()),
			})
			continue
		}
		if inlineAutos[d.Config.GetId()] {
			errs = append(errs, ValidationError{
				Code:    "duplicate_id",
				File:    d.Path,
				Field:   fmt.Sprintf("automations[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("id %q is already declared inline in main.pkl", d.Config.GetId()),
			})
			hardErr = true
			continue
		}
		snap.Automations = append(snap.Automations, d.Config)
	}

	inlineAreas := map[string]bool{}
	for _, a := range snap.GetAreas() {
		inlineAreas[a.GetId()] = true
	}
	for _, d := range disc.Areas {
		expectedID := strings.TrimSuffix(filepath.Base(d.Path), ".pkl")
		if d.Config.GetId() != expectedID {
			errs = append(errs, ValidationError{
				Code:    "filename_id_mismatch",
				File:    d.Path,
				Field:   fmt.Sprintf("areas[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("filename %q does not match id %q", expectedID, d.Config.GetId()),
			})
			continue
		}
		if inlineAreas[d.Config.GetId()] {
			errs = append(errs, ValidationError{
				Code:    "duplicate_id",
				File:    d.Path,
				Field:   fmt.Sprintf("areas[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("id %q is already declared inline in main.pkl", d.Config.GetId()),
			})
			hardErr = true
			continue
		}
		snap.Areas = append(snap.Areas, d.Config)
	}

	inlineScenes := map[string]bool{}
	for _, s := range snap.GetScenes() {
		inlineScenes[s.GetId()] = true
	}
	for _, d := range disc.Scenes {
		expectedID := strings.TrimSuffix(filepath.Base(d.Path), ".pkl")
		if d.Config.GetId() != expectedID {
			errs = append(errs, ValidationError{
				Code:    "filename_id_mismatch",
				File:    d.Path,
				Field:   fmt.Sprintf("scenes[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("filename %q does not match id %q", expectedID, d.Config.GetId()),
			})
			continue
		}
		if inlineScenes[d.Config.GetId()] {
			errs = append(errs, ValidationError{
				Code:    "duplicate_id",
				File:    d.Path,
				Field:   fmt.Sprintf("scenes[%s]", d.Config.GetId()),
				Message: fmt.Sprintf("id %q is already declared inline in main.pkl", d.Config.GetId()),
			})
			hardErr = true
			continue
		}
		snap.Scenes = append(snap.Scenes, d.Config)
	}

	if snap.EntityAreas == nil && len(disc.EntityAreas) > 0 {
		snap.EntityAreas = make(map[string]string, len(disc.EntityAreas))
	}
	for k, v := range disc.EntityAreas {
		if existing, ok := snap.EntityAreas[k]; ok && existing != v {
			errs = append(errs, ValidationError{
				Code:    "duplicate_entity_area",
				File:    "entity-areas.pkl",
				Field:   fmt.Sprintf("entityAreas[%s]", k),
				Message: fmt.Sprintf("key %q is already mapped to %q inline; file maps it to %q", k, existing, v),
			})
			hardErr = true
			continue
		}
		snap.EntityAreas[k] = v
	}

	if hardErr {
		return snap, errs, fmt.Errorf("config merge failed: duplicate id(s) — see validation errors")
	}
	return snap, errs, nil
}
