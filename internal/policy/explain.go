package policy

import (
	"context"
	"fmt"

	"github.com/fdatoo/switchyard/internal/auth"
)

type Trace struct {
	Decision string   // "ALLOWED" | "DENIED"
	Reason   string   // matches ErrForbidden.Reason
	RuleName string   // populated for explicit_deny
	Steps    []string // human-readable trace lines
}

func Explain(ctx context.Context, r *Runtime, p auth.Principal, a auth.Action, t auth.Target) Trace {
	tr := Trace{}
	if p.Kind == "system" {
		tr.Decision = "ALLOWED"
		tr.Reason = "system_bypass"
		tr.Steps = append(tr.Steps, "principal kind = system → bypass")
		return tr
	}
	c := r.compiled.Load()
	roles := r.roles.For(p)
	rolesList := sortedRoles(roles)
	tr.Steps = append(tr.Steps, fmt.Sprintf("principal %s → roles %v", p.ID, rolesList))
	actionKey := MakeActionKey(a.Service, a.Method)
	verb := Verb(a.Verb)

	permitted := false
	for _, role := range rolesList {
		if c.permitsAction(role, verb, actionKey) {
			tr.Steps = append(tr.Steps, fmt.Sprintf("action_allowlist[(%s, %s)] permits %s ✓", role, verb, actionKey))
			permitted = true
			break
		}
	}
	if !permitted {
		tr.Decision = "DENIED"
		tr.Reason = "action_denied"
		tr.Steps = append(tr.Steps, fmt.Sprintf("no role permits %s/%s", verb, actionKey))
		return tr
	}

	target := targetFromAuth(t)
	if t.Kind == "entity" {
		allowed := false
		for _, role := range rolesList {
			for i, rule := range c.AllowRules[role] {
				if ruleMatches(rule, a, target) {
					tr.Steps = append(tr.Steps, fmt.Sprintf(
						"allow_rules[%s]: %s.allow[%d] matches ✓", role, rule.PolicyName, i))
					allowed = true
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			tr.Decision = "DENIED"
			tr.Reason = "target_denied"
			tr.Steps = append(tr.Steps, "no allow rule matches target")
			return tr
		}
		for _, role := range rolesList {
			for i, rule := range c.DenyRules[role] {
				if ruleMatches(rule, a, target) {
					tr.Decision = "DENIED"
					tr.Reason = "explicit_deny"
					tr.RuleName = rule.PolicyName
					tr.Steps = append(tr.Steps, fmt.Sprintf(
						"deny_rules[%s]: %s.deny[%d] matches ✗", role, rule.PolicyName, i))
					return tr
				}
			}
		}
	}
	tr.Decision = "ALLOWED"
	return tr
}

func sortedRoles(roles map[RoleSlug]struct{}) []RoleSlug {
	out := make([]RoleSlug, 0, len(roles))
	for r := range roles {
		out = append(out, r)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
