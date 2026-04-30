package policy

// Target is what the runtime asks the matcher about.
type Target struct {
	Kind  string // "entity" | "list" | ""
	ID    string
	Area  AreaSlug
	Class string
}

// AreaTree provides parent → children edges for the area hierarchy.
type AreaTree struct {
	Children map[AreaSlug][]AreaSlug
	Parent   map[AreaSlug]AreaSlug
}

// SelectorMatches returns true iff the target falls inside the compiled selector.
// Empty selector matches nothing (default-deny safe). MatchAny short-circuits to true.
func SelectorMatches(sel CompiledSelector, t Target) bool {
	if sel.MatchAny {
		return true
	}
	if len(sel.AreaSet) > 0 {
		if _, ok := sel.AreaSet[t.Area]; ok {
			return true
		}
	}
	if len(sel.ClassSet) > 0 {
		if _, ok := sel.ClassSet[t.Class]; ok {
			return true
		}
	}
	if len(sel.EntitySet) > 0 {
		if _, ok := sel.EntitySet[t.ID]; ok {
			return true
		}
	}
	return false
}

// ExpandAreas resolves area-slug literals (possibly including "*") into a flat
// set including all transitive descendants.
func ExpandAreas(tree AreaTree, decls []string) map[AreaSlug]struct{} {
	out := map[AreaSlug]struct{}{}
	for _, d := range decls {
		if d == "*" {
			for slug := range tree.Children {
				walkDescendants(tree, slug, out)
			}
			continue
		}
		slug := AreaSlug(d)
		if _, ok := tree.Children[slug]; !ok {
			continue
		}
		walkDescendants(tree, slug, out)
	}
	return out
}

func walkDescendants(tree AreaTree, root AreaSlug, out map[AreaSlug]struct{}) {
	if _, seen := out[root]; seen {
		return
	}
	out[root] = struct{}{}
	for _, child := range tree.Children[root] {
		walkDescendants(tree, child, out)
	}
}
