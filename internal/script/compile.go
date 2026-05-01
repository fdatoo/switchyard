package script

import (
	"fmt"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

// CompileScripts turns the snapshot's ScriptConfig list into an immutable
// name-keyed map of runtime Script values. Returns an aggregated CompileError
// with every problem found so authors get a full list at validation time.
func CompileScripts(snap *configpb.ConfigSnapshot) (map[string]*Script, error) {
	out := make(map[string]*Script)
	var errs []*ItemError

	for i, s := range snap.GetScripts() {
		name := s.GetName()
		if name == "" {
			errs = append(errs, &ItemError{Name: fmt.Sprintf("scripts[%d]", i), Path: "name", Reason: "empty"})
			continue
		}
		if _, dup := out[name]; dup {
			errs = append(errs, &ItemError{Name: name, Path: "name", Reason: "duplicate script name"})
			continue
		}

		handler := s.GetHandler()
		if err := ghstarlark.ParseOnly(handler, false); err != nil {
			errs = append(errs, &ItemError{Name: name, Path: "handler", Reason: err.Error()})
			continue
		}

		params := make([]Param, 0, len(s.GetParams()))
		okParams := true
		for pi, p := range s.GetParams() {
			rp := Param{
				Name:     p.GetName(),
				Type:     p.GetType(),
				Required: p.GetRequired(),
			}
			if rp.Name == "" {
				errs = append(errs, &ItemError{Name: name, Path: fmt.Sprintf("params[%d].name", pi), Reason: "empty"})
				okParams = false
				continue
			}
			if rp.Type == configpb.ScriptParam_TYPE_UNSPECIFIED {
				errs = append(errs, &ItemError{Name: name, Path: fmt.Sprintf("params[%d].type", pi), Reason: "unspecified type"})
				okParams = false
				continue
			}
			if p.GetDefault() != "" {
				v, err := rp.Coerce(p.GetDefault())
				if err != nil {
					errs = append(errs, &ItemError{Name: name, Path: fmt.Sprintf("params[%d].default", pi), Reason: err.Error()})
					okParams = false
					continue
				}
				rp.HasDefault = true
				rp.Default = v
			}
			params = append(params, rp)
		}
		if !okParams {
			continue
		}

		out[name] = &Script{
			Name:    name,
			Params:  params,
			Handler: handler,
		}
	}

	if len(errs) > 0 {
		return nil, &CompileError{Items: errs}
	}
	return out, nil
}
