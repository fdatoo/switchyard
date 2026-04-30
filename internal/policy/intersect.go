package policy

// CompiledTokenScope is the request-time form of a token's scope.
type CompiledTokenScope struct {
	AllowedActions map[ActionKey]struct{} // empty ⇒ no action narrowing
	AllowedTargets CompiledSelector       // zero ⇒ no target narrowing
}

func (s CompiledTokenScope) PermitsAction(verb Verb, key ActionKey) bool {
	_ = verb // verbs are folded into the action key by the issuer
	if len(s.AllowedActions) == 0 {
		return true
	}
	_, ok := s.AllowedActions[key]
	return ok
}

func (s CompiledTokenScope) PermitsTarget(t Target) bool {
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
