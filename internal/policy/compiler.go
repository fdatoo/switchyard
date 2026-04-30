package policy

import (
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
)

type RawPolicy struct {
	Name     string
	Subjects []RoleSlug
	Allow    []RawRule
	Deny     []RawRule
}

type RawRule struct {
	Verbs    []Verb
	Targets  RawSelector
	Services []string
}

type RawSelector struct {
	Areas     []string
	Classes   []string
	EntityIDs []string
	MatchAny  bool
}

type RoleGraph struct {
	Roles map[RoleSlug]RoleNode
}

type RoleNode struct {
	Slug     RoleSlug
	Inherits []RoleSlug
}

type CatalogAction struct {
	Service  string
	Method   string
	Verb     Verb
	IsEntity bool
}

type ActionCatalog struct {
	Actions []CatalogAction
}

var generation uint64

func Compile(rawPolicies []RawPolicy, roles RoleGraph, areas AreaTree, catalog ActionCatalog) (*Compiled, error) {
	inheritance, err := resolveRoleInheritance(roles)
	if err != nil {
		return nil, err
	}
	out := &Compiled{
		Generation:      atomic.AddUint64(&generation, 1),
		ActionAllowlist: map[RoleVerb]map[ActionKey]struct{}{},
		AllowRules:      map[RoleSlug][]CompiledRule{},
		DenyRules:       map[RoleSlug][]CompiledRule{},
		AreaExpansion:   map[SelectorHash]map[AreaSlug]struct{}{},
		RoleInheritance: inheritance,
	}
	catalogIndex := indexCatalog(catalog)
	knownServices := map[string]struct{}{}
	for _, a := range catalog.Actions {
		knownServices[a.Service] = struct{}{}
	}

	for _, rp := range rawPolicies {
		for _, role := range rp.Subjects {
			for _, allow := range rp.Allow {
				cr, err := compileRule(rp.Name, allow, areas, catalogIndex, knownServices, out, false)
				if err != nil {
					return nil, err
				}
				out.AllowRules[role] = append(out.AllowRules[role], cr)
				addToActionAllowlist(out, role, allow, catalog)
			}
			for _, deny := range rp.Deny {
				cr, err := compileRule(rp.Name, deny, areas, catalogIndex, knownServices, out, true)
				if err != nil {
					return nil, err
				}
				out.DenyRules[role] = append(out.DenyRules[role], cr)
			}
		}
	}
	return out, nil
}

func resolveRoleInheritance(g RoleGraph) (map[RoleSlug]map[RoleSlug]struct{}, error) {
	out := map[RoleSlug]map[RoleSlug]struct{}{}
	for slug := range g.Roles {
		seen := map[RoleSlug]struct{}{}
		var visit func(RoleSlug, map[RoleSlug]struct{}) error
		visit = func(s RoleSlug, path map[RoleSlug]struct{}) error {
			if _, cycle := path[s]; cycle {
				return fmt.Errorf("%w through %q", ErrCycle, s)
			}
			if _, done := seen[s]; done {
				return nil
			}
			seen[s] = struct{}{}
			path[s] = struct{}{}
			for _, parent := range g.Roles[s].Inherits {
				if err := visit(parent, path); err != nil {
					return err
				}
			}
			delete(path, s)
			return nil
		}
		if err := visit(slug, map[RoleSlug]struct{}{}); err != nil {
			return nil, err
		}
		out[slug] = seen
	}
	return out, nil
}

func compileRule(policyName string, rr RawRule, areas AreaTree,
	catalog map[string][]CatalogAction, knownServices map[string]struct{},
	sink *Compiled, isDeny bool) (CompiledRule, error) {

	cr := CompiledRule{PolicyName: policyName}
	if len(rr.Verbs) > 0 {
		cr.Verbs = map[Verb]struct{}{}
		for _, v := range rr.Verbs {
			cr.Verbs[v] = struct{}{}
		}
	}
	if len(rr.Services) > 0 {
		cr.Services = map[string]struct{}{}
		for _, s := range rr.Services {
			if _, ok := knownServices[s]; !ok {
				return CompiledRule{}, fmt.Errorf("policy: %s references unknown service %q", policyName, s)
			}
			cr.Services[s] = struct{}{}
		}
	}
	sel, err := compileSelector(rr.Targets, areas, sink)
	if err != nil {
		return CompiledRule{}, err
	}
	cr.Targets = sel

	if isDeny && len(rr.Services) > 0 && !selectorIsEmpty(sel) {
		for _, svc := range rr.Services {
			for _, a := range catalog[svc] {
				if !a.IsEntity {
					return CompiledRule{}, fmt.Errorf(
						"policy: %s deny rule on non-entity action %s.%s must not have target selector",
						policyName, a.Service, a.Method)
				}
			}
		}
	}
	return cr, nil
}

func compileSelector(rs RawSelector, areas AreaTree, sink *Compiled) (CompiledSelector, error) {
	if rs.MatchAny {
		return CompiledSelector{MatchAny: true}, nil
	}
	hash := HashSelector(rs.Areas, rs.Classes, rs.EntityIDs)
	expansion, ok := sink.AreaExpansion[hash]
	if !ok {
		expansion = ExpandAreas(areas, rs.Areas)
		sink.AreaExpansion[hash] = expansion
	}
	cs := CompiledSelector{Hash: hash, AreaSet: expansion}
	if len(rs.Classes) > 0 {
		cs.ClassSet = map[string]struct{}{}
		for _, c := range rs.Classes {
			cs.ClassSet[c] = struct{}{}
		}
	}
	if len(rs.EntityIDs) > 0 {
		cs.EntitySet = map[string]struct{}{}
		for _, id := range rs.EntityIDs {
			cs.EntitySet[id] = struct{}{}
		}
	}
	return cs, nil
}

func selectorIsEmpty(cs CompiledSelector) bool {
	return !cs.MatchAny && len(cs.AreaSet) == 0 && len(cs.ClassSet) == 0 && len(cs.EntitySet) == 0
}

func indexCatalog(c ActionCatalog) map[string][]CatalogAction {
	out := map[string][]CatalogAction{}
	for _, a := range c.Actions {
		out[a.Service] = append(out[a.Service], a)
	}
	return out
}

func addToActionAllowlist(out *Compiled, role RoleSlug, rr RawRule, catalog ActionCatalog) {
	verbs := rr.Verbs
	if len(verbs) == 0 {
		verbs = AllVerbs
	}
	for _, a := range catalog.Actions {
		if len(rr.Services) > 0 {
			ok := false
			for _, svc := range rr.Services {
				if svc == a.Service {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		for _, v := range verbs {
			if v != a.Verb {
				continue
			}
			key := RoleVerb{Role: role, Verb: v}
			if _, ok := out.ActionAllowlist[key]; !ok {
				out.ActionAllowlist[key] = map[ActionKey]struct{}{}
			}
			out.ActionAllowlist[key][MakeActionKey(a.Service, a.Method)] = struct{}{}
		}
	}
}

// PermitsAction is the exported test/explain surface.
func (c *Compiled) PermitsAction(role RoleSlug, verb Verb, key ActionKey) bool {
	return c.permitsAction(role, verb, key)
}

// ErrCycle is returned when role inheritance contains a cycle.
var ErrCycle = errors.New("policy: role inheritance cycle")

// SortedActionsForRole returns sorted ActionKeys permitted for role+verb.
func (c *Compiled) SortedActionsForRole(role RoleSlug, verb Verb) []ActionKey {
	set, ok := c.ActionAllowlist[RoleVerb{Role: role, Verb: verb}]
	if !ok {
		return nil
	}
	out := make([]ActionKey, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
