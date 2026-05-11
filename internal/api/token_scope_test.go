package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/policy"
)

func TestDecodeTokenScopeReturnsMatcher(t *testing.T) {
	blob, err := proto.Marshal(&authpb.TokenScope{
		AllowTools:    []string{"gohome__*"},
		AllowServices: []string{"EntityService.*"},
		AllowTargets: &authpb.TokenTargetSelector{
			Areas: []string{"kitchen"},
		},
	})
	require.NoError(t, err)

	scope, err := decodeTokenScope(context.Background(), "tok_decode_matcher", blob)
	require.NoError(t, err)
	require.True(t, scope.Allow("gohome__turn_on", policy.Target{Area: "kitchen"}))
	require.True(t, scope.Allow("EntityService.Get", policy.Target{Area: "kitchen"}))
	require.False(t, scope.Allow("EntityService.Get", policy.Target{Area: "garage"}))
	require.False(t, scope.Allow("ConfigService.Apply", policy.Target{Area: "kitchen"}))
}

func TestCompileTokenScopeRejectsEmbeddedWildcard(t *testing.T) {
	_, err := compileTokenScopePB(&authpb.TokenScope{
		AllowServices: []string{"Entity*.Get"},
	})
	require.ErrorIs(t, err, ErrValidationFailed)
}
