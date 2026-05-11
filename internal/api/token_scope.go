package api

import (
	"errors"
	"fmt"
	"strings"

	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/policy"
)

var errTokenScopeBroader = errors.New("token scope broader than issuer")

func compileTokenScopePB(scope *authpb.TokenScope) (policy.CompiledTokenScope, error) {
	if err := validateTokenScopePB(scope); err != nil {
		return policy.CompiledTokenScope{}, err
	}
	if scope == nil {
		return policy.CompiledTokenScope{}, nil
	}
	return policy.CompiledTokenScope{
		AllowedTools:    append([]string(nil), scope.GetAllowTools()...),
		AllowedServices: append([]string(nil), scope.GetAllowServices()...),
		AllowedTargets:  compileTokenTargetSelector(scope.GetAllowTargets()),
	}, nil
}

func validateTokenScopePB(scope *authpb.TokenScope) error {
	if scope == nil {
		return nil
	}
	for _, pattern := range scope.GetAllowTools() {
		if err := validateTokenScopePattern(pattern); err != nil {
			return err
		}
	}
	for _, pattern := range scope.GetAllowServices() {
		if err := validateTokenScopePattern(pattern); err != nil {
			return err
		}
	}
	for _, item := range scope.GetAllowTargets().GetAreas() {
		if item == "" {
			return fmt.Errorf("%w: empty area selector", ErrValidationFailed)
		}
	}
	for _, item := range scope.GetAllowTargets().GetClasses() {
		if item == "" {
			return fmt.Errorf("%w: empty class selector", ErrValidationFailed)
		}
	}
	for _, item := range scope.GetAllowTargets().GetEntityIds() {
		if item == "" {
			return fmt.Errorf("%w: empty entity selector", ErrValidationFailed)
		}
	}
	return nil
}

func validateTokenScopePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("%w: empty token scope pattern", ErrValidationFailed)
	}
	wildcards := strings.Count(pattern, "*")
	if wildcards == 0 {
		return nil
	}
	if wildcards == 1 && strings.HasSuffix(pattern, "*") {
		return nil
	}
	return fmt.Errorf("%w: token scope wildcards must be trailing-only", ErrValidationFailed)
}

func compileTokenTargetSelector(sel *authpb.TokenTargetSelector) policy.CompiledSelector {
	if sel == nil {
		return policy.CompiledSelector{}
	}
	return policy.CompiledSelector{
		AreaSet:   areaSet(sel.GetAreas()),
		ClassSet:  stringSet(sel.GetClasses()),
		EntitySet: stringSet(sel.GetEntityIds()),
	}
}

func areaSet(items []string) map[policy.AreaSlug]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[policy.AreaSlug]struct{}, len(items))
	for _, item := range items {
		out[policy.AreaSlug(item)] = struct{}{}
	}
	return out
}

func stringSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}

func emptyTokenScopePB(scope *authpb.TokenScope) bool {
	return scope == nil ||
		(len(scope.GetAllowTools()) == 0 &&
			len(scope.GetAllowServices()) == 0 &&
			len(scope.GetAllowTargets().GetAreas()) == 0 &&
			len(scope.GetAllowTargets().GetClasses()) == 0 &&
			len(scope.GetAllowTargets().GetEntityIds()) == 0)
}

func tokenScopeSummary(scope *authpb.TokenScope) string {
	if emptyTokenScopePB(scope) {
		return "full"
	}
	parts := make([]string, 0, 5)
	if n := len(scope.GetAllowTools()); n > 0 {
		parts = append(parts, fmt.Sprintf("tools=%d", n))
	}
	if n := len(scope.GetAllowServices()); n > 0 {
		parts = append(parts, fmt.Sprintf("services=%d", n))
	}
	if n := len(scope.GetAllowTargets().GetAreas()); n > 0 {
		parts = append(parts, fmt.Sprintf("areas=%d", n))
	}
	if n := len(scope.GetAllowTargets().GetClasses()); n > 0 {
		parts = append(parts, fmt.Sprintf("classes=%d", n))
	}
	if n := len(scope.GetAllowTargets().GetEntityIds()); n > 0 {
		parts = append(parts, fmt.Sprintf("entities=%d", n))
	}
	return strings.Join(parts, ",")
}
