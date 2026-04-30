# Auth & Policy Setup

gohome ships passkey-first authentication, scoped API tokens, and Pkl-declared roles and policies. This guide walks through initial bootstrap, day-to-day token management, policy authoring, and MCP HTTP transport configuration.

## Bootstrap walkthrough

### 1. Define a user in `auth/users.pkl`

```pkl
// auth/users.pkl
amends "gohome:config"

import "gohome:auth" as auth

users {
  new auth.User {
    slug         = "alice"
    display_name = "Alice"
    roles        = List(new auth.Role {
      slug         = "admin"
      display_name = "Admin"
    })
  }
}
```

### 2. Apply the config

```sh
gohome config apply
```

The daemon reloads policy immediately; no restart required.

### 3. Issue an enrollment token

```sh
gohome auth bootstrap alice
```

Sample output:

```
╔══ ENROLLMENT TOKEN — STORE THIS NOW ══╗
enc_enroll_v1.abc123xyz
╚═══════════════════════════════════════╝
Expires: 2026-04-28T10:00:00Z
```

The default TTL is 1 hour. Pass `--ttl` to adjust (e.g. `--ttl 24h`).

### 4. Register a passkey

Open the web UI and redeem the enrollment token. The UI prompts for a passkey credential via WebAuthn. Once registered, the enrollment token is consumed and the user can sign in with their passkey — no password required.

---

## WebAuthn UX notes

**Discoverable credentials (passkeys).** gohome uses `residentKey: required` so no username is needed at sign-in. The authenticator discovers the credential automatically.

**Multi-device sync.** Passkeys created on Apple devices sync via iCloud Keychain. On Android / Chrome, they sync via Google Password Manager. Hardware security keys (FIDO2) are also supported but are not synced — treat them as a single-device credential.

**Sign-count discipline.** WebAuthn sign counts help detect cloned authenticators. gohome tracks the count per credential. On a mismatch the daemon logs a warning at `WARN` level but still completes authentication (cloning is possible but not proven). Check `gohomed` logs if you see repeated mismatch warnings.

**Lost passkey.** Issue a new enrollment token and re-register:

```sh
gohome auth bootstrap alice
# follow the URL in a browser and register a new passkey
```

Previous credentials for the user remain active unless explicitly revoked via the web UI.

---

## Token issuance

Tokens are always minted for the authenticated caller — there is no admin-on-behalf-of flow.

### Create a token

```sh
gohome auth tokens create --label "my-script"
```

Sample output:

```
╔══ TOKEN — STORE THIS NOW ══╗
ghp_v1.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
╚════════════════════════════╝
Token id: tok_01hx3q9vz2fgk8p7m4nj
```

The token is shown exactly once. Store it securely before closing the terminal.

### Use a token

Tokens are Bearer tokens. Pass them in the `Authorization` header:

```sh
curl -H "Authorization: Bearer ghp_v1.xxx..." http://localhost:8080/api/v1/entities
```

### Revoke a token

```sh
gohome auth tokens revoke tok_01hx3q9vz2fgk8p7m4nj
```

---

## Policy authoring tutorial

Policies are declared in Pkl and applied with `gohome config apply`. Changes take effect immediately with no daemon restart.

### Define roles in `auth/users.pkl`

Roles are defined inline on each `User`. Each `Role` has a `slug` and `display_name`:

```pkl
// auth/users.pkl
amends "gohome:config"

import "gohome:auth" as auth

local admin_role = new auth.Role {
  slug         = "admin"
  display_name = "Admin"
}

local viewer_role = new auth.Role {
  slug         = "viewer"
  display_name = "Viewer"
}

users {
  new auth.User {
    slug         = "alice"
    display_name = "Alice"
    roles        = List(admin_role)
  }
  new auth.User {
    slug         = "bob"
    display_name = "Bob"
    roles        = List(viewer_role)
  }
}
```

### Write capability rules in `auth/policy.pkl`

Policies map roles to `allow` / `deny` lists of `CapabilityRule`:

```pkl
// auth/policy.pkl
amends "gohome:config"

import "gohome:auth"   as auth
import "gohome:policy" as pol

local admin_role = new auth.Role {
  slug         = "admin"
  display_name = "Admin"
}

local viewer_role = new auth.Role {
  slug         = "viewer"
  display_name = "Viewer"
}

policies {
  new pol.Policy {
    name     = "admin-full-access"
    subjects = List(admin_role)
    allow    = List(
      new pol.CapabilityRule {
        verbs    = List("read", "write", "call", "admin")
        services = List("*")
        targets  = pol.AnyEntity
      }
    )
  }
  new pol.Policy {
    name     = "viewer-read-only"
    subjects = List(viewer_role)
    allow    = List(
      new pol.CapabilityRule {
        verbs    = List("read")
        services = List("EntityService")
        targets  = pol.AnyEntity
      }
    )
  }
}
```

### Apply and verify

```sh
gohome config apply
```

Verify a specific permission with `gohome auth explain`. The `--action` flag takes a gRPC `Service.Method` and `--target` takes a `kind:id` pair:

```sh
gohome auth explain \
  --user alice \
  --action EntityService.CallCapability \
  --verb call \
  --target entity:light.kitchen
```

Sample output:

```
Decision: ALLOWED
Reason:   matched policy admin-full-access (allow rule)
Rule:     admin-full-access
```

```sh
gohome auth explain \
  --user bob \
  --action EntityService.CallCapability \
  --verb write \
  --target entity:sensor.temperature_living_room
```

Sample output:

```
Decision: DENIED
Reason:   no allow rule grants write on EntityService for viewer
```

---

## MCP HTTP transport setup

From C9, `gohomed` exposes `/mcp` as a [Streamable HTTP MCP](https://modelcontextprotocol.io/docs/concepts/transports) endpoint in addition to the Unix-socket stdio transport. Authentication uses Bearer tokens.

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or equivalent:

```json
{
  "mcpServers": {
    "gohome": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

### Claude Code (CLI)

```json
{
  "mcpServers": {
    "gohome": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

Add this to your `~/.claude/claude_desktop_config.json` or project-level `.claude/mcp.json`.

Create a dedicated token for each client (sign in first, then run):

```sh
gohome auth tokens create --label "claude-desktop"
gohome auth tokens create --label "claude-code"
```

---

## Troubleshooting

### "Why was X denied?"

Run `gohome auth explain` to trace the policy decision. The `--action` flag takes a `Service.Method` (gRPC-style), `--verb` one of `read`, `write`, `call`, or `admin`, and `--target` an optional `kind:id`:

```sh
gohome auth explain \
  --user <slug> \
  --action <Service>.<Method> \
  --verb <verb> \
  --target <kind>:<id>
```

Example:

```sh
gohome auth explain \
  --user bob \
  --action EntityService.ListEntities \
  --verb read
```

### Passkey lost

Issue a new enrollment token and re-register via the web UI:

```sh
gohome auth bootstrap <slug>
```

### Throttle tripped

Authentication attempts are rate-limited (default: 10 failures per 10-minute window). After tripping the throttle, wait for the window to expire or restart `gohomed` (the throttle counter is in-memory and resets on restart).

### "Invalid session" after server restart

Sessions use signed cookies. If the cookie signing key changes between restarts, existing sessions are invalidated. Ensure `GOHOME_SESSION_KEY` (or the equivalent config field) is set to a stable value across restarts — do not let it default to a random value in production.
