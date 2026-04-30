package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonpb "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/policy"
)

func TestEntityFilter_FilterMode_Narrows(t *testing.T) {
	// empty policy → all denied for user principal
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	f := api.NewEntityFilter(rt, commonpb.PolicyMode_POLICY_MODE_FILTER)
	p := auth.Principal{ID: "user:alice", Kind: "user"}
	candidates := []policy.Target{{Kind: "entity", ID: "light.living_room"}}

	allowed, err := f.Preflight(context.Background(), p, candidates)
	require.NoError(t, err)  // filter mode: no error
	assert.Empty(t, allowed) // all denied by empty policy
}

func TestEntityFilter_StrictMode_PermissionDenied(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	f := api.NewEntityFilter(rt, commonpb.PolicyMode_POLICY_MODE_STRICT)
	p := auth.Principal{ID: "user:alice", Kind: "user"}
	candidates := []policy.Target{{Kind: "entity", ID: "light.living_room"}}

	_, err := f.Preflight(context.Background(), p, candidates)
	require.Error(t, err)
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, connect.CodePermissionDenied, ce.Code())
}

func TestEntityFilter_NilRuntime_AllowsAll(t *testing.T) {
	f := api.NewEntityFilter(nil, commonpb.PolicyMode_POLICY_MODE_FILTER)
	p := auth.Principal{ID: "user:alice", Kind: "user"}
	candidates := []policy.Target{
		{Kind: "entity", ID: "light.living_room"},
		{Kind: "entity", ID: "switch.kitchen"},
	}

	allowed, err := f.Preflight(context.Background(), p, candidates)
	require.NoError(t, err)
	assert.Equal(t, candidates, allowed)
}

func TestEntityFilter_NilRuntime_AllowsEntity(t *testing.T) {
	f := api.NewEntityFilter(nil, commonpb.PolicyMode_POLICY_MODE_FILTER)
	p := auth.Principal{ID: "user:alice", Kind: "user"}
	target := policy.Target{Kind: "entity", ID: "light.living_room"}
	assert.True(t, f.AllowsEntity(context.Background(), p, target))
}
