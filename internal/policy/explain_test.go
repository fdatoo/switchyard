package policy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/policy"
)

func TestExplain_DenyByExplicitRule(t *testing.T) {
	rt := setupRuntime(t)
	trace := policy.Explain(context.Background(), rt,
		auth.Principal{ID: "user:nora", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "lock.nora_room", Attr: map[string]string{"area": "nora_room", "class": "Lock"}})
	require.Equal(t, "DENIED", trace.Decision)
	require.Equal(t, "explicit_deny", trace.Reason)
	require.Equal(t, "kids_bedrooms_only", trace.RuleName)
	require.NotEmpty(t, trace.Steps)
}

func TestExplain_AllowOnAllowedTarget(t *testing.T) {
	rt := setupRuntime(t)
	trace := policy.Explain(context.Background(), rt,
		auth.Principal{ID: "user:nora", Kind: "user"},
		auth.Action{Service: "EntityService", Method: "CallCapability", Verb: "call"},
		auth.Target{Kind: "entity", ID: "light.nora_room_lamp", Attr: map[string]string{"area": "nora_room", "class": "Light"}})
	require.Equal(t, "ALLOWED", trace.Decision)
}
