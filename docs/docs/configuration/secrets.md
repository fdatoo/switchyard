# Secrets

!!! status-alpha "Alpha — shipped, interface evolving"
    `env:`, `file:`, and `keyring:` secret sources are shipped. Community module integrations (`vault:`, `1password:`, `bitwarden:`) are planned.

Secrets — API keys, passwords, tokens — are never stored in Pkl source or in the event store. They are declared as typed references in Pkl and resolved to plaintext by the Go runtime at config apply time, immediately before the resolved values are handed to driver instances.

## What NEVER to do

**Do not put secret values in Pkl source:**

```pkl
// WRONG — this commits the token to git
apiToken = "sk-abc123xyz"
```

If a secret value ends up in Pkl, it ends up in git history. Use a secret reference instead.

## Secret reference types

All four are defined in `switchyard:base` as **typealiases** — constrained `String` values with a mandatory prefix:

```pkl
module switchyard.base

typealias EnvSecret     = String(matches(Regex("env:[A-Z_][A-Z0-9_]*")))
typealias FileSecret    = String(matches(Regex("file:/.+")))
typealias KeyringSecret = String(matches(Regex("keyring:[^/]+/.+")))
typealias Secret        = String(matches(Regex("(env:[A-Z_]|file:/|keyring:).+")))
```

Secrets are plain strings with a prefix. The Pkl evaluator validates the format at evaluation time; the Go runtime resolves them to plaintext at apply time.

### `env:` — environment variable

```pkl
apiToken: base.Secret = "env:HUE_API_TOKEN"
```

Resolved by `os.Getenv("HUE_API_TOKEN")` at apply time. The value must be set in the daemon's environment — in a systemd unit via `EnvironmentFile=`, in a Docker container via `-e` or `--env-file`, or in a `.env` file sourced before launching `switchyardd`.

This is the lowest-friction option for simple setups.

### `file:` — file on disk

```pkl
apiToken: base.Secret = "file:/run/secrets/hue_api_token"
```

Resolved by reading the file at the given absolute path and trimming leading/trailing whitespace. The file must be readable by the user running `switchyardd`.

This is well-suited to secrets injected by a secrets manager (HashiCorp Vault agent, Kubernetes secrets projected into a volume, Docker secrets).

### `keyring:` — system keyring

```pkl
apiToken: base.Secret = "keyring:switchyard/hue_api_token"
```

Format: `keyring:<service>/<account>`. Resolved via the system keyring: macOS Keychain, Linux Secret Service (GNOME Keyring, KWallet), or Windows Credential Manager. Backed by the `go-keyring` library.

This is the best option for workstation installs where the operator wants OS-level secret storage with no plaintext files.

To pre-populate a keyring secret from the CLI:

```
$ secret-tool store --label "Hue API token" service switchyard user hue_api_token
```

Or on macOS:

```
$ security add-generic-password -s switchyard -a hue_api_token -w "sk-abc123xyz"
```

## How resolution works

At `switchyard config apply` time, after the Pkl evaluator produces a `ConfigSnapshot`, the secret resolver walks the snapshot and replaces every tagged secret reference with its resolved plaintext value.

The Pkl evaluator serializes secret references as tagged strings: `"__secret__:env:HUE_API_TOKEN"`, `"__secret__:file:/run/secrets/hue"`, `"__secret__:keyring:switchyard/hue_api_token"`. The Go resolver detects these tags and dispatches to the appropriate resolver.

**Resolved values are never persisted:**

- Not written to the event store. `ConfigApplied` events carry diff metadata only.
- Not printed in `switchyard config apply` diff output.
- Not written to any log file.
- Passed to the carport supervisor in memory, held only for the duration of driver instance startup.

If a secret cannot be resolved (variable not set, file missing, keyring entry absent), `Apply` returns an error before any side-effects occur. No driver instance is started or restarted with a partially-resolved config.

## Using secrets in driver config

```pkl
// drivers.pkl
import "switchyard:base"    as base
import "switchyard:carport" as carport
import "driver:hue"     as hue

drivers: Listing<carport.DriverInstance> = new {
  new hue.HueInstance {
    id         = "hue_main"
    driverName = "hue"
    bridgeHost = "10.0.0.42"
    // Secret reference — resolved at apply time
    apiToken   = "env:HUE_API_TOKEN"
  }
}
```

## Community modules (planned)

!!! status-planned "Planned — not yet implemented"
    These secret source types are designed but not yet shipped. They will be installable as community Pkl modules.

| Module | Example |
|---|---|
| `vault:` | HashiCorp Vault KV secret lookup |
| `1password:` | 1Password secret reference by vault/item/field |
| `bitwarden:` | Bitwarden CLI secret lookup |

When these ship, they will follow the same `Secret` interface. Your Pkl config will import the community module and declare a typed reference; the Go resolver dispatches to the appropriate backend.

## Checklist for production use

- Use `env:` secrets with a systemd `EnvironmentFile=` pointing at a file with mode `0600` owned by the daemon user.
- Use `file:` secrets with files mounted from a secrets manager (Vault agent, Kubernetes secret projection).
- Use `keyring:` secrets on workstation installs.
- Never commit `.env` files containing actual secret values. Add them to `.gitignore`.
- Rotate secrets by updating the environment variable or file, then running `switchyard config apply`. If the secret is an `env:` reference whose variable name did not change, the driver instance config hash does not change, so the driver is not restarted — the new value takes effect on the next driver restart or on a subsequent config apply that does change the instance.
