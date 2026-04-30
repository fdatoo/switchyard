package policy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/policy"
)

func setupRuntime(t *testing.T) *policy.Runtime {
	t.Helper()
	rules := []policy.RawPolicy{
		{Name: "admin_full", Subjects: []policy.RoleSlug{"admin"},
			Allow: []policy.RawRule{{Targets: policy.RawSelector{MatchAny: true}}}},
		{Name: "kids_bedrooms_only", Subjects: []policy.RoleSlug{"kids"},
			Allow: []policy.RawRule{{
				Verbs:   []policy.Verb{policy.VerbRead, policy.VerbCall},
				Targets: policy.RawSelector{Areas: []string{"kids_floor"}},
			}},
			Deny: []policy.RawRule{{
				Verbs:   []policy.Verb{policy.VerbCall},
				Targets: policy.RawSelector{Classes: []string{"Lock"}},
			}}},
	}
	c, err := policy.Compile(rules, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.NoError(t, err)
	rt := policy.NewRuntime(staticRoles{
		"user:fdatoo": {"admin"},
		"user:nora":   {"kids"},
	})
	rt.Replace(c)
	return rt
}

type staticRoles map[string][]policy.RoleSlug

func (s staticRoles) For(p auth.Principal) map[policy.RoleSlug]struct{} {
	out := map[policy.RoleSlug]struct{}{}
	for _, r := range s[p.ID] {
		out[r] = struct{}{}
	}
	return out
}

func TestAuthorize_SystemPrincipalBypass(t *testing.T) {
	rt := setupRuntime(t)
	err := rt.Authorize(context.Background(),
		auth.Principal{ID: "system:local", Kind: "system"},
		auth.Action{Service: "ConfigService", Method: "Apply", Verb: "admin"},
		auth.Target{})
	require.NoError(t, err)
}

func TestAuthorize_AdminAllowedEverything(t *testing.T) {
	rt := setupRuntime(t)
	err := rt.Authorize(context.Background(),
		auth.Principal{ID: "user:fdatoo", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "lock.front_door", Area: "front_entry", Class: "Lock"})
	require.NoError(t, err)
}

func TestAuthorize_KidDeniedFrontDoorLock(t *testing.T) {
	rt := setupRuntime(t)
	err := rt.Authorize(context.Background(),
		auth.Principal{ID: "user:nora", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "lock.front_door", Area: "front_entry", Class: "Lock"})
	require.Error(t, err)
	var fb *policy.ErrForbidden
	require.ErrorAs(t, err, &fb)
	require.Equal(t, "target_denied", fb.Reason)
}

func TestAuthorize_KidDeniedKidsFloorLock(t *testing.T) {
	rt := setupRuntime(t)
	err := rt.Authorize(context.Background(),
		auth.Principal{ID: "user:nora", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "lock.nora_room", Area: "nora_room", Class: "Lock"})
	require.Error(t, err)
	var fb *policy.ErrForbidden
	require.ErrorAs(t, err, &fb)
	require.Equal(t, "explicit_deny", fb.Reason)
	require.Equal(t, "kids_bedrooms_only", fb.RuleName)
}

func TestAuthorize_KidAllowedKidsFloorLight(t *testing.T) {
	rt := setupRuntime(t)
	err := rt.Authorize(context.Background(),
		auth.Principal{ID: "user:nora", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "light.nora_room_ceiling", Area: "nora_room", Class: "Light"})
	require.NoError(t, err)
}

func TestFilterEntities_SplitsAllowedAndDenied(t *testing.T) {
	rt := setupRuntime(t)
	candidates := []policy.Target{
		{Kind: "entity", ID: "light.kitchen", Area: "kitchen", Class: "Light"},
		{Kind: "entity", ID: "light.nora_room_lamp", Area: "nora_room", Class: "Light"},
		{Kind: "entity", ID: "lock.nora_room", Area: "nora_room", Class: "Lock"},
	}
	allowed, denied := rt.FilterEntities(context.Background(),
		auth.Principal{ID: "user:nora", Kind: "user"}, "read", candidates)
	require.Len(t, allowed, 1)
	require.Equal(t, "light.nora_room_lamp", allowed[0].ID)
	require.Len(t, denied, 2)
}

func TestRuntime_OnReload_FiresOnReplace(t *testing.T) {
	rt := setupRuntime(t)
	fired := make(chan struct{}, 1)
	rt.OnReload(func() { fired <- struct{}{} })
	rt.Replace(&policy.Compiled{Generation: 999})
	select {
	case <-fired:
	default:
		t.Fatal("OnReload subscriber did not fire")
	}
}
