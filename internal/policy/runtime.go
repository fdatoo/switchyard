package policy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/fdatoo/switchyard/internal/auth"
)

// Roles is the abstraction the runtime uses to translate Principal → role set.
type Roles interface {
	For(auth.Principal) map[RoleSlug]struct{}
}

type Runtime struct {
	roles    Roles
	compiled atomic.Pointer[Compiled]

	subsMu sync.Mutex
	subs   []func()
}

func NewRuntime(roles Roles) *Runtime {
	return &Runtime{roles: roles}
}

// Replace atomically swaps the compiled artifact and notifies subscribers.
func (r *Runtime) Replace(c *Compiled) {
	r.compiled.Store(c)
	r.subsMu.Lock()
	subs := append([]func(){}, r.subs...)
	r.subsMu.Unlock()
	for _, fn := range subs {
		fn()
	}
}

// OnReload registers a callback fired after every successful Replace.
func (r *Runtime) OnReload(fn func()) {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()
	r.subs = append(r.subs, fn)
}

// CurrentGeneration returns the generation of the active artifact, or 0 if none installed.
func (r *Runtime) CurrentGeneration() uint64 {
	c := r.compiled.Load()
	if c == nil {
		return 0
	}
	return c.Generation
}

// ErrForbidden is the rich denial type.
type ErrForbidden struct {
	Reason   string // "action_denied" | "target_denied" | "explicit_deny" | "token_action_denied" | "token_target_denied" | "no_policy"
	RuleName string
}

func (e *ErrForbidden) Error() string {
	if e.RuleName != "" {
		return fmt.Sprintf("policy: %s (rule %q)", e.Reason, e.RuleName)
	}
	return fmt.Sprintf("policy: %s", e.Reason)
}

// Authorize is the hot-path call. Returns nil for permit, *ErrForbidden for deny.
func (r *Runtime) Authorize(ctx context.Context, p auth.Principal, a auth.Action, t auth.Target) error {
	if p.Kind == "system" {
		return nil
	}
	c := r.compiled.Load()
	if c == nil {
		return &ErrForbidden{Reason: "no_policy"}
	}
	roles := r.roles.For(p)
	actionKey := MakeActionKey(a.Service, a.Method)
	verb := Verb(a.Verb)

	permitted := false
	for role := range roles {
		if c.permitsAction(role, verb, actionKey) {
			permitted = true
			break
		}
	}
	if !permitted {
		return &ErrForbidden{Reason: "action_denied"}
	}

	if scope, ok := tokenScopeFromCtx(ctx); ok {
		if t.Kind == "entity" {
			if !scope.Allow(string(actionKey), targetFromAuth(t)) {
				if !scope.PermitsAction(verb, actionKey) {
					return &ErrForbidden{Reason: "token_action_denied"}
				}
				return &ErrForbidden{Reason: "token_target_denied"}
			}
		} else if !scope.PermitsAction(verb, actionKey) {
			return &ErrForbidden{Reason: "token_action_denied"}
		}
	}

	if t.Kind == "entity" {
		target := targetFromAuth(t)
		allowed := false
		for role := range roles {
			for _, rule := range c.AllowRules[role] {
				if ruleMatches(rule, a, target) {
					allowed = true
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			return &ErrForbidden{Reason: "target_denied"}
		}
		for role := range roles {
			for _, rule := range c.DenyRules[role] {
				if ruleMatches(rule, a, target) {
					return &ErrForbidden{Reason: "explicit_deny", RuleName: rule.PolicyName}
				}
			}
		}
	}
	return nil
}

// FilterEntities partitions candidates into allowed/denied for the given verb.
// An entity is included in allowed only if it passes the allow check for the
// given verb AND is not matched by any deny rule (across all verbs) for the
// principal's roles. This conservative filter avoids showing entities that the
// principal cannot interact with at all.
func (r *Runtime) FilterEntities(ctx context.Context, p auth.Principal, verb string, candidates []Target) (allowed, denied []Target) {
	c := r.compiled.Load()
	roles := r.roles.For(p)
	for _, t := range candidates {
		err := r.Authorize(ctx, p, auth.Action{Service: "EntityService", Method: "Subscribe", Verb: verb}, auth.Target{
			Kind:  "entity",
			ID:    t.ID,
			Area:  string(t.Area),
			Class: t.Class,
		})
		if err != nil {
			denied = append(denied, t)
			continue
		}
		// Also check if the entity is covered by any deny rule regardless of verb.
		// This ensures entities the principal cannot interact with are excluded.
		if c != nil {
			target := Target{Kind: t.Kind, ID: t.ID, Area: t.Area, Class: t.Class}
			explicitlyDenied := false
			for role := range roles {
				for _, rule := range c.DenyRules[role] {
					if SelectorMatches(rule.Targets, target) {
						explicitlyDenied = true
						break
					}
				}
				if explicitlyDenied {
					break
				}
			}
			if explicitlyDenied {
				denied = append(denied, t)
				continue
			}
		}
		allowed = append(allowed, t)
	}
	return allowed, denied
}

func ruleMatches(r CompiledRule, a auth.Action, t Target) bool {
	if len(r.Verbs) > 0 {
		if _, ok := r.Verbs[Verb(a.Verb)]; !ok {
			return false
		}
	}
	if len(r.Services) > 0 {
		if _, ok := r.Services[a.Service]; !ok {
			return false
		}
	}
	return SelectorMatches(r.Targets, t)
}

func targetFromAuth(t auth.Target) Target {
	out := Target{Kind: t.Kind, ID: t.ID, Area: AreaSlug(t.Area), Class: t.Class}
	if out.Area == "" && t.Attr != nil {
		out.Area = AreaSlug(t.Attr["area"])
	}
	if out.Class == "" && t.Attr != nil {
		out.Class = t.Attr["class"]
	}
	return out
}

// --- token-scope context plumbing ---

type ctxKey struct{}

func WithTokenScope(ctx context.Context, scope CompiledTokenScope) context.Context {
	return context.WithValue(ctx, ctxKey{}, scope)
}

func TokenScopeFromContext(ctx context.Context) (CompiledTokenScope, bool) {
	return tokenScopeFromCtx(ctx)
}

func tokenScopeFromCtx(ctx context.Context) (CompiledTokenScope, bool) {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return CompiledTokenScope{}, false
	}
	return v.(CompiledTokenScope), true
}
