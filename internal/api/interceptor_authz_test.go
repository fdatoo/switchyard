package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gohomev1alpha1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/policy"
)

// testProcedureCatalog implements api.ProcedureCatalog for tests.
type testProcedureCatalog struct {
	action auth.Action
	target auth.Target
	found  bool
}

func (f *testProcedureCatalog) Resolve(_ string, _ any) (auth.Action, auth.Target, bool) {
	return f.action, f.target, f.found
}

// testUnaryFunc is a connect.UnaryFunc that succeeds unconditionally.
func testUnaryFunc(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
	return connect.NewResponse(&gohomev1alpha1.CurrentUserResponse{}), nil
}

// testConnectRequest returns a minimal connect.AnyRequest for tests.
func testConnectRequest() connect.AnyRequest {
	return connect.NewRequest(&gohomev1alpha1.CurrentUserRequest{})
}

// testNoopRoles implements policy.Roles: every principal has no roles.
type testNoopRoles struct{}

func (testNoopRoles) For(_ auth.Principal) map[policy.RoleSlug]struct{} { return nil }

// testEmptyCompiled returns a compiled artifact with no policies.
func testEmptyCompiled() *policy.Compiled {
	c, _ := policy.Compile(nil, policy.RoleGraph{Roles: map[policy.RoleSlug]policy.RoleNode{}}, policy.AreaTree{}, policy.ActionCatalog{})
	return c
}

func TestNewAuthorize_NilRuntime_PassesThrough(t *testing.T) {
	interceptor := api.NewAuthorize(nil, nil, nil, nil)
	fn := interceptor(testUnaryFunc)
	resp, err := fn(context.Background(), testConnectRequest())
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewAuthorize_NilCatalog_PassesThrough(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())
	interceptor := api.NewAuthorize(rt, nil, nil, nil)
	fn := interceptor(testUnaryFunc)
	resp, err := fn(context.Background(), testConnectRequest())
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewAuthorize_ProcedureNotInCatalog_PassesThrough(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	interceptor := api.NewAuthorize(rt, &testProcedureCatalog{found: false}, nil, nil)
	fn := interceptor(testUnaryFunc)
	resp, err := fn(auth.WithPrincipal(context.Background(), auth.Principal{Kind: "system"}), testConnectRequest())
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewAuthorize_SystemPrincipal_Allowed(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	cat := &testProcedureCatalog{
		action: auth.Action{Service: "TestService", Method: "Foo", Verb: "read"},
		found:  true,
	}
	interceptor := api.NewAuthorize(rt, cat, nil, nil)
	fn := interceptor(testUnaryFunc)
	resp, err := fn(auth.WithPrincipal(context.Background(), auth.Principal{Kind: "system"}), testConnectRequest())
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNewAuthorize_UserWithNoPolicy_Denied(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	// empty compiled: no action allowlist → action_denied for any non-system principal
	rt.Replace(testEmptyCompiled())

	cat := &testProcedureCatalog{
		action: auth.Action{Service: "TestService", Method: "Foo", Verb: "read"},
		found:  true,
	}
	principal := auth.Principal{ID: "user:alice", Kind: "user"}
	interceptor := api.NewAuthorize(rt, cat, nil, nil)
	fn := interceptor(testUnaryFunc)
	_, err := fn(auth.WithPrincipal(context.Background(), principal), testConnectRequest())
	require.Error(t, err)
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, connect.CodePermissionDenied, ce.Code())
}

func TestNewAuthorize_AllowedPath_EmitsMetric(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	cat := &testProcedureCatalog{
		action: auth.Action{Service: "TestService", Method: "Foo", Verb: "read"},
		found:  true,
	}
	m := observability.NewMetrics()
	interceptor := api.NewAuthorize(rt, cat, nil, m)
	fn := interceptor(testUnaryFunc)
	resp, err := fn(auth.WithPrincipal(context.Background(), auth.Principal{Kind: "system"}), testConnectRequest())
	require.NoError(t, err)
	assert.NotNil(t, resp)

	val := testutil.ToFloat64(m.PolicyAuthorizeTotal.WithLabelValues("allowed", ""))
	assert.Equal(t, float64(1), val, "PolicyAuthorizeTotal{result=allowed} should be 1")
}

func TestNewAuthorize_DeniedPath_EmitsMetric(t *testing.T) {
	rt := policy.NewRuntime(testNoopRoles{})
	rt.Replace(testEmptyCompiled())

	cat := &testProcedureCatalog{
		action: auth.Action{Service: "TestService", Method: "Foo", Verb: "read"},
		found:  true,
	}
	m := observability.NewMetrics()
	interceptor := api.NewAuthorize(rt, cat, nil, m)
	fn := interceptor(testUnaryFunc)
	principal := auth.Principal{ID: "user:alice", Kind: "user"}
	_, err := fn(auth.WithPrincipal(context.Background(), principal), testConnectRequest())
	require.Error(t, err)
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, connect.CodePermissionDenied, ce.Code())

	count := testutil.CollectAndCount(m.PolicyAuthorizeTotal)
	assert.Greater(t, count, 0, "PolicyAuthorizeTotal should have at least one observed label set for denied")
}
