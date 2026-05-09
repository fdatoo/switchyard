// Package widgetpack — Installer chains the §15.4 install pipeline:
//
//	OCI pull → cosign verify → stage → manifest validate → bundle hash check
//	→ SDK compatibility → class collision check → atomic commit → emit event
//
// Each stage maps a known failure mode to a stable FailureReason that callers
// (Connect handler, CLI) translate to user-facing messages. Failures before
// the atomic-rename commit roll back by removing the staging directory; the
// post-rename Store.Add failure rolls back by removing the final directory.
package widgetpack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// HostSDKVersion is the @switchyard/widget-sdk version this build is
// compatible with. A manifest's sdkVersion must match this in major-version
// semver (i.e. same major; minor/patch may differ). Bump on SDK breaking
// changes.
const HostSDKVersion = "1.0.0"

// ErrInstallFailed wraps internal install errors that don't have a dedicated
// FailureReason — file-system failures, mkdir errors, and other I/O. Use
// FailureError for any failure mode that callers may need to react to.
var ErrInstallFailed = errors.New("widgetpack: install failed")

// FailureReason is a stable string carried in FailureError so callers can
// map errors to user-facing messages without parsing the message text.
type FailureReason string

const (
	ReasonBadRef              FailureReason = "bad_ref"
	ReasonRegistryUnreachable FailureReason = "registry_unreachable"
	ReasonBadArtifact         FailureReason = "bad_artifact"
	ReasonSignatureInvalid    FailureReason = "signature_invalid"
	ReasonHashMismatch        FailureReason = "hash_mismatch"
	ReasonSDKIncompatible     FailureReason = "sdk_incompatible"
	ReasonClassCollision      FailureReason = "class_collision"
	ReasonManifestInvalid     FailureReason = "manifest_invalid"
	ReasonAlreadyExists       FailureReason = "already_exists"
)

// FailureError is returned from Install for known failure modes. The Reason
// field is a stable token; the wrapped Err carries diagnostic detail.
type FailureError struct {
	Reason FailureReason
	Err    error
}

func (e *FailureError) Error() string {
	if e.Err == nil {
		return string(e.Reason)
	}
	return string(e.Reason) + ": " + e.Err.Error()
}

func (e *FailureError) Unwrap() error { return e.Err }

// InstallRequest carries the parameters for a pack installation.
type InstallRequest struct {
	// Ref is the OCI reference (repo:tag) of the pack to install.
	Ref string
}

// DashboardLister is the subset of dashboard.Backend that Uninstall queries
// to build the in-use class set. Today (F-156 unimplemented) the only
// production binding returns an empty list — Uninstall always proceeds.
type DashboardLister interface {
	ClassRefs(ctx context.Context) ([]string, error) // list of "<pack>/<class>" or builtin class IDs in any dashboard
}

// emptyDashboardLister is the default; replace via Installer.SetDashboardLister.
type emptyDashboardLister struct{}

func (emptyDashboardLister) ClassRefs(_ context.Context) ([]string, error) { return nil, nil }

// Installer chains the install pipeline. It is safe for concurrent use:
// concurrent Install calls for the same (name@version) are serialized via
// muInflight; calls for different keys run independently.
type Installer struct {
	store          *Store
	verifier       *Verifier
	policy         *TrustPolicy
	fetcher        *Fetcher
	builtinClasses []string
	dl             DashboardLister

	// muInflight gates concurrent installs of the same (name@version) so we
	// don't race two callers into the rename step. Keyed by name@version.
	muInflight sync.Map // string -> *sync.Mutex
}

// NewInstaller wires the install pipeline. builtinClasses is the set of
// builtin class IDs (e.g. "Gauge", "EntityToggle") used for collision checks
// — packs may not register a class whose ID is already builtin.
//
// dl is the DashboardLister used by Uninstall to check whether a pack's
// classes are in use. Pass nil to use the default empty lister (always
// permits uninstall); F-156 will plumb the real lister.
func NewInstaller(
	store *Store,
	verifier *Verifier,
	policy *TrustPolicy,
	fetcher *Fetcher,
	builtinClasses []string,
	dl DashboardLister,
) *Installer {
	if dl == nil {
		dl = emptyDashboardLister{}
	}
	return &Installer{
		store:          store,
		verifier:       verifier,
		policy:         policy,
		fetcher:        fetcher,
		builtinClasses: builtinClasses,
		dl:             dl,
	}
}

// Install runs the full §15.4 flow.
//
// Returns *FailureError for known failure modes (bad ref, signature reject,
// hash mismatch, etc.) and a generic ErrInstallFailed-wrapped error for
// I/O / system failures. On success the returned *InstalledPack mirrors the
// entry that was added to the Store.
func (i *Installer) Install(ctx context.Context, req InstallRequest) (*InstalledPack, error) {
	// Step 0. Validate request.
	if req.Ref == "" {
		return nil, &FailureError{Reason: ReasonBadRef, Err: errors.New("ref required")}
	}

	// Step 1. Pull artifact + signature from the OCI registry.
	art, err := i.fetcher.Fetch(ctx, req.Ref)
	if err != nil {
		return nil, &FailureError{Reason: ReasonRegistryUnreachable, Err: err}
	}

	// Step 2. Verify signature against trust policy. The verifier handles
	// the "no signature + AllowUnsigned" case internally.
	//
	// A nil verifier is tolerated: it lets the daemon start when the production
	// trust root is not yet wired (NewProductionVerifier is currently stubbed).
	// In that mode, unsigned packs are still installable when the policy allows
	// them; signed packs are rejected with ReasonSignatureInvalid.
	var vres *VerificationResult
	if i.verifier != nil {
		vres, err = i.verifier.Verify(ctx, art.LayerBlob, art.SignatureBundle, i.policy)
		if err != nil {
			return nil, &FailureError{Reason: ReasonSignatureInvalid, Err: err}
		}
	} else {
		if len(art.SignatureBundle) > 0 {
			return nil, &FailureError{
				Reason: ReasonSignatureInvalid,
				Err:    errors.New("verifier not configured; cannot verify signed pack"),
			}
		}
		if i.policy == nil || !i.policy.AllowUnsigned() {
			return nil, &FailureError{Reason: ReasonSignatureInvalid, Err: ErrUnsignedNotAllowed}
		}
		vres = &VerificationResult{Status: "unsigned"}
	}

	// Step 3. Stage the tarball into <DataDir>/widgets/.staging/<rand>/.
	// On any error before the rename below, the deferred cleanup wipes this.
	stagingRoot := filepath.Join(i.store.Root(), ".staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, fmt.Errorf("%w: mkdir staging: %v", ErrInstallFailed, err)
	}
	stagingDir, err := os.MkdirTemp(stagingRoot, "pack-")
	if err != nil {
		return nil, fmt.Errorf("%w: stage tmp: %v", ErrInstallFailed, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	if err := untarGz(art.LayerBlob, stagingDir); err != nil {
		return nil, &FailureError{Reason: ReasonBadArtifact, Err: err}
	}

	// Step 4. Manifest validate (Pkl evaluation enforces structural constraints).
	manifest, err := EvalManifest(ctx, filepath.Join(stagingDir, "manifest.pkl"))
	if err != nil {
		return nil, &FailureError{Reason: ReasonManifestInvalid, Err: err}
	}

	// Step 5. Hash verify: bundle file's SHA-256 must match manifest.bundleHash.
	bundlePath := filepath.Join(stagingDir, manifest.Bundle)
	bundleSHA, err := sha256File(bundlePath)
	if err != nil {
		return nil, &FailureError{Reason: ReasonBadArtifact, Err: err}
	}
	computed := "sha256:" + bundleSHA
	if computed != manifest.BundleHash {
		return nil, &FailureError{
			Reason: ReasonHashMismatch,
			Err:    fmt.Errorf("computed %s, manifest %s", computed, manifest.BundleHash),
		}
	}

	// Step 6. SDK compatibility (major-version equality for v1).
	if !semverMajorEqual(manifest.SDKVersion, HostSDKVersion) {
		return nil, &FailureError{
			Reason: ReasonSDKIncompatible,
			Err:    fmt.Errorf("manifest sdkVersion=%s host=%s", manifest.SDKVersion, HostSDKVersion),
		}
	}

	// Step 7. Class collision check.
	if err := i.checkCollisions(ctx, manifest); err != nil {
		return nil, &FailureError{Reason: ReasonClassCollision, Err: err}
	}

	// Per-(name@version) install-mutex: serialize concurrent attempts to
	// install the exact same pack version. Mutex acquisition happens after
	// the manifest is parsed (we need name+version to key it).
	key := manifest.Name + "@" + manifest.Version
	muIface, _ := i.muInflight.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Step 7b. Already-exists check (must be inside the mutex to be race-free).
	if _, err := i.store.Get(ctx, manifest.Name, manifest.Version); err == nil {
		return nil, &FailureError{Reason: ReasonAlreadyExists, Err: errors.New(key)}
	}

	// Step 8. Commit: atomic rename staging → <Root>/<name>/<version>/.
	finalDir := filepath.Join(i.store.Root(), manifest.Name, manifest.Version)
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return nil, fmt.Errorf("%w: mkdir parent: %v", ErrInstallFailed, err)
	}
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return nil, fmt.Errorf("%w: rename: %v", ErrInstallFailed, err)
	}
	committed = true

	// Step 9 (reload) is a no-op for the daemon: served bundles are read on
	// demand from the on-disk root, and Store.Add (below) emits the event
	// (Step 10) that wakes any subscribers.
	pack := InstalledPack{
		Name:            manifest.Name,
		Version:         manifest.Version,
		SHA256:          computed,
		SignatureStatus: vres.Status,
		SignerIdentity:  vres.SignerIdentity,
		Classes:         manifest.Classes,
		Description:     manifest.Description,
		Homepage:        manifest.Homepage,
		License:         manifest.License,
	}
	if err := i.store.Add(ctx, pack); err != nil {
		// Narrow rollback window: the rename succeeded but the registry
		// write failed. Remove the just-created final directory so the next
		// attempt can retry cleanly.
		_ = os.RemoveAll(finalDir)
		return nil, fmt.Errorf("%w: store.Add: %v", ErrInstallFailed, err)
	}
	// Step 11. Return the installed-pack snapshot.
	return &pack, nil
}

// checkCollisions errors if any class in m collides with a builtin or with
// a class already registered by a different pack.
//
// Builtins are unqualified ("Gauge"); pack classes are namespaced
// ("packname/ClassID"). A new pack may not reuse a builtin id, and may not
// reuse a (packname/classid) that another installed pack has registered.
// Re-installing the same (name, version) is excluded — that case is handled
// by the already-exists check.
func (i *Installer) checkCollisions(ctx context.Context, m *Manifest) error {
	taken := make(map[string]bool)
	for _, b := range i.builtinClasses {
		taken[b] = true
	}
	packs, err := i.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list packs: %w", err)
	}
	for _, p := range packs {
		if p.Name == m.Name && p.Version == m.Version {
			continue
		}
		for _, c := range p.Classes {
			taken[p.Name+"/"+c] = true
		}
	}
	for _, c := range m.Classes {
		if taken[c] || taken[m.Name+"/"+c] {
			return fmt.Errorf("class %q collides", c)
		}
	}
	return nil
}

// Uninstall removes a pack. With force=false, returns an error if any
// dashboard references one of the pack's classes.
func (i *Installer) Uninstall(ctx context.Context, name, version string, force bool) error {
	pack, err := i.store.Get(ctx, name, version)
	if err != nil {
		return err
	}
	if !force {
		refs, err := i.dl.ClassRefs(ctx)
		if err != nil {
			return fmt.Errorf("widgetpack: list class refs: %w", err)
		}
		inUse := make([]string, 0)
		for _, c := range pack.Classes {
			full := name + "/" + c
			for _, ref := range refs {
				if ref == full {
					inUse = append(inUse, full)
					break
				}
			}
		}
		if len(inUse) > 0 {
			return fmt.Errorf("widgetpack: pack %s in use by classes %v", pack.Name, inUse)
		}
	}
	if err := os.RemoveAll(filepath.Join(i.store.Root(), name, version)); err != nil {
		return fmt.Errorf("widgetpack: remove dir: %w", err)
	}
	return i.store.Remove(ctx, name, version)
}

// untarGz extracts a gzipped tarball into dest. Symlinks, devices, and
// other non-regular entries are skipped — packs are not permitted to ship
// them. Any path that, after Clean+Join, falls outside dest is rejected as
// a path-traversal attempt.
func untarGz(blob []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	// Resolve dest once so we can compare each entry's destination against it.
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("abs dest: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		// Reject absolute paths and any entry whose joined path escapes dest.
		// strings.HasPrefix(clean, "..") alone catches single-level escapes;
		// the full Abs-and-prefix check below also catches multi-segment
		// traversals like "subdir/../../../etc/passwd".
		clean := filepath.Clean(hdr.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("path escape: %s", hdr.Name)
		}
		full := filepath.Join(absDest, clean)
		// The Join+absolute prefix check is the authoritative defense.
		if !strings.HasPrefix(full, absDest+string(filepath.Separator)) && full != absDest {
			return fmt.Errorf("path escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// Skip symlinks, devices, fifos, etc. — never legitimate in a
			// widget pack.
		}
	}
}

// sha256File returns the lowercase hex SHA-256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// semverMajorEqual reports whether a and b have equal major versions.
// Both "1.2.3" and "v1.2.3" are accepted; non-numeric majors yield false.
func semverMajorEqual(a, b string) bool {
	ma, err1 := majorOf(a)
	mb, err2 := majorOf(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return ma == mb
}

// majorOf parses the leading integer of a semver string, tolerating an
// optional leading "v". "1.2.3" → 1, "v2.0.0-rc1" → 2.
func majorOf(v string) (int, error) {
	v = strings.TrimPrefix(v, "v")
	end := strings.IndexAny(v, ".+-")
	if end < 0 {
		end = len(v)
	}
	if end == 0 {
		return 0, fmt.Errorf("empty major in %q", v)
	}
	return strconv.Atoi(v[:end])
}
