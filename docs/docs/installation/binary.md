# Static binary

!!! status-alpha "Alpha — shipped, interface evolving"
    Binaries are published to GitHub Releases. Signatures are generated with sigstore/cosign. The release matrix and artifact names are stable but may gain additional platforms in future releases.

The static binary is the simplest install method. There are no runtime dependencies — drop the binary on any supported host and it runs.

## Supported platforms

| Platform | Architecture | Binary name |
|---|---|---|
| Linux | x86-64 (amd64) | `switchyardd_linux_amd64`, `switchyard_linux_amd64` |
| Linux | arm64 | `switchyardd_linux_arm64`, `switchyard_linux_arm64` |
| Linux | armv7 | `switchyardd_linux_armv7`, `switchyard_linux_armv7` |
| macOS | arm64 (Apple Silicon) | `switchyardd_darwin_arm64`, `switchyard_darwin_arm64` |
| macOS | x86-64 | `switchyardd_darwin_amd64`, `switchyard_darwin_amd64` |
| Windows | x86-64 (amd64) | `switchyardd_windows_amd64.exe`, `switchyard_windows_amd64.exe` |

## Download

=== "Linux (amd64)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/switchyard/releases/download/${VERSION}"

    curl -fsSL "${BASE}/switchyardd_linux_amd64" -o /tmp/switchyardd
    curl -fsSL "${BASE}/switchyard_linux_amd64"  -o /tmp/switchyard
    ```

=== "Linux (arm64)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/switchyard/releases/download/${VERSION}"

    curl -fsSL "${BASE}/switchyardd_linux_arm64" -o /tmp/switchyardd
    curl -fsSL "${BASE}/switchyard_linux_arm64"  -o /tmp/switchyard
    ```

=== "macOS (Apple Silicon)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/switchyard/releases/download/${VERSION}"

    curl -fsSL "${BASE}/switchyardd_darwin_arm64" -o /tmp/switchyardd
    curl -fsSL "${BASE}/switchyard_darwin_arm64"  -o /tmp/switchyard
    ```

=== "macOS (Intel)"

    ```bash
    VERSION="v0.1.0"
    BASE="https://github.com/fynn-labs/switchyard/releases/download/${VERSION}"

    curl -fsSL "${BASE}/switchyardd_darwin_amd64" -o /tmp/switchyardd
    curl -fsSL "${BASE}/switchyard_darwin_amd64"  -o /tmp/switchyard
    ```

=== "Windows (amd64)"

    Run in PowerShell:

    ```powershell
    $VERSION = "v0.1.0"
    $BASE    = "https://github.com/fynn-labs/switchyard/releases/download/$VERSION"

    Invoke-WebRequest "$BASE/switchyardd_windows_amd64.exe" -OutFile "$Env:TEMP\switchyardd.exe"
    Invoke-WebRequest "$BASE/switchyard_windows_amd64.exe"  -OutFile "$Env:TEMP\switchyard.exe"
    ```

Replace `v0.1.0` with the latest version from the [Releases page](https://github.com/fynn-labs/switchyard/releases).

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

**Verify `switchyardd`:**

The release artifacts use either the cosign v2 bundle format (a single `.bundle` file) or the older v1 format (separate `.pem` and `.sig` files), depending on the cosign version used to sign the release. Check the release page to see which files are present.

```bash
VERSION="v0.1.0"
BASE="https://github.com/fynn-labs/switchyard/releases/download/${VERSION}"

# cosign v2 (recommended) — uses a single .bundle file
cosign verify-blob \
  --bundle "${BASE}/switchyardd_linux_amd64.bundle" \
  --certificate-identity "https://github.com/fynn-labs/switchyard/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  /tmp/switchyardd

# cosign v1 (if .sig and .pem are provided instead of .bundle)
cosign verify-blob \
  --certificate "${BASE}/switchyardd_linux_amd64.pem" \
  --signature   "${BASE}/switchyardd_linux_amd64.sig" \
  --certificate-identity "https://github.com/fynn-labs/switchyard/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  /tmp/switchyardd
```

A successful verification prints:

```
Verified OK
```

Repeat the same command for `switchyard`, substituting the binary name throughout.

!!! tip "Check the RELEASE.yaml manifest"
    Each release also ships a `RELEASE.yaml` that lists SHA-256 checksums and the driver-compatibility matrix for that version. Download it from the same release page for a quick checksum check:

    ```bash
    # Expected checksum (from the release manifest)
    curl -fsSL "${BASE}/RELEASE.yaml" | grep switchyardd_linux_amd64

    # Actual checksum (of the binary you downloaded)
    sha256sum /tmp/switchyardd
    ```

    Compare the two outputs — they must match exactly.

## Place binaries in $PATH

=== "Linux / macOS"

    ```bash
    chmod +x /tmp/switchyardd /tmp/switchyard
    sudo mv /tmp/switchyardd /usr/local/bin/switchyardd
    sudo mv /tmp/switchyard  /usr/local/bin/switchyard
    ```

    Confirm:

    ```bash
    switchyardd --version
    switchyard --version
    ```

=== "Windows"

    Move the `.exe` files to a directory on your `%PATH%`, or add their location to `%PATH%` via System Properties → Environment Variables.

    Confirm in PowerShell:

    ```powershell
    & "$Env:TEMP\switchyardd.exe" --version
    & "$Env:TEMP\switchyard.exe"  --version
    ```

## Self-update

Once installed, you can update in-place without re-running these steps:

```bash
# Downloads the new binary, verifies its signature, atomically replaces,
# and restarts the daemon if it is running under systemd.
switchyard self-update
```

`switchyard self-update` honours the same sigstore signature verification as the manual steps above. It will not replace the binary if verification fails.

## Next step

Continue to [First run](first-run.md) to create your config directory and start the daemon.
