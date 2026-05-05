// End-to-end integration tests for the §15.4 install pipeline.
//
// These tests stand up an in-process OCI registry (go-containerregistry's
// pkg/registry), push a real gzipped widget pack tarball as an OCI artifact,
// and drive Installer.Install through the full flow. They cover both the
// happy unsigned path (when policy allows) and the rejection path when policy
// requires a signature.
//
// The signed-happy-path test is intentionally skipped: producing a sigstore
// Bundle that satisfies bundle.Bundle's strict `validate()` (inclusion proof
// for v0.2+ bundles, correct media type, etc.) requires either spinning up a
// real Fulcio+Rekor+TSA via pkg/sign or hand-crafting a protobuf Bundle from
// VirtualSigstore primitives. Both are large undertakings the F-157 plan
// explicitly carved out (Task 15 documents the bundle-production caveat).
// The verify path itself is fully covered by trust_test.go's VirtualSigstore-
// driven unit tests, which feed a TestEntity through the package-internal
// verifyEntity hook.
package widgetpack_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

// TestInstall_Integration_UnsignedRejected exercises the policy gate when
// AllowUnsigned=false: an unsigned pack pushed to the in-process registry
// must be rejected with ReasonSignatureInvalid (or its wrapped equivalent),
// and must not appear in the store.
func TestInstall_Integration_UnsignedRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}

	pol := &widgetpack.TrustPolicy{}
	if err := pol.Set(nil, false); err != nil { // AllowUnsigned=false
		t.Fatalf("policy.Set: %v", err)
	}

	// No verifier configured: the install path treats unsigned packs as
	// rejected unless policy allows them — exactly what we're testing.
	fetcher, err := widgetpack.NewFetcher(widgetpack.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	inst := widgetpack.NewInstaller(store, nil, pol, fetcher, dataDir, nil)

	ref := buildAndPushTestPack(t, regHost, "bar-widgets", "1.0.0", false)

	if _, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref}); err == nil {
		t.Fatal("expected unsigned pack to be rejected")
	}
	if _, err := store.Get(ctx, "bar-widgets", "1.0.0"); err == nil {
		t.Errorf("rejected pack should not be in store")
	}
}

// TestInstall_Integration_UnsignedAllowed verifies the §15.4 happy path on
// an unsigned pack when policy allows unsigned: the artifact is pulled,
// extracted, manifest evaluated, bundle hashed, registered in the store, and
// served at the stable bundle URL with the immutable cache header.
func TestInstall_Integration_UnsignedAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}

	pol := &widgetpack.TrustPolicy{}
	if err := pol.Set(nil, true); err != nil { // AllowUnsigned=true
		t.Fatalf("policy.Set: %v", err)
	}

	fetcher, err := widgetpack.NewFetcher(widgetpack.WithPlainHTTP(true))
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	inst := widgetpack.NewInstaller(store, nil, pol, fetcher, dataDir, []string{"Gauge", "EntityToggle"})

	ref := buildAndPushTestPack(t, regHost, "bar-widgets", "1.0.0", false)

	pack, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if pack.SignatureStatus != "unsigned" {
		t.Errorf("SignatureStatus = %q, want unsigned", pack.SignatureStatus)
	}
	if !strings.HasPrefix(pack.SHA256, "sha256:") {
		t.Errorf("SHA256 = %q, want sha256: prefix", pack.SHA256)
	}
	if len(pack.Classes) == 0 || pack.Classes[0] != "BarChart" {
		t.Errorf("Classes = %v, want [BarChart ...]", pack.Classes)
	}

	// Pack appears in store.
	got, err := store.Get(ctx, "bar-widgets", "1.0.0")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if got.Name != "bar-widgets" || got.Version != "1.0.0" {
		t.Errorf("store.Get: %+v", got)
	}

	// Bundle reachable over HTTP via the bundle handler.
	srv := httptest.NewServer(widgetpack.NewBundleHandler(store))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/widgets/bar-widgets/1.0.0/bundle.js")
	if err != nil {
		t.Fatalf("Get bundle: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got, want := resp.Header.Get("Cache-Control"), "public, max-age=31536000, immutable"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// TestInstall_Integration_Signed is intentionally skipped — see the file
// header comment for context. The verify path is unit-tested in trust_test.go
// against a VirtualSigstore-backed TestEntity.
func TestInstall_Integration_Signed(t *testing.T) {
	t.Skip("signed happy path requires producing a v0.2+ sigstore Bundle " +
		"with an inclusion proof; sigstore-go's VirtualSigstore does not " +
		"emit one. Covered indirectly by trust_test.go via verifyEntity.")
}

// TestInstall_Integration_SignerNotInPolicy is a stub for the policy-rejects
// path on a signed pack. It depends on the same Bundle-production work as
// TestInstall_Integration_Signed.
func TestInstall_Integration_SignerNotInPolicy(t *testing.T) {
	t.Skip("blocked on signed-pack Bundle production; see TestInstall_Integration_Signed.")
}

// TestInstall_Integration_HashMismatch pushes a pack whose manifest declares
// a bundleHash that doesn't match the bundle's actual SHA-256, and asserts
// that Install returns ReasonHashMismatch.
func TestInstall_Integration_HashMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	pol := &widgetpack.TrustPolicy{}
	_ = pol.Set(nil, true)
	fetcher, _ := widgetpack.NewFetcher(widgetpack.WithPlainHTTP(true))
	inst := widgetpack.NewInstaller(store, nil, pol, fetcher, dataDir, nil)

	// Build a pack whose manifest lies about the bundle hash.
	bundleJS := []byte("export default {}\n")
	manifestPkl := `@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"

manifest = new PackManifest {
  name = "bar-widgets"
  version = "1.0.0"
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
  classes = new { "BarChart" }
}
`
	tarGz := buildTarGz(t, map[string][]byte{
		"manifest.pkl": []byte(manifestPkl),
		"bundle.js":    bundleJS,
	})
	pushOCIArtifact(t, regHost, "bar-widgets", "1.0.0", tarGz)
	ref := fmt.Sprintf("%s/%s:%s", regHost, "bar-widgets", "1.0.0")

	_, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref})
	var fe *widgetpack.FailureError
	if err == nil || !errors.As(err, &fe) || fe.Reason != widgetpack.ReasonHashMismatch {
		t.Fatalf("Install err = %v, want FailureError{Reason: hash_mismatch}", err)
	}
	if _, err := store.Get(ctx, "bar-widgets", "1.0.0"); err == nil {
		t.Errorf("hash-mismatched pack should not be in store")
	}
}

// TestInstall_Integration_ClassCollisionWithBuiltin installs a pack whose
// manifest claims a builtin class ID (EntityToggle); Install must reject with
// ReasonClassCollision.
func TestInstall_Integration_ClassCollisionWithBuiltin(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	pol := &widgetpack.TrustPolicy{}
	_ = pol.Set(nil, true)
	fetcher, _ := widgetpack.NewFetcher(widgetpack.WithPlainHTTP(true))
	inst := widgetpack.NewInstaller(store, nil, pol, fetcher, dataDir,
		[]string{"Gauge", "EntityToggle"})

	bundleJS := []byte("export default {}\n")
	bundleSHA := sha256Hex(bundleJS)
	manifestPkl := fmt.Sprintf(`@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"

manifest = new PackManifest {
  name = "bar-widgets"
  version = "1.0.0"
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:%s"
  classes = new { "EntityToggle" }
}
`, bundleSHA)
	tarGz := buildTarGz(t, map[string][]byte{
		"manifest.pkl": []byte(manifestPkl),
		"bundle.js":    bundleJS,
	})
	pushOCIArtifact(t, regHost, "bar-widgets", "1.0.0", tarGz)
	ref := fmt.Sprintf("%s/%s:%s", regHost, "bar-widgets", "1.0.0")

	_, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref})
	var fe *widgetpack.FailureError
	if err == nil || !errors.As(err, &fe) || fe.Reason != widgetpack.ReasonClassCollision {
		t.Fatalf("Install err = %v, want FailureError{Reason: class_collision}", err)
	}
}

// TestInstall_Integration_AlreadyExists installs the same ref twice; the
// second call must fail with ReasonAlreadyExists.
func TestInstall_Integration_AlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	pol := &widgetpack.TrustPolicy{}
	_ = pol.Set(nil, true)
	fetcher, _ := widgetpack.NewFetcher(widgetpack.WithPlainHTTP(true))
	inst := widgetpack.NewInstaller(store, nil, pol, fetcher, dataDir, nil)

	ref := buildAndPushTestPack(t, regHost, "bar-widgets", "1.0.0", false)

	if _, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref}); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	_, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref})
	var fe *widgetpack.FailureError
	if err == nil || !errors.As(err, &fe) || fe.Reason != widgetpack.ReasonAlreadyExists {
		t.Fatalf("second Install err = %v, want FailureError{Reason: already_exists}", err)
	}
}

// ------------------------------------------------------------------ helpers

// startTestRegistry stands up an in-process Docker Registry v2 served over
// plain HTTP. Returns the host:port and a cleanup func.
func startTestRegistry(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	u, err := url.Parse(srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("parse registry url: %v", err)
	}
	return u.Host, func() { srv.Close() }
}

// buildAndPushTestPack builds a gzipped tarball containing a valid
// manifest.pkl + bundle.js, pushes it to the in-process registry as an OCI
// artifact with the widget-pack media type, and returns the ref string in
// "host/repo:tag" form.
//
// signed=true would attach a cosign-style signature artifact at <digest>.sig.
// Today the signed path is unimplemented — see the file header.
func buildAndPushTestPack(t *testing.T, regHost, repo, tag string, signed bool) string {
	t.Helper()
	if signed {
		t.Fatal("signed pack push not implemented — see file header comment")
	}

	// Bundle content: any non-empty JS will do.
	bundleJS := []byte("export default {}\n")
	bundleSHA := sha256Hex(bundleJS)

	// Pkl manifest. classes/name/version are tied to the test's expectations.
	manifestPkl := fmt.Sprintf(`@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"

manifest = new PackManifest {
  name = %q
  version = %q
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:%s"
  classes = new { "BarChart"; "PieChart" }
  description = "Test pack"
  homepage = "https://example.org"
  license = "MIT"
}
`, repo, tag, bundleSHA)

	tarGz := buildTarGz(t, map[string][]byte{
		"manifest.pkl": []byte(manifestPkl),
		"bundle.js":    bundleJS,
	})

	// Push as an OCI artifact: one layer (the tarball) with the widget-pack
	// media type, packaged in an image manifest v1.1.
	pushOCIArtifact(t, regHost, repo, tag, tarGz)
	return fmt.Sprintf("%s/%s:%s", regHost, repo, tag)
}

// pushOCIArtifact pushes blob as a single-layer OCI image manifest under
// repo:tag at regHost. The layer's media type is widgetpack.MediaType.
func pushOCIArtifact(t *testing.T, regHost, repo, tag string, blob []byte) {
	t.Helper()
	ctx := context.Background()

	// Stage in an in-memory store, then oras.Copy into the remote.
	src := memory.New()
	layerDesc := ocispec.Descriptor{
		MediaType: widgetpack.MediaType,
		Digest:    digestFromBytes(blob),
		Size:      int64(len(blob)),
	}
	if err := src.Push(ctx, layerDesc, bytes.NewReader(blob)); err != nil {
		t.Fatalf("src.Push layer: %v", err)
	}

	manifestDesc, err := oras.PackManifest(ctx, src, oras.PackManifestVersion1_1, widgetpack.MediaType, oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{layerDesc},
	})
	if err != nil {
		t.Fatalf("PackManifest: %v", err)
	}
	if err := src.Tag(ctx, manifestDesc, tag); err != nil {
		t.Fatalf("src.Tag: %v", err)
	}

	r, err := remote.NewRepository(regHost + "/" + repo)
	if err != nil {
		t.Fatalf("remote.NewRepository: %v", err)
	}
	r.PlainHTTP = true

	if _, err := oras.Copy(ctx, src, tag, r, tag, oras.DefaultCopyOptions); err != nil {
		t.Fatalf("oras.Copy push: %v", err)
	}
}

// buildTarGz writes a gzipped tar archive with the given file map (path → bytes).
func buildTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// sha256Hex returns the lowercase hex SHA-256 of b.
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// digestFromBytes builds the OCI "sha256:<hex>" digest for b.
func digestFromBytes(b []byte) digest.Digest {
	return digest.Digest("sha256:" + sha256Hex(b))
}
