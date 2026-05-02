# Auth & Policies

!!! status-planned "Planned — not yet implemented"
    The auth system is fully designed but not yet shipped. The Pkl schema is in place; the runtime (passkeys, sessions, policy enforcement) lands in C9.

switchyard is a multi-user system from day one. Users and their permissions are declared in `auth.pkl` using the same Pkl config model as the rest of the system. Credentials (passwords, passkeys, tokens) are stored separately in the runtime database — never in Pkl source.

## Users

Users are declared in `auth.pkl`:

```pkl
// auth.pkl
import "switchyard:auth" as auth

users: Listing<auth.User> = new {
  new auth.User {
    slug         = "fdatoo"
    display_name = "Fynn"
    roles        = List(roles.admin)
    active       = true
  }
  new auth.User {
    slug         = "partner"
    display_name = "Sam"
    roles        = List(roles.member)
    active       = true
  }
  new auth.User {
    slug         = "nora"
    display_name = "Nora"
    roles        = List(roles.kids)
    active       = true
    // Restrict to passkey only — password login disabled for this user
    password_allowed = false
  }
}
```

The full `User` shape:

```pkl
class User {
  slug:         String              // ^[a-z][a-z0-9_-]{1,31}$
  display_name: String
  roles:        List<Role>
  active:       Boolean = true

  password_allowed: Boolean = true
  passkey_allowed:  Boolean = true
  oidc_subject:     String? = null  // RESERVED for v1.x OIDC

  // For headless bootstrap: pre-stage a hash via `switchyard auth hash-password`.
  // Used on first login, ignored on subsequent reloads.
  bootstrap_password_hash: String? = null
}
```

User slugs are git-tracked and human-edited. Credentials (password hashes, passkey credential blobs, session tokens) live in the runtime SQLite database and never touch Pkl.

## Roles

switchyard has three built-in roles:

| Role | What it can do |
|---|---|
| `admin` | Full access to everything — config, users, tokens, all entities |
| `member` | Read and call any entity; cannot admin users or change config |
| `guest` | Read-only access to entities in areas explicitly shared with guests |

You can define custom roles with single-inheritance composition:

```pkl
roles: Listing<auth.Role> = new {
  new auth.Role {
    slug         = "kids"
    display_name = "Kids"
    inherits     = List(roles.member)  // inherits member baseline
  }
  new auth.Role {
    slug         = "cleaner"
    display_name = "Cleaner"
    inherits     = List()
  }
}
```

A user's effective permission set is the transitive union across all inherited roles. Cycles in the `inherits` graph fail `switchyard config validate`.

## Auth methods

### Passkeys (primary)

Passkeys (WebAuthn) are the primary auth method. They are phishing-resistant, multi-factor by design, and require no passwords to manage. The web UI drives the WebAuthn ceremony; the CLI can initiate enrollment via `switchyard auth bootstrap`.

### Password (fallback)

Argon2id-hashed passwords are supported as a fallback — useful for headless setups or environments where WebAuthn is impractical. Password login can be disabled per-user (`password_allowed = false`) or globally in `AuthSettings`.

### API tokens

Scoped API tokens are issued via `switchyard auth tokens create`. Tokens carry an explicit scope — which tools, services, and entity targets they can access. A token's effective permission is the intersection of the user's policy and the token's scope: a token can only narrow permissions, never widen them.

## Policies

Policies declare what roles are allowed to do. They are declared alongside users in `auth.pkl`:

```pkl
import "switchyard:auth"   as auth
import "switchyard:policy" as policy

policies: Listing<policy.Policy> = new {

  // Admins can do everything
  new policy.Policy {
    name     = "admin_full"
    subjects = List(roles.admin)
    allow    = List(new policy.CapabilityRule {
      targets = policy.AnyEntity
    })
  }

  // Members can read and call anything
  new policy.Policy {
    name     = "member_baseline"
    subjects = List(roles.member)
    allow    = List(new policy.CapabilityRule {
      verbs   = List("read", "call")
      targets = policy.AnyEntity
    })
  }

  // Kids — the "kids policy" example
  // Can read and call entities in their bedroom area,
  // but explicitly CANNOT call Locks or Alarms anywhere.
  new policy.Policy {
    name     = "kids_bedrooms_only"
    subjects = List(roles.kids)
    allow    = List(new policy.CapabilityRule {
      verbs   = List("read", "call")
      targets = new policy.EntitySelector {
        // Hierarchical — implicitly covers nora_room, milo_room, any future child area
        areas = List("upstairs")
      }
    })
    deny = List(new policy.CapabilityRule {
      verbs   = List("call")
      targets = new policy.EntitySelector {
        classes = List("Lock", "Alarm")
      }
    })
  }

}
```

### Policy evaluation

The evaluation rule is: **permitted if any allow rule matches AND no deny rule matches**. Deny wins. Default-deny when no allow matches.

Area selectors are hierarchical — a rule on `upstairs` matches all entities in any room nested under `upstairs`. Class selectors match by entity class name. Entity-id selectors match specific entity ids.

### `switchyard auth explain`

To debug a policy decision:

```
$ switchyard auth explain \
    --user nora \
    --action EntityService.CallCapability \
    --target entity:lock.front_door

Decision:    DENIED
Reason:      explicit_deny
Matching rule:
  Policy: kids_bedrooms_only
  Rule:   deny[0]
  Verbs:  ["call"]
  Targets: classes=["Lock", "Alarm"]
```

## Enforcement points

Policy is enforced at four points:

| Point | How |
|---|---|
| CLI (`switchyard`) | CLI authenticates via UDS (local → `system:local`, full access) or session cookie |
| Connect-RPC | Every RPC goes through the auth interceptor chain; policy checked before the handler |
| MCP server | Every tool call and resource subscription goes through the same policy runtime |
| Subscriptions | `policy_mode = "filter"` (default): denied entities are silently excluded. `policy_mode = "strict"`: the call fails if any entity in the subscription set is denied |

The `system:local` principal (a CLI call over the Unix domain socket) bypasses policy entirely — anyone with shell access can already edit the Pkl config and read the SQLite database.

## Auth events

Every auth action lands on the event log:

| Event | When |
|---|---|
| `LoginSucceeded` | Successful passkey or password login |
| `LoginFailed` | Failed attempt (bad credential, throttled, user inactive) |
| `TokenMinted` | API token issued |
| `TokenRevoked` | Token revoked |
| `PolicyDenied` | An authorization check failed |
| `PolicyCompiled` | Policy compiler ran (on startup and on every `switchyard config apply`) |

These events sit in the same cursor-ordered timeline as `StateChanged` and `ConfigApplied` events, so you can trace a policy-denied at cursor 4821 back to the preceding config apply.

## AuthSettings

Global auth behaviour is configured via `AuthSettings` in `auth.pkl`:

```pkl
settings: auth.AuthSettings = new {
  password_login_enabled = true
  passkey_login_enabled  = true

  // WebAuthn relying party (required for passkeys)
  rp_id          = "home.example.com"
  rp_display_name = "My Home"
  rp_origins     = List("https://home.example.com")

  // Session TTLs
  access_cookie_ttl  = 15.min
  refresh_cookie_ttl = 30.d

  // Throttle (per source IP × auth method)
  failed_attempts_threshold = 10
  failed_attempts_window    = 10.min
  failed_attempts_block     = 15.min
}
```

## Bootstrap flow

A fresh install has Pkl-declared users but no credentials. To add the first credential:

1. Run `switchyard config apply` to project users into the runtime database.
2. Run `switchyard auth bootstrap <slug>` over the local Unix socket. This mints a one-time enrollment token and prints it once.
3. Open the web UI, paste the token, and complete the passkey registration or password setup.

See `switchyard auth --help` for the full CLI surface: `login`, `logout`, `whoami`, `tokens`, `users`, `passkeys`, `set-password`, `explain`, `policies`.
