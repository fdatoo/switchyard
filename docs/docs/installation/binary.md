# Static binary

!!! status-alpha "Alpha — shipped, interface evolving"
    Binaries are published to GitHub Releases. Signatures are generated with sigstore/cosign. The release matrix and artifact names are stable but may gain additional platforms in future releases.

The static binary is the simplest install method. There are no runtime dependencies — drop the binary on any supported host and it runs.

## Supported platforms

| Platform | Architecture | Binary name |
|---|---|---|
| Linux | x86-64 (amd64) | `gohomed_linux_amd64`, `gohome_linux_amd64` |
| Linux | arm64 | `gohomed_linux_arm64`, `gohome_linux_arm64` |
| Linux | armv7 | `gohomed_linux_armv7`, `gohome_linux_armv7` |
| macOS | arm64 (Apple Silicon) | `gohomed_darwin_arm64`, `gohome_darwin_arm64` |
| macOS | x86-64 | `gohomed_darwin_amd64`, `gohome_darwin_amd64` |
| Windows | x86-64 (amd64) | `gohomed_windows_amd64.exe`, `gohome_windows_amd64.exe` |

## Download

=== "Linux (amd64)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/gohome/releases/download/${VERSION}"

    curl -fsSL "${BASE}/gohomed_linux_amd64" -o /tmp/gohomed
    curl -fsSL "${BASE}/gohome_linux_amd64"  -o /tmp/gohome
    ```

=== "Linux (arm64)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/gohome/releases/download/${VERSION}"

    curl -fsSL "${BASE}/gohomed_linux_arm64" -o /tmp/gohomed
    curl -fsSL "${BASE}/gohome_linux_arm64"  -o /tmp/gohome
    ```

=== "macOS (Apple Silicon)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/gohome/releases/download/${VERSION}"

    curl -fsSL "${BASE}/gohomed_darwin_arm64" -o /tmp/gohomed
    curl -fsSL "${BASE}/gohome_darwin_arm64"  -o /tmp/gohome
    ```

=== "macOS (Intel)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/gohome/releases/download/${VERSION}"

    curl -fsSL "${BASE}/gohomed_darwin_amd64" -o /tmp/gohomed
    curl -fsSL "${BASE}/gohome_darwin_amd64"  -o /tmp/gohome
    ```

=== "Windows (amd64)"

    Run in PowerShell:

    ```powershell
    $VERSION = "v0.1.0"
    $BASE    = "https://github.com/fynn-labs/gohome/releases/download/$VERSION"

    Invoke-WebRequest "$BASE/gohomed_windows_amd64.exe" -OutFile "$Env:TEMP\gohomed.exe"
    Invoke-WebRequest "$BASE/gohome_windows_amd64.exe"  -OutFile "$Env:TEMP\gohome.exe"
    ```

Replace `v0.1.0` with the latest version from the [Releases page](https://github.com/fynn-labs/gohome/releases).

## Verify the signature

Every artifact on the release page is signed with [sigstore/cosign](https://docs.sigstore.dev/). Verifying ensures you received an untampered binary built from the canonical source.

**Install cosign** if you don't have it:

=== "Linux (amd64)"
    ```bash
    curl -fsSL https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64 \
      -o /usr/local/bin/cosign && chmod +x /usr/local/bin/cosign
    ```

=== "Linux (arm64)"
    ```bash
    curl -fsSL https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-arm64 \
      -o /usr/local/bin/cosign && chmod +x /usr/local/bin/cosign
    ```

=== "macOS"
    ```bash
    brew install cosign
    ```

=== "Windows"
    ```powershell
    winget install sigstore.cosign
    ```

**Verify `gohomed`:**

The release artifacts use either the cosign v2 bundle format (a single `.bundle` file) or the older v1 format (separate `.pem` and `.sig` files), depending on the cosign version used to sign the release. Check the release page to see which files are present.

```bash
VERSION="v0.1.0"
BASE="https://github.com/fynn-labs/gohome/releases/download/${VERSION}"

# cosign v2 (recommended) — uses a single .bundle file
cosign verify-blob \
  --bundle "${BASE}/gohomed_linux_amd64.bundle" \
  --certificate-identity "https://github.com/fynn-labs/gohome/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  /tmp/gohomed

# cosign v1 (if .sig and .pem are provided instead of .bundle)
cosign verify-blob \
  --certificate "${BASE}/gohomed_linux_amd64.pem" \
  --signature   "${BASE}/gohomed_linux_amd64.sig" \
  --certificate-identity "https://github.com/fynn-labs/gohome/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  /tmp/gohomed
```

A successful verification prints:

```
Verified OK
```

Repeat the same command for `gohome`, substituting the binary name throughout.

!!! tip "Check the RELEASE.yaml manifest"
    Each release also ships a `RELEASE.yaml` that lists SHA-256 checksums and the driver-compatibility matrix for that version. Download it from the same release page for a quick checksum check:

    ```bash
    # Expected checksum (from the release manifest)
    curl -fsSL "${BASE}/RELEASE.yaml" | grep gohomed_linux_amd64

    # Actual checksum (of the binary you downloaded)
    sha256sum /tmp/gohomed
    ```

    Compare the two outputs — they must match exactly.

## Place binaries in $PATH

=== "Linux / macOS"

    ```bash
    chmod +x /tmp/gohomed /tmp/gohome
    sudo mv /tmp/gohomed /usr/local/bin/gohomed
    sudo mv /tmp/gohome  /usr/local/bin/gohome
    ```

    Confirm:

    ```bash
    gohomed --version
    gohome --version
    ```

=== "Windows"

    Move the `.exe` files to a directory on your `%PATH%`, or add their location to `%PATH%` via System Properties → Environment Variables.

    Confirm in PowerShell:

    ```powershell
    & "$Env:TEMP\gohomed.exe" --version
    & "$Env:TEMP\gohome.exe"  --version
    ```

## Self-update

Once installed, you can update in-place without re-running these steps:

```bash
# Downloads the new binary, verifies its signature, atomically replaces,
# and restarts the daemon if it is running under systemd.
gohome self-update
```

`gohome self-update` honours the same sigstore signature verification as the manual steps above. It will not replace the binary if verification fails.

## Next step

Continue to [First run](first-run.md) to create your config directory and start the daemon.
