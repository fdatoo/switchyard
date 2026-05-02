package config

import (
	"fmt"
	"strings"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

// RegistryQuerier checks whether a driver name is known to the registry.
type RegistryQuerier interface {
	DriverExists(name string) bool
}

// Compile validates cross-references in a snapshot. Returns all errors found.
func Compile(snap *configpb.ConfigSnapshot, querier RegistryQuerier) []ValidationError {
	var errs []ValidationError

	seenInstances := map[string]bool{}
	for _, di := range snap.GetDriverInstances() {
		if seenInstances[di.GetId()] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("driverInstances[%s]", di.GetId()),
				Message: "duplicate driver instance id",
			})
		}
		seenInstances[di.GetId()] = true

		if querier != nil && di.GetDriverName() != "" && !querier.DriverExists(di.GetDriverName()) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("driverInstances[%s].driverName", di.GetId()),
				Message: fmt.Sprintf("unknown driver %q — is the driver binary registered?", di.GetDriverName()),
			})
		}
	}

	seenEntities := map[string]bool{}
	for _, e := range snap.GetEntities() {
		if seenEntities[e.GetId()] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("entities[%s]", e.GetId()),
				Message: "duplicate entity id",
			})
		}
		seenEntities[e.GetId()] = true

		if !isValidEntityID(e.GetId()) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("entities[%s].id", e.GetId()),
				Message: `entity id must be "<type>.<name>" e.g. "light.living_room"`,
			})
		}
	}

	seenAutomations := map[string]bool{}
	for _, a := range snap.GetAutomations() {
		if seenAutomations[a.GetId()] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("automations[%s]", a.GetId()),
				Message: "duplicate automation id",
			})
		}
		seenAutomations[a.GetId()] = true
	}

	scriptNames := map[string]bool{}
	for _, s := range snap.GetScripts() {
		name := s.GetName()
		if name == "" {
			continue
		}
		if scriptNames[name] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("scripts[%s]", name),
				Message: "duplicate script name",
			})
		}
		scriptNames[name] = true
	}
	for _, a := range snap.GetAutomations() {
		for i, ac := range a.GetActions() {
			collectScriptRefs(ac, fmt.Sprintf("automations[%s].actions[%d]", a.GetId(), i), scriptNames, &errs)
		}
	}

	return errs
}

func collectScriptRefs(ac *configpb.ActionConfig, path string, known map[string]bool, errs *[]ValidationError) {
	if sc := ac.GetScript(); sc != nil && !known[sc.GetName()] {
		*errs = append(*errs, ValidationError{
			Field:   path + ".script.name",
			Message: fmt.Sprintf("unknown script %q", sc.GetName()),
		})
	}
	if seq := ac.GetSequence(); seq != nil {
		for i, c := range seq.GetActions() {
			collectScriptRefs(c, fmt.Sprintf("%s.sequence.actions[%d]", path, i), known, errs)
		}
	}
	if par := ac.GetParallel(); par != nil {
		for i, c := range par.GetActions() {
			collectScriptRefs(c, fmt.Sprintf("%s.parallel.actions[%d]", path, i), known, errs)
		}
	}
}

func isValidEntityID(id string) bool {
	parts := strings.SplitN(id, ".", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
