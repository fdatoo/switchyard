// Test trust-root infrastructure shared between trust_test.go and the
// upcoming install_integration_test.go (Task 15).
//
// We lean on sigstore-go's pkg/testing/ca.VirtualSigstore — a battle-tested
// in-memory CA + Rekor + TSA bundle. VirtualSigstore implements
// root.TrustedMaterial directly, so it plugs straight into NewVerifier.
//
// VirtualSigstore.Sign returns a *ca.TestEntity, which itself implements
// verify.SignedEntity. The unit tests in trust_test.go feed this entity to
// the package-internal verifyEntity hook, exercising the full sigstore-go
// verification pipeline (cert chain, transparency log inclusion, observer
// timestamps) without going through JSON.
//
// The integration test in Task 15 will instead pull a cosign-style bundle JSON
// from an in-process OCI registry; that test will need a separate helper
// (signBlobBundleJSON) that serialises a TestEntity to bundle bytes. Building
// that helper requires populating fields that VirtualSigstore.generateTlogEntry
// leaves nil (KindVersion, InclusionPromise) — see Task 15.

package widgetpack

import (
	"testing"

	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// testTrustRoot wraps a VirtualSigstore so tests can sign blobs with arbitrary
// SAN URIs and feed the resulting SignedEntity to the verifier.
type testTrustRoot struct {
	vs *ca.VirtualSigstore
}

// newTestTrustRoot stands up a fresh in-memory Sigstore for one test. The
// VirtualSigstore implements root.TrustedMaterial, so it can be passed
// straight to widgetpack.NewVerifier.
func newTestTrustRoot(t *testing.T) *testTrustRoot {
	t.Helper()
	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		t.Fatalf("NewVirtualSigstore: %v", err)
	}
	return &testTrustRoot{vs: vs}
}

// signBlobEntity signs payload with a fresh leaf cert whose SAN URI is
// identityURI and returns the resulting verify.SignedEntity. The entity wraps
// the leaf cert, message signature, transparency log entry, and signed
// timestamp — i.e., everything the sigstore verifier needs.
func (r *testTrustRoot) signBlobEntity(t *testing.T, payload []byte, identityURI, issuer string) verify.SignedEntity {
	t.Helper()
	entity, err := r.vs.Sign(identityURI, issuer, payload)
	if err != nil {
		t.Fatalf("VirtualSigstore.Sign: %v", err)
	}
	return entity
}
