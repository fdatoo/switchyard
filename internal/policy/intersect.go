package policy

import "strings"

// CompiledTokenScope is the request-time form of a token's scope.
type CompiledTokenScope struct {
	AllowedActions  map[ActionKey]struct{} // empty with no patterns ⇒ no action narrowing
	AllowedTools    []string               // trailing-* patterns are allowed
	AllowedServices []string               // trailing-* patterns are allowed
	AllowedTargets  CompiledSelector       // zero ⇒ no target narrowing
}

func (s CompiledTokenScope) PermitsAction(verb Verb, key ActionKey) bool {
	_ = verb // verbs are folded into the action key by the issuer
	return s.actionAllowed(string(key))
}

func (s CompiledTokenScope) PermitsTarget(t Target) bool {
	return s.targetAllowed(t)
}

// Allow returns whether the token scope permits an action/target pair.
func (s CompiledTokenScope) Allow(action string, target Target) bool {
	return s.actionAllowed(action) && s.targetAllowed(target)
}

// Contains returns whether every permission granted by child is also granted by s.
func (s CompiledTokenScope) Contains(child CompiledTokenScope) bool {
	return s.containsActions(child) && s.containsTargets(child)
}

func (s CompiledTokenScope) actionAllowed(action string) bool {
	if len(s.AllowedActions) == 0 && len(s.AllowedTools) == 0 && len(s.AllowedServices) == 0 {
		return true
	}
	if _, ok := s.AllowedActions[ActionKey(action)]; ok {
		return true
	}
	for _, pattern := range s.AllowedTools {
		if tokenPatternMatches(pattern, action) {
			return true
		}
	}
	for _, pattern := range s.AllowedServices {
		if tokenPatternMatches(pattern, action) {
			return true
		}
	}
	return false
}

func (s CompiledTokenScope) targetAllowed(t Target) bool {
	if s.AllowedTargets.MatchAny {
		return true
	}
	if len(s.AllowedTargets.AreaSet) == 0 &&
		len(s.AllowedTargets.ClassSet) == 0 &&
		len(s.AllowedTargets.EntitySet) == 0 {
		return true
	}
	return SelectorMatches(s.AllowedTargets, t)
}

func (s CompiledTokenScope) containsActions(child CompiledTokenScope) bool {
	if !s.hasActionNarrowing() {
		return true
	}
	if !child.hasActionNarrowing() {
		return false
	}
	for action := range child.AllowedActions {
		if !s.actionAllowed(string(action)) {
			return false
		}
	}
	for _, pattern := range child.AllowedTools {
		if !s.coversPattern(pattern) {
			return false
		}
	}
	for _, pattern := range child.AllowedServices {
		if !s.coversPattern(pattern) {
			return false
		}
	}
	return true
}

func (s CompiledTokenScope) hasActionNarrowing() bool {
	return len(s.AllowedActions) > 0 || len(s.AllowedTools) > 0 || len(s.AllowedServices) > 0
}

func (s CompiledTokenScope) coversPattern(child string) bool {
	for parent := range s.AllowedActions {
		if tokenPatternCovers(string(parent), child) {
			return true
		}
	}
	for _, parent := range s.AllowedTools {
		if tokenPatternCovers(parent, child) {
			return true
		}
	}
	for _, parent := range s.AllowedServices {
		if tokenPatternCovers(parent, child) {
			return true
		}
	}
	return false
}

func (s CompiledTokenScope) containsTargets(child CompiledTokenScope) bool {
	parent := s.AllowedTargets
	target := child.AllowedTargets
	if parent.MatchAny {
		return true
	}
	if selectorEmpty(parent) {
		return true
	}
	if selectorEmpty(target) || target.MatchAny {
		return false
	}
	return setContainsAll(parent.AreaSet, target.AreaSet) &&
		setContainsAll(parent.ClassSet, target.ClassSet) &&
		setContainsAll(parent.EntitySet, target.EntitySet)
}

func selectorEmpty(sel CompiledSelector) bool {
	return !sel.MatchAny && len(sel.AreaSet) == 0 && len(sel.ClassSet) == 0 && len(sel.EntitySet) == 0
}

func setContainsAll[K comparable](parent, child map[K]struct{}) bool {
	for key := range child {
		if _, ok := parent[key]; !ok {
			return false
		}
	}
	return true
}

func tokenPatternMatches(pattern, action string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(action, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == action
}

func tokenPatternCovers(parent, child string) bool {
	parentWildcard := strings.HasSuffix(parent, "*")
	childWildcard := strings.HasSuffix(child, "*")
	if !parentWildcard {
		return !childWildcard && parent == child
	}
	parentPrefix := strings.TrimSuffix(parent, "*")
	if !childWildcard {
		return strings.HasPrefix(child, parentPrefix)
	}
	return strings.HasPrefix(strings.TrimSuffix(child, "*"), parentPrefix)
}
