// Package widgetpack — trust policy and cosign keyless signature verification.
//
// This file wires the sigstore-go verifier (https://github.com/sigstore/sigstore-go)
// to the daemon's pack install flow. The public surface is small and stable:
//
//   - TrustPolicy   — installer-supplied list of allowed signer identities (with
//     glob support) plus an "allow unsigned" escape hatch.
//   - Verifier      — wraps a sigstore-go *verify.Verifier together with the
//     trust material it was built from. Construct via NewVerifier (test-injectable
//     trust root) or NewProductionVerifier (production TUF root, currently stubbed).
//   - VerificationResult — the outcome we hand back to the caller. Status is one
//     of "verified" or "unsigned"; on failure we return an error rather than
//     populate the result, so InstalledPack.SignatureStatus tracks Status verbatim.
package widgetpack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// ErrSignatureRejected is returned when verification fails for any reason —
// invalid signature, untrusted signer, identity not on the allowlist, etc.
var ErrSignatureRejected = errors.New("widgetpack: signature rejected")

// ErrUnsignedNotAllowed is returned when no signature was supplied and the
// trust policy disallows unsigned packs.
var ErrUnsignedNotAllowed = errors.New("widgetpack: unsigned pack not allowed")

// TrustPolicy describes which pack signers the daemon trusts.
//
// AllowedSigners holds glob patterns matched against the cert's SAN URI
// (e.g. "https://github.com/myhandle/*"). Matching uses path.Match semantics.
// AllowUnsigned controls behaviour when the OCI artifact carries no signature.
type TrustPolicy struct {
	mu             sync.RWMutex
	allowedSigners []string
	allowUnsigned  bool
}

// Set replaces the policy fields atomically. Safe for concurrent use with Verify.
// It returns an error if any signer pattern is not a valid path.Match glob.
func (p *TrustPolicy) Set(signers []string, allowUnsigned bool) error {
	for _, pat := range signers {
		if _, err := path.Match(pat, ""); err != nil {
			return fmt.Errorf("widgetpack: invalid signer pattern %q: %w", pat, err)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if signers == nil {
		p.allowedSigners = nil
	} else {
		p.allowedSigners = append(p.allowedSigners[:0], signers...)
	}
	p.allowUnsigned = allowUnsigned
	return nil
}

// AllowUnsigned reports whether the policy permits unsigned packs.
func (p *TrustPolicy) AllowUnsigned() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.allowUnsigned
}

// matchesAllowedSigner returns true when the given subject matches any of the
// allowed-signer globs. An empty allow-list means "no signers permitted".
func (p *TrustPolicy) matchesAllowedSigner(subject string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, pat := range p.allowedSigners {
		if ok, err := path.Match(pat, subject); err == nil && ok {
			return true
		}
	}
	return false
}

// VerificationResult is the outcome of a successful Verify call. Failures are
// surfaced as errors, not as a populated result with a "rejected" status.
type VerificationResult struct {
	// Status is "verified" or "unsigned". Maps directly to
	// InstalledPack.SignatureStatus.
	Status string

	// SignerIdentity is the cert SAN URI when Status == "verified". Empty when
	// Status == "unsigned".
	SignerIdentity string
}

// Verifier checks cosign-style sigstore bundles against an injected trust root.
//
// Construct with NewVerifier (tests) or NewProductionVerifier (production TUF
// root). The struct is safe for concurrent use; the underlying sigstore-go
// verifier is read-only after construction.
type Verifier struct {
	sev *verify.Verifier
}

// NewVerifier wraps a caller-supplied TrustedMaterial. Tests inject an
// in-memory VirtualSigstore via this entry point; production code uses
// NewProductionVerifier instead.
//
// The verifier requires both a transparency log entry and an observer
// timestamp on every bundle (the sigstore-go default for cosign keyless).
func NewVerifier(tm root.TrustedMaterial) (*Verifier, error) {
	if tm == nil {
		return nil, errors.New("widgetpack: NewVerifier: tm must not be nil")
	}
	sev, err := verify.NewVerifier(tm,
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: build sigstore verifier: %w", err)
	}
	return &Verifier{sev: sev}, nil
}

// NewProductionVerifier is the production entry point. It is intentionally
// stubbed for now — wiring sigstore-go's TUF client into the daemon is tracked
// separately. Until that lands, the daemon should construct a Verifier via
// NewVerifier with a trust root loaded out-of-band, or refuse to start when
// AllowUnsigned == false.
//
// See https://pkg.go.dev/github.com/sigstore/sigstore-go/pkg/tuf for the TUF
// client surface that will back this constructor.
func NewProductionVerifier(_ context.Context) (*Verifier, error) {
	return nil, errors.New("widgetpack: production verifier not yet wired; use NewVerifier with an injected trust root")
}

// Verify checks that signatureBundle is a valid cosign keyless signature over
// payload (or, if signatureBundle is nil, applies the unsigned policy).
//
// pol must be non-nil. A signed bundle whose signer identity is not matched
// by pol.allowedSigners is rejected; an unsigned payload is rejected unless
// pol.AllowUnsigned() is true.
//
//   - payload: the bytes the bundle was signed over (the OCI artifact bytes).
//   - signatureBundle: the sigstore JSON bundle blob (cosign pushes this to
//     <ref>.sig). nil indicates no signature is present.
//
// Returns:
//   - {Status: "unsigned"} when signatureBundle == nil and pol.AllowUnsigned.
//   - {Status: "verified", SignerIdentity: <SAN URI>} when the bundle verifies
//     and the cert subject matches an allowed glob.
//   - ErrUnsignedNotAllowed / ErrSignatureRejected (wrapped) on failure.
//
// ctx is reserved for future TUF refresh in NewProductionVerifier; not currently plumbed.
func (v *Verifier) Verify(
	ctx context.Context,
	payload []byte,
	signatureBundle []byte,
	pol *TrustPolicy,
) (*VerificationResult, error) {
	if len(signatureBundle) == 0 {
		if pol != nil && pol.AllowUnsigned() {
			return &VerificationResult{Status: "unsigned"}, nil
		}
		return nil, ErrUnsignedNotAllowed
	}

	b := &bundle.Bundle{}
	if err := b.UnmarshalJSON(signatureBundle); err != nil {
		return nil, fmt.Errorf("%w: parse bundle: %v", ErrSignatureRejected, err)
	}

	return v.verifyEntity(ctx, payload, b, pol)
}

// verifyEntity is the interior of Verify, factored so internal callers can pass
// a sigstore SignedEntity directly without going through JSON. Tests in this
// package use this to feed a TestEntity from VirtualSigstore.Sign.
func (v *Verifier) verifyEntity(
	_ context.Context,
	payload []byte,
	entity verify.SignedEntity,
	pol *TrustPolicy,
) (*VerificationResult, error) {
	// Verify cryptographic integrity, transparency log inclusion, and timestamp.
	// Identity matching is done separately so we can apply our own glob policy
	// against the SAN URI, rather than the strict equality / single-regex check
	// sigstore-go's CertificateIdentity provides.
	res, err := v.sev.Verify(entity, verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(payload)),
		verify.WithoutIdentitiesUnsafe(),
	))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSignatureRejected, err)
	}

	// Pull the cert SAN URI for the signer.
	subject := ""
	if res.Signature != nil {
		subject = res.Signature.Certificate.SubjectAlternativeName
	}
	if subject == "" {
		return nil, fmt.Errorf("%w: signer identity missing from cert", ErrSignatureRejected)
	}

	if pol == nil || !pol.matchesAllowedSigner(subject) {
		return nil, fmt.Errorf("%w: signer %q not on allowed list", ErrSignatureRejected, subject)
	}

	return &VerificationResult{
		Status:         "verified",
		SignerIdentity: subject,
	}, nil
}
