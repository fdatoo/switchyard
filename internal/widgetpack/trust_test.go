package widgetpack

import (
	"context"
	"errors"
	"testing"
)

// Tests for the cosign-keyless verifier built on sigstore-go.
//
// "No signature" cases exercise Verify directly. "With signature" cases use
// verifyEntity, the package-internal hook that takes a verify.SignedEntity —
// this lets us feed sigstore-go's VirtualSigstore.Sign output straight into
// the verifier without serialising to JSON. The end-to-end JSON path is
// covered by the OCI integration test in Task 15.

const testIssuer = "https://accounts.example.com"

func TestVerify_AllowedSignerGlob(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	payload := []byte("hello widgetpack")
	entity := root.signBlobEntity(t, payload, "https://github.com/myhandle/foo", testIssuer)

	pol := &TrustPolicy{}
	if err := pol.Set([]string{"https://github.com/myhandle/*"}, false); err != nil {
		t.Fatal(err)
	}

	res, err := v.verifyEntity(context.Background(), payload, entity, pol)
	if err != nil {
		t.Fatalf("verifyEntity: %v", err)
	}
	if res.Status != "verified" {
		t.Errorf("status = %q, want %q", res.Status, "verified")
	}
	if res.SignerIdentity != "https://github.com/myhandle/foo" {
		t.Errorf("signer = %q, want %q", res.SignerIdentity, "https://github.com/myhandle/foo")
	}
}

func TestVerify_SignerGlob_NoMatch_Rejected(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	payload := []byte("hello widgetpack")
	entity := root.signBlobEntity(t, payload, "https://github.com/randomattacker/foo", testIssuer)

	pol := &TrustPolicy{}
	if err := pol.Set([]string{"https://github.com/myhandle/*"}, false); err != nil {
		t.Fatal(err)
	}

	res, err := v.verifyEntity(context.Background(), payload, entity, pol)
	if err == nil {
		t.Fatalf("expected rejection, got result %+v", res)
	}
	if !errors.Is(err, ErrSignatureRejected) {
		t.Errorf("error = %v, want wraps ErrSignatureRejected", err)
	}
}

func TestVerify_NoSignature_AllowUnsigned(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	pol := &TrustPolicy{}
	if err := pol.Set(nil, true); err != nil {
		t.Fatal(err)
	}

	res, err := v.Verify(context.Background(), []byte("payload"), nil, pol)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.Status != "unsigned" {
		t.Errorf("status = %q, want %q", res.Status, "unsigned")
	}
	if res.SignerIdentity != "" {
		t.Errorf("signer = %q, want empty", res.SignerIdentity)
	}
}

func TestVerify_NoSignature_DenyUnsigned(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	pol := &TrustPolicy{}
	if err := pol.Set(nil, false); err != nil {
		t.Fatal(err)
	}

	res, err := v.Verify(context.Background(), []byte("payload"), nil, pol)
	if err == nil {
		t.Fatalf("expected rejection, got result %+v", res)
	}
	if !errors.Is(err, ErrUnsignedNotAllowed) {
		t.Errorf("error = %v, want wraps ErrUnsignedNotAllowed", err)
	}
}

func TestVerify_BundleMismatch_Rejected(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	payloadA := []byte("payload A")
	payloadB := []byte("payload B — different bytes")
	entity := root.signBlobEntity(t, payloadA, "https://github.com/myhandle/foo", testIssuer)

	pol := &TrustPolicy{}
	if err := pol.Set([]string{"https://github.com/myhandle/*"}, false); err != nil {
		t.Fatal(err)
	}

	// Verify against a different payload than what was signed.
	res, err := v.verifyEntity(context.Background(), payloadB, entity, pol)
	if err == nil {
		t.Fatalf("expected rejection of payload mismatch, got result %+v", res)
	}
	if !errors.Is(err, ErrSignatureRejected) {
		t.Errorf("error = %v, want wraps ErrSignatureRejected", err)
	}
}

// TestVerify_GarbageBundle_Rejected exercises the JSON path — the production
// hot path — by feeding random bytes as a bundle.
func TestVerify_GarbageBundle_Rejected(t *testing.T) {
	t.Parallel()
	root := newTestTrustRoot(t)
	v, err := NewVerifier(root.vs)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	pol := &TrustPolicy{}
	if err := pol.Set([]string{"https://github.com/myhandle/*"}, false); err != nil {
		t.Fatal(err)
	}

	_, err = v.Verify(context.Background(), []byte("payload"), []byte("not a bundle"), pol)
	if err == nil {
		t.Fatalf("expected rejection of garbage bundle")
	}
	if !errors.Is(err, ErrSignatureRejected) {
		t.Errorf("error = %v, want wraps ErrSignatureRejected", err)
	}
}

func TestTrustPolicy_Set_RejectsBadPattern(t *testing.T) {
	pol := &TrustPolicy{}
	if err := pol.Set([]string{"https://github.com/[unclosed"}, false); err == nil {
		t.Error("expected error for malformed glob pattern")
	}
}
