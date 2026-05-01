package widgetpack_test

import (
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestTrustPolicy_AllowUnsigned(t *testing.T) {
	tp := &widgetpack.TrustPolicy{AllowUnsigned: true}
	if !tp.Verify("unsigned", nil) {
		t.Error("expected unsigned to be allowed")
	}
}

func TestTrustPolicy_DenyUnsigned(t *testing.T) {
	tp := &widgetpack.TrustPolicy{AllowUnsigned: false}
	if tp.Verify("unsigned", nil) {
		t.Error("expected unsigned to be denied")
	}
}

func TestTrustPolicy_AllowVerified(t *testing.T) {
	tp := &widgetpack.TrustPolicy{}
	if !tp.Verify("verified", nil) {
		t.Error("expected verified to be allowed")
	}
}

func TestTrustPolicy_DenyInvalid(t *testing.T) {
	tp := &widgetpack.TrustPolicy{AllowUnsigned: true}
	if tp.Verify("invalid", nil) {
		t.Error("expected invalid to be denied")
	}
}
