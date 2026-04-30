# C9 — Auth & Policy Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-25
**Status:** Draft
**Depends on:** C1 (Event Core), C4 (Pkl Config), C7 (Connect-RPC API), C8 (MCP Server)
**Closes:** C7's `AuthService` `UNIMPLEMENTED` stubs and the C7 auth seam (`LocalPeerCredAuthenticator` chained to `RejectAllAuthenticator`, `AllowAllAuthorizer`); C8's deferred items — MCP HTTP transport, MCP-scoped token issuance, per-tool / per-resource policy, real `principal_id` in MCP audit events.

---

## Table of Contents

1. [Scope](#1-scope)
2. [Background](#2-background)
3. [Architecture Overview](#3-architecture-overview)
4. [Identity & Users](#4-identity--users)
5. [Auth Methods & Sessions](#5-auth-methods--sessions)
6. [Policy: Pkl Schema, Compiler, Runtime](#6-policy-pkl-schema-compiler-runtime)
7. [Enforcement Points](#7-enforcement-points)
8. [Audit Events](#8-audit-events)
9. [CLI Surface](#9-cli-surface)
10. [Configuration](#10-configuration)
11. [Testing Strategy](#11-testing-strategy)
12. [Implementation Order](#12-implementation-order)
13. [Decision Record](#13-decision-record)
14. [Explicit Deferrals](#14-explicit-deferrals)

---

## 1. Scope

C9 makes the gohome daemon multi-user, policy-governed, and remote-agent-accessible by completing the auth seam C7 stubbed and the MCP scaffolding C8 deferred. After C9, every Connect-RPC and MCP call resolves to a real principal (or `system:local`), is gated by Pkl-declared policy, and lands in the audit log; agents can be issued narrowly-scoped tokens; the MCP server speaks Streamable HTTP with bearer-token auth in addition to its C8-shipped stdio transport.

### 1.1 In scope

- **Identity:** Pkl-declared `User` (slug, display name, role assignments, active flag) projected into a `auth_users` registry table; credentials live in registry-side tables that never touch Pkl.
- **Auth methods:** Passkey (WebAuthn) primary; password (Argon2id) fallback that can be globally disabled in Pkl; long-lived API tokens (user-bound, scope-narrowed).
- **Sessions:** HTTP-only / `Secure` / `SameSite=Strict` cookies; short access TTL with HMAC-signed claims; refresh cookie with server-side rotation and replay detection.
- **Roles:** Pkl-declared with single-inheritance composition. Built-in `admin`, `member`, `guest` plus operator-defined custom roles.
- **Policies:** Pkl-declared `allow + deny` rules; deny wins; default-deny when no allow matches. Hierarchical area matching, class matching, and literal entity-id matching in the selector grammar.
- **Policy compiler & runtime:** Pkl `PolicyConfig` compiled on every successful `ConfigApplied` into an artifact with a precomputed `(role × verb) → action allowlist` (fast O(1) reject) and per-role rule lists for runtime selector evaluation; atomic generation swap.
- **Enforcement:** the C7 stub `Authenticator` and `Authorizer` swapped for real implementations. Connect interceptors wired with the real chain; subscription streams honor a `policy_mode` knob (`filter` / `strict`); MCP tool/resource dispatch goes through the seam C8 already laid.
- **Tokens:** `gohome auth tokens create --user <slug> --label <text> [--scope mcp] [--allow-tool ...] [--allow-area ...] ...` mints user-bound bearer tokens with explicit scope; effective permission is `user.policy ∩ token.scope`.
- **MCP HTTP transport:** Streamable HTTP at `/mcp` on the C7 listener, bearer-token auth via `Authorization: Bearer <gohome-api-token>`. Same `internal/mcp` package as C8's stdio.
- **`AuthService` implementation:** the C7-stubbed RPCs (`Login`, `Logout`, `CurrentUser`, `CreateToken`, `RevokeToken`, `ListUsers`, `RegisterPasskey`, `StartWebAuthnChallenge`) all light up; new RPCs `Refresh`, `MintEnrollmentToken`, `RedeemEnrollmentToken`, `ChangePassword`, `ExplainAuthorization`.
- **Bootstrap CLI:** `gohome auth bootstrap <user-slug>` mints a one-time enrollment token over UDS for an already-Pkl-declared user.
- **Audit:** new `AuthEvent` payload kinds (login success/failure, session refresh / replay, passkey register / unregister, token mint / revoke / reject, policy denied, policy compiled).
- **`system:local` semantics:** UDS callers (`Principal.Kind == "system"`) bypass `Authorize` entirely; audit still records `principal_id = "system:local"`.
- **CLI surface:** `gohome auth login`, `whoami`, `logout`, `tokens {create,list,revoke}`, `users list`, `passkeys {list,remove}`, `set-password`, `hash-password`, `explain`, `policies {list,inspect}`, `bootstrap`, `rotate-cookie-key`.
- **Metrics:** `gohome_auth_*` and `gohome_policy_*` series under the existing C7 Prometheus registry.

### 1.2 Explicit non-goals

- **OIDC.** Deferred to v1.x. The Pkl `User` shape reserves an `oidc_subject` field for forward compatibility; no runtime path in v1.0.
- **TLS lifecycle** (ACME, cert rotation, self-signed generation, mTLS). Owned by C13. C9 consumes whatever C7 already accepts via `Listener.TLSConfig`.
- **OAuth 2.1 MCP transport.** Deferred. Bearer tokens cover v1.
- **Two-factor auth beyond passkeys.** Passkeys are inherently multi-factor; layered TOTP not in v1.
- **Per-policy time windows** ("kids can turn off lights only after 7pm"). Expressed via Starlark in C6 automations, not in policy. Deferred.
- **Group sync from external IdPs.** Bound up with OIDC, deferred together.
- **Account lockout policy beyond simple per-IP throttle.** A homelab fat-finger should not lock a household member out of their own home; ops-tooling concern.
- **Browser-side WebAuthn UI.** C10 owns the React flows. C9 ships the server side; integration tests use `go-webauthn`'s virtual authenticator.
- **Strict-mode enforcement on non-streaming Connect RPCs.** `policy_mode` only applies to subscriptions; point-target RPCs are either permitted or denied — there is no "filter" to apply.

---

## 2. Background

Master design §7.4 names users, roles, passkey-first auth with password fallback and OIDC opt-in, scoped API tokens, Pkl-declared policies, and four enforcement points (Authenticate, Authorize, subscription filtering, MCP token attribution). C7 shipped the seam: a `Principal` / `Authenticator` / `Authorizer` interface set, a `LocalPeerCredAuthenticator` granting `system:local` on UDS, a `RejectAllAuthenticator` on TCP, an `AllowAllAuthorizer` stub, per-method action catalogs on every service, and `AuthService` RPCs stubbed `UNIMPLEMENTED`. C8 built the MCP server on top of that seam, with every tool dispatch and resource subscription already calling `auth.Authorize` — the calls are no-ops today because the authorizer is `AllowAll`. C8 deferred to C9: HTTP transport, MCP-scoped token issuance, per-tool / per-resource policy, and real `principal_id` in audit events.

C9 closes all of those threads. Concretely it:

- replaces the `LocalPeerCredAuthenticator → RejectAllAuthenticator` chain with `LocalPeerCredAuthenticator → BearerToken → SessionCookie → RejectAllAuthenticator`;
- replaces `AllowAllAuthorizer` with the policy runtime built in §6;
- implements `AuthService`'s eight C7-stubbed RPCs and adds five more (`Refresh`, `MintEnrollmentToken`, `RedeemEnrollmentToken`, `ChangePassword`, `ExplainAuthorization`);
- adds the `/mcp` Streamable HTTP route to the C7 listener;
- ships the `gohome auth *` CLI tree.

OIDC was scoped out of C9 (per Q1 of the brainstorm) to keep this spec focused; the `oidc_subject` field in the Pkl `User` shape and the chain-position in `internal/auth/authn` are reserved so a v1.x OIDC subspec slots in without disturbing the v1.0 surface.

---

## 3. Architecture Overview

### 3.1 Process and component map

```
                                  ┌─────────────────────────┐
   gohome (CLI) ─── UDS / TCP ─►  │       gohomed           │
   Browser     ─── TCP+TLS ─►     │                         │
   MCP client  ─── stdio ─►       │  ┌───────────────────┐  │
   MCP client  ─── TCP+TLS ─►     │  │ listener mux      │  │
                                  │  │  /healthz         │  │
                                  │  │  /webhooks/{slug} │  │
                                  │  │  /mcp     ◄── new │  │
                                  │  │  Connect handlers │  │
                                  │  └────────┬──────────┘  │
                                  │           │             │
                                  │  ┌────────▼──────────┐  │
                                  │  │ interceptor stack │  │
                                  │  │  authenticate     │  │  ◄── chain (UDS-peercred, bearer, cookie, reject)
                                  │  │  authorize        │  │  ◄── policy-backed (replaces AllowAll)
                                  │  └────────┬──────────┘  │
                                  │           │             │
                                  │  ┌────────▼──────────┐  │
                                  │  │ services (C7)     │  │
                                  │  │  + AuthService    │  │  ◄── now implemented
                                  │  │  + MCP dispatch   │  │  ◄── policy live
                                  │  └────────┬──────────┘  │
                                  │           │             │
                                  │  ┌────────▼──────────┐  │
                                  │  │ internal/policy   │  │  ◄── new
                                  │  │  compiler/runtime │  │
                                  │  └───────────────────┘  │
                                  │  ┌───────────────────┐  │
                                  │  │ internal/auth     │  │  ◄── grown from C7 stub
                                  │  │  identity store   │  │
                                  │  │  webauthn         │  │
                                  │  │  password         │  │
                                  │  │  tokens           │  │
                                  │  │  sessions         │  │
                                  │  └───────────────────┘  │
                                  └─────────────────────────┘
                                          │
                                          ▼ events: AuthEvent, ConfigApplied(policy)
                                       eventstore (C1)
```

### 3.2 Package map

```
gohome/
├── internal/auth/                    ← grown from C7 stub
│   ├── auth.go                       ← (existing) Principal/Authenticator/Authorizer/Action/Target
│   ├── local.go                      ← (existing) UDS peer-cred → system:local
│   ├── reject.go                     ← (existing) RejectAll
│   ├── chain.go                      ← (existing) Chain combinator
│   ├── identity/
│   │   ├── store.go                  ← user lookup, role assignments, active flag, projection from ConfigApplied
│   │   └── store_test.go
│   ├── credentials/
│   │   ├── password.go               ← Argon2id hash/verify, disable-globally check
│   │   ├── webauthn.go               ← go-webauthn integration, RP config, attestation
│   │   ├── tokens.go                 ← issue, hash-at-rest, lookup, revoke, intersect-scope
│   │   ├── enrollment.go             ← bootstrap one-time tokens
│   │   └── *_test.go
│   ├── sessions/
│   │   ├── store.go                  ← server-side session + refresh table
│   │   ├── cookies.go                ← cookie marshal/unmarshal, HMAC, attribute policy
│   │   └── *_test.go
│   ├── authn/
│   │   ├── chain.go                  ← combines local-peercred + bearer + cookie + reject
│   │   ├── bearer.go                 ← Authorization: Bearer <token> → Principal
│   │   ├── cookie.go                 ← session cookie → Principal
│   │   └── *_test.go
│   ├── audit/
│   │   └── recorder.go               ← emits AuthEvent payloads
│   └── throttle/
│       ├── throttle.go               ← per-IP × per-method failed-attempt counter
│       └── throttle_test.go
├── internal/policy/                  ← new
│   ├── schema.go                     ← Compiled, CompiledRule, CompiledSelector types
│   ├── compiler.go                   ← Pkl PolicyConfig → Compiled
│   ├── runtime.go                    ← Authorize, FilterEntities, OnReload
│   ├── selector.go                   ← EntitySelector eval, hierarchical area expansion
│   ├── intersect.go                  ← user-policy ∩ token-scope
│   ├── explain.go                    ← AuthService.ExplainAuthorization backing
│   └── *_test.go
├── internal/api/
│   ├── service_auth.go               ← AuthService impl (replaces UNIMPLEMENTED stub)
│   ├── interceptor_authn.go          ← real authenticator chain wired in
│   └── interceptor_authz.go          ← policy-backed authorizer wired in
├── internal/mcp/
│   └── transport_http.go             ← Streamable HTTP transport, mounted at /mcp
├── internal/cli/
│   ├── cmd_auth.go                   ← `gohome auth login/logout/whoami/tokens/users/passkeys/...`
│   ├── cmd_auth_bootstrap.go         ← `gohome auth bootstrap`
│   ├── cmd_auth_explain.go           ← `gohome auth explain`
│   ├── cmd_auth_policies.go          ← `gohome auth policies list/inspect`
│   └── styles_auth.go                ← lipgloss styles unique to the auth CLI
└── proto/gohome/event/v1/
    └── auth_event.proto              ← new payloads
```

`internal/policy` knows nothing about HTTP, MCP, or the registry — pure compilation + decision functions. `internal/auth/credentials/*` knows nothing about HTTP — accepts password strings / WebAuthn payloads / token bytes, returns Principals or errors. `internal/auth/authn/*` is the HTTP-aware glue. `internal/api/interceptor_*` is the only place these wire into Connect.

---

## 4. Identity & Users

### 4.1 Pkl `User` shape

`internal/config/pkl/gohome/auth.pkl` (new):

```pkl
class User {
  // Identity (Pkl-owned, git-tracked)
  slug:         String(matches(Regex(#"^[a-z][a-z0-9_-]{1,31}$"#)))
  display_name: String(length > 0)
  roles:        List<Role>
  active:       Boolean = true

  // Auth-method posture (Pkl-owned: which methods this user MAY use)
  // Actual credentials are in the registry projection.
  password_allowed: Boolean = true
  passkey_allowed:  Boolean = true
  oidc_subject:     String? = null         // RESERVED for v1.x OIDC; ignored in v1.0

  // Bootstrap convenience: a hash the operator pre-staged via `gohome auth hash-password`.
  // The first successful login copies this into the registry credential row, then
  // the field is ignored on subsequent reloads. Removing it from Pkl does NOT
  // revoke the password — that goes through the credentials store.
  bootstrap_password_hash: String? = null
}

class Role {
  slug:         String(matches(Regex(#"^[a-z][a-z0-9_-]{1,31}$"#)))
  display_name: String(length > 0)
  inherits:     List<Role> = List()        // role composition; cycle-checked at compile
}
```

`AuthSettings` (also in `auth.pkl`) holds the global knobs — full schema in §10.

A user's effective role set is the transitive closure of `roles` over `Role.inherits`. Cycles fail `gohome config validate` with a rich error pointing at the offending role.

### 4.2 Identity ownership split

Two stores:

- **Pkl** owns *who exists*: slug, display name, role assignments, active flag, auth-method posture flags. This is git-tracked and human-edited.
- **Registry** owns *what credentials exist*: password hashes, passkey credentials, sessions, tokens, enrollment tokens. None of this is git-tracked or human-edited; binary blobs and rotating secrets that change with every device re-enrollment shouldn't churn the config.

The `bootstrap_password_hash` field is the one bridge: an operator can pre-stage a password hash in Pkl, and the first successful login copies it into the registry. After that the field is ignored. Removing it from Pkl does **not** revoke the password (which lives in the registry); use `gohome auth set-password` or a `RevokePassword` admin call.

### 4.3 Registry credential tables

Schema (lives alongside the C7 tables; same SQLite database file):

```sql
CREATE TABLE auth_users (
  slug             TEXT PRIMARY KEY,
  display_name     TEXT NOT NULL,
  active           INTEGER NOT NULL,
  password_allowed INTEGER NOT NULL,
  passkey_allowed  INTEGER NOT NULL
);

CREATE TABLE auth_user_roles (
  user_slug TEXT NOT NULL,
  role_slug TEXT NOT NULL,
  PRIMARY KEY (user_slug, role_slug)
);

CREATE TABLE auth_passwords (
  user_slug      TEXT PRIMARY KEY,
  argon2id_hash  TEXT NOT NULL,            -- encoded form (parameters embedded)
  set_at         INTEGER NOT NULL,
  set_by         TEXT NOT NULL             -- "user:fdatoo", "system:bootstrap", "admin:foo"
);

CREATE TABLE auth_passkeys (
  credential_id BLOB    PRIMARY KEY,
  user_slug     TEXT    NOT NULL,
  public_key    BLOB    NOT NULL,
  sign_count    INTEGER NOT NULL,
  attestation   BLOB,
  label         TEXT,                      -- "fdatoo's iPhone"
  registered_at INTEGER NOT NULL,
  last_used_at  INTEGER
);

CREATE TABLE auth_tokens (
  token_id      TEXT PRIMARY KEY,          -- ULID, also the prefix segment of the bearer string
  user_slug     TEXT NOT NULL,
  hash_b64      TEXT NOT NULL,             -- sha256(secret); secret never stored
  scope_blob    BLOB NOT NULL,             -- protobuf-encoded TokenScope (tools + selector)
  label         TEXT NOT NULL,
  issued_at     INTEGER NOT NULL,
  issued_by     TEXT NOT NULL,             -- principal_id of issuer
  expires_at    INTEGER,                   -- nullable
  revoked_at    INTEGER,
  last_used_at  INTEGER
);

CREATE TABLE auth_sessions (
  session_id      TEXT PRIMARY KEY,        -- ULID
  user_slug       TEXT NOT NULL,
  refresh_hash    TEXT NOT NULL,           -- sha256(refresh secret)
  issued_at       INTEGER NOT NULL,
  refresh_ttl_at  INTEGER NOT NULL,        -- absolute expiry
  refresh_idle_at INTEGER NOT NULL,        -- bumped on each use
  user_agent      TEXT,
  remote_ip       TEXT
);

CREATE TABLE auth_enrollment_tokens (
  token_hash    TEXT PRIMARY KEY,          -- sha256(token); plaintext shown to operator once
  user_slug     TEXT NOT NULL,
  intent        TEXT NOT NULL,             -- "register_passkey" | "set_password"
  expires_at    INTEGER NOT NULL,
  consumed_at   INTEGER
);

CREATE TABLE auth_attempts (
  bucket       TEXT NOT NULL,              -- "<ip>|<method>", e.g. "10.0.0.1|password"
  attempted_at INTEGER NOT NULL,
  succeeded    INTEGER NOT NULL
);
CREATE INDEX auth_attempts_bucket ON auth_attempts(bucket, attempted_at);
```

**Population rules:**

- `auth_users` and `auth_user_roles`: rebuilt from the latest `ConfigApplied` event by the identity-store projector. Same projection mechanism C7 uses for the entity registry.
- `auth_passwords`, `auth_passkeys`, `auth_tokens`, `auth_sessions`, `auth_enrollment_tokens`: populated by replaying `AuthEvent`s (§8).
- `auth_attempts`: ephemeral; bucketed in SQLite for survivability across daemon restarts but not derived from events. The recorder vacuums rows past `failed_attempts_window`.

### 4.4 Bootstrap flow

A fresh install has Pkl-declared users but zero credentials. The operator's first-login experience:

1. Operator writes their user into `auth/users.pkl`, runs `gohome config apply`. `ConfigApplied` event flows; the identity-store projector populates `auth_users`. User exists; no credentials yet.
2. Operator runs `gohome auth bootstrap fdatoo --intent register_passkey` over UDS. The CLI is `system:local` (§7.1); the `AuthService.MintEnrollmentToken` handler:
   - Verifies `auth_users.fdatoo` exists.
   - Mints a 128-bit random token, stores `sha256(token)` in `auth_enrollment_tokens` with `expires_at = now + 1h` (configurable via `--ttl`).
   - Returns the plaintext token to the CLI, which prints it once.
3. Operator opens the web UI in a browser (over HTTPS — see §1 deferrals re: TLS). UI shows an "Enrollment" page asking for the token + a passkey registration.
4. UI calls `AuthService.RedeemEnrollmentToken(token)` → server validates, returns a short-lived signed challenge.
5. UI calls `AuthService.StartWebAuthnChallenge(intent: "register")` and `AuthService.RegisterPasskey(credential, label)` in the standard WebAuthn dance (§5.1). On success, an `AuthEvent{kind: passkey_registered}` lands on the log; `auth_passkeys` populates; the enrollment token is consumed.
6. Operator immediately redirected to a normal login flow with the just-registered passkey.

`--intent set_password` is the password-fallback variant: the redemption page asks for a new password instead of a passkey. Useful for headless setups where a browser-WebAuthn flow is impractical, or for environments where passkeys are disabled globally.

---

## 5. Auth Methods & Sessions

### 5.1 Passkeys (WebAuthn) — primary

**Library:** `github.com/go-webauthn/webauthn` (the canonical Go implementation; battle-tested, used by Authelia and similar projects). It wraps the FIDO2/WebAuthn ceremonies; we own the storage adapter and the `AuthService` glue.

**Relying party config** comes from `AuthSettings` (§10): `rp_id`, `rp_display_name`, `rp_origins`, `webauthn_user_verification`. C9 enforces the WebAuthn spec rule that `rp_id` is a registrable suffix of every origin; mismatch fails `gohome config validate`.

**Registration ceremony** (post-enrollment-token redemption, or by an already-logged-in user adding a second device):

1. Client → `AuthService.StartWebAuthnChallenge(intent: "register", user_slug)` — server returns a `PublicKeyCredentialCreationOptions` with a fresh challenge. The challenge is stored in the session (or under the enrollment token's scratch row) — never round-tripped through the client unsigned.
2. Browser performs `navigator.credentials.create({...})`.
3. Client → `AuthService.RegisterPasskey(credential, label)` — server validates attestation against the stored challenge, persists `auth_passkeys` row, emits `AuthEvent{kind: passkey_registered}`.

**Authentication ceremony:**

1. Client → `AuthService.StartWebAuthnChallenge(intent: "login")` — returns `PublicKeyCredentialRequestOptions` with `allowCredentials = []` (we use **discoverable / resident keys** so the user picks the credential — better UX, no username enumeration during the prompt).
2. `navigator.credentials.get({...})`.
3. Client → `AuthService.Login(passkey_assertion)` — server resolves credential → user, validates assertion (signature, sign-count monotonic, RP hash, user-verification flag), bumps `sign_count`, mints a session (§5.4), emits `AuthEvent{kind: login_succeeded, auth_method: "passkey"}`.

**Multi-credential per user** is the default — the credential table is keyed by `credential_id`, and a user can have any number of devices registered. UI exposes "Manage devices" listing labels + `last_used_at`, with a remove button. Removing a credential emits `AuthEvent{kind: passkey_unregistered}`.

**`sign_count` discipline:** on every assertion we require `assertion.sign_count > stored.sign_count` unless either is zero (some authenticators don't track it). Counter regression triggers a `LoginFailed` event with reason `sign_count_regression` and rejects — the standard cloned-authenticator detection. We do not auto-disable the credential; we flag it in `gohome auth passkeys list` with a warning so the operator can investigate and revoke manually.

### 5.2 Password — fallback

**Hashing:** Argon2id via `golang.org/x/crypto/argon2`. Default parameters are tuned in `AuthSettings`: `argon2id_time = 3`, `argon2id_memory_kib = 65536` (64 MiB), `argon2id_parallelism = 4`. The encoded hash carries its parameters, so a future tuning change verifies old hashes correctly and triggers a silent re-hash on next successful login.

**Storage:** `auth_passwords` row per user. Plaintext touches the daemon only inside `internal/auth/credentials/password.go`; the function signatures are `Set(ctx, user, plaintext) error` and `Verify(ctx, user, plaintext) (ok bool, needs_rehash bool, err error)`. No plaintext crosses package boundaries.

**Login flow:**

1. Client → `AuthService.Login{password: {user_slug, plaintext}}`.
2. Throttle check (§5.5).
3. Lookup `auth_users.<slug>`; if missing or `active = 0` or `password_allowed = 0` or global `password_login_enabled = false`, return `unauthenticated` with reason `password_not_available` and emit `LoginFailed`. **Same response time and shape as "wrong password"** — no user-enumeration oracle.
4. Lookup `auth_passwords.<slug>`; verify hash. On success, mint session (§5.4), emit `LoginSucceeded`. On failure, emit `LoginFailed` with reason `bad_credentials`.

**Disable globally** (`AuthSettings.password_login_enabled = false`): password login fails for everyone, immediately, regardless of `password_allowed` per-user. Hashes stay in the registry (operator can re-enable). Designed so a homelab that wants passkey-only can flip one switch without losing recovery state.

**Password change:** `AuthService.ChangePassword(old, new)` — verifies old, sets new. Bootstrap-stage `bootstrap_password_hash` from Pkl is treated as the initial hash on first observation, then ignored on subsequent reloads (the `set_at` column carries the discipline: if `auth_passwords.<slug>` exists, Pkl bootstrap hash is ignored; if it doesn't and `bootstrap_password_hash` is set, a row is created with `set_by = "system:bootstrap"`).

### 5.3 API tokens

**Token format:** `gohome_<token_id_ulid>_<secret_b32>`. The `gohome_` prefix is the public-leak detector signal (so secret-scanners pick them up); `<token_id>` is the lookup key (used to find the row and check expiry / revocation in O(1)); `<secret>` is the part we actually verify (sha256 + constant-time compare against `hash_b64`). Plaintext is shown to the operator exactly once at mint time.

**Token scope** (Pkl mirror; on the wire it's a protobuf):

```proto
message TokenScope {
  // Tool / RPC allowlist. Empty list = "no restriction beyond user policy."
  repeated string allow_tools    = 1;   // "gohome__get_state"; supports trailing wildcard "gohome__*"
  repeated string allow_services = 2;   // "EntityService.List"; supports trailing wildcard "EntityService.*"

  // Entity narrowing
  EntitySelector allow_targets   = 3;   // empty = no narrowing beyond user policy
}
```

**Issuance:**

```
gohome auth tokens create \
    --user fdatoo \
    --label "claude-desktop" \
    --ttl 90d \
    --scope mcp \                       # shorthand: allow_tools = ["gohome__*"]
    --allow-tool gohome__get_state \    # explicit override; replaces shorthand
    --allow-area kitchen \
    --allow-class Light
```

- `--scope mcp` is a templating shortcut. Without `--scope`, the token gets an empty (= no narrowing) scope.
- `--allow-tool` and `--allow-service` accept trailing `*` for grouping.
- `--allow-area`, `--allow-class`, `--allow-entity` populate `allow_targets`.
- The mint requires the issuer to be authenticated — `system:local` (CLI on UDS) or any user with the `admin` verb on `AuthService.CreateToken`. **Self-mint** of equal-or-narrower scope is allowed; mint with broader scope than the issuer requires `admin` (an admin can mint a token for another user; a member can only mint a narrower token for themselves).
- **Effective permission rule:** at request time, `permitted = user.policy.allow(action, target) AND token.scope.allow(action, target) AND not user.policy.deny(action, target)`. Token scope is a pure intersection — it cannot widen, it cannot override deny.

**Listing / revocation:** `gohome auth tokens list`, `gohome auth tokens revoke <id>`. Revocation flips `revoked_at`; the bearer-token authenticator checks the column on every request (a single indexed lookup).

**Audit:** `AuthEvent{kind: token_minted, …}` at mint time; `AuthEvent{kind: token_used, …}` is **not** emitted (would be one event per RPC; lives in the standard request-id slog instead); `AuthEvent{kind: token_revoked, …}` at revocation; `AuthEvent{kind: token_rejected, …}` on a rejected presentation.

### 5.4 Sessions (cookie + refresh)

**Cookies:**

- `gohome_access` — short-lived (default 15 min). Carries an HMAC-signed claim `{session_id, user_slug, exp}` — the server validates without a DB read on the hot path.
- `gohome_refresh` — long-lived (default 30 d absolute, 14 d idle). Carries `{session_id, refresh_secret}`; server hashes the secret and looks up `auth_sessions.refresh_hash` for validation.
- Both: `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`. (`SameSite=Strict` rules out cross-site embeds, fine for a self-hosted dashboard.)

**HMAC signing key:** generated on first start, persisted under the daemon's data dir (`<data_dir>/auth/cookie.key`, mode 0600). Rotated by `gohome auth rotate-cookie-key` (existing sessions become invalid; everyone re-logs in). The key is **not** in Pkl — pattern matches how C7 handles TLS cert paths (operator-supplied file, not Pkl-stored bytes).

**Refresh rotation:**

- `AuthService.Refresh()` (called by the browser when the access cookie expires) consumes the refresh cookie, verifies hash + idle/absolute deadlines, **rotates the refresh secret** (new secret stored, old hash overwritten), bumps `refresh_idle_at`, returns a new pair of cookies.
- **Rotation invariant:** if a refresh cookie is presented twice (replay attack — attacker stole the cookie, victim refreshed first, attacker presents stale one), the second presentation finds a non-matching hash and the server **revokes the entire session** (deletes the row, emits `AuthEvent{kind: session_replay_detected}`). Forces re-login and surfaces the compromise in the audit log.

**Logout:** `AuthService.Logout()` deletes the `auth_sessions` row, clears both cookies (`max-age=0`). Emits `AuthEvent{kind: logout}`.

**Sessions vs tokens:** distinct code paths, distinct tables. A session is a UI-bound credential with rotation; a token is an API-bound credential with explicit scope. Both produce a `Principal` from the same `Authenticator` chain (cookie authenticator and bearer authenticator are siblings in the chain).

### 5.5 Failed-auth throttle

Per `AuthSettings.failed_attempts_*`:

- `auth_attempts` row inserted on every login attempt with `(bucket = "<ip>|<auth_method>", attempted_at, succeeded)`.
- Pre-attempt check: count rows in `failed_attempts_window`; if count of failures ≥ `failed_attempts_threshold`, return `unauthenticated` with reason `throttled` for `failed_attempts_block` regardless of credential validity. Emit `AuthEvent{kind: login_failed, reason: "throttled"}`.
- Successful login does **not** clear the attempt counter — it just records `succeeded = 1` in the row. The window slides naturally; this prevents an attacker who guesses correctly from immediately resetting the throttle for further guesses.
- Behind a reverse proxy, the interceptor uses the leftmost untrusted hop in `X-Forwarded-For` per `Listener.WebhookConfig.trusted_proxies` (already C7 territory).

This is intentionally a soft throttle, not an account lockout: lockouts on a homelab cause more pain than they prevent (someone fat-fingering a household member's password from the kitchen tablet should not lock that household member out of their own home). A hard lockout policy is out of scope for v1.

---

## 6. Policy: Pkl Schema, Compiler, Runtime

### 6.1 Pkl `Policy` schema

`internal/config/pkl/gohome/policy.pkl` (new):

```pkl
module gohome.policy
import "gohome/auth.pkl" as auth

// Closed enum: every Authorize call's Action.Verb is one of these.
typealias Verb = "read" | "call" | "write" | "admin"

class EntitySelector {
  // All three lists are OR within a list, AND across lists.
  // Empty selector matches NOTHING (default-deny safe). Use the constant
  // `gohome.policy.AnyEntity` to mean "match all entities."
  areas:       List<String> = List()       // area slugs; hierarchical — parent matches descendants
  classes:     List<String> = List()       // entity class names: "Light", "Lock", ...
  entity_ids:  List<String> = List()       // literal entity IDs: "lock.front_door"
}

const AnyEntity: EntitySelector = new {
  areas      = List("*")
  classes    = List("*")
  entity_ids = List("*")
}

class CapabilityRule {
  // Empty verbs list means "all verbs."
  verbs:    List<Verb>      = List()
  targets:  EntitySelector  = new EntitySelector {}   // empty ⇒ matches nothing; use AnyEntity for "all"

  // For non-entity actions (config admin, token mint, …), targets is ignored
  // and the rule matches purely on verb. The compiler checks that any rule
  // referencing a non-entity action has empty `targets` to keep authoring honest.
  services: List<String>    = List()       // optional narrowing to specific services, e.g. List("EntityService")
}

class Policy {
  name:     String                          // "kids_bedrooms_only"
  subjects: List<auth.Role>                 // roles this policy applies to
  allow:    List<CapabilityRule> = List()
  deny:     List<CapabilityRule> = List()
}
```

A user's effective rule set = union of `allow`/`deny` from all policies whose `subjects` overlap their role set (transitively, via `Role.inherits`).

**Evaluation:** `permitted iff (any allow rule matches) AND (no deny rule matches)`. Default-deny when no allow matches.

### 6.2 Worked Pkl example

```pkl
import "gohome/auth.pkl"   as auth
import "gohome/policy.pkl" as policy

policies = List(
  // Admin sees everything
  new policy.Policy {
    name = "admin_full"
    subjects = List(roles.admin)
    allow = List(new policy.CapabilityRule { targets = policy.AnyEntity })
  },

  // Members can read/call anything but cannot admin
  new policy.Policy {
    name = "member_baseline"
    subjects = List(roles.member)
    allow = List(new policy.CapabilityRule {
      verbs   = List("read", "call")
      targets = policy.AnyEntity
    })
  },

  // Kids can read/call only on their bedrooms — but never the front door lock or alarm
  new policy.Policy {
    name = "kids_bedrooms_only"
    subjects = List(roles.kids)
    allow = List(new policy.CapabilityRule {
      verbs   = List("read", "call")
      targets = new policy.EntitySelector {
        areas = List("kids_floor")            // hierarchical — implicitly covers nora_room, milo_room
      }
    })
    deny = List(new policy.CapabilityRule {
      verbs   = List("call")
      targets = new policy.EntitySelector {
        classes = List("Lock", "Alarm")
      }
    })
  },
)
```

### 6.3 Compiler

`internal/policy/compiler.go` runs whenever `ConfigApplied` lands (i.e., on startup and on every `gohome config apply`). Inputs:

- The freshly-evaluated `PolicyConfig` from Pkl.
- The fully-loaded `RoleGraph` (with inheritance edges resolved, cycle-checked).
- The fully-loaded `AreaTree` from C4 — needed because hierarchical area matching pre-computes ancestor sets.

**Outputs (the compiled artifact):**

```go
// internal/policy/schema.go
type Compiled struct {
    // Per (role, verb): the action set the role is even allowed to attempt,
    // intersected with the action catalog. Precomputed allowlist for the
    // hot-path fast reject.
    //
    // ActionKey is "ServiceName.Method" — e.g. "EntityService.CallCapability".
    // Wildcard entries `("EntityService.*", verb)` are expanded at compile time
    // into one row per concrete RPC by reading the action catalog.
    ActionAllowlist map[RoleVerb]map[ActionKey]struct{}

    // Per role: the rule lists the runtime walks to evaluate the target
    // selector (only consulted when ActionAllowlist already permitted the action).
    AllowRules map[RoleSlug][]CompiledRule
    DenyRules  map[RoleSlug][]CompiledRule

    // Pre-expanded area set per selector — replaces the hierarchical walk at
    // request time with a flat "area_slug ∈ set" check. Built once per
    // (compile generation, selector); cached by selector structural hash so
    // identical selectors share the set.
    AreaExpansion map[SelectorHash]map[AreaSlug]struct{}

    // Generation counter; bumped on every successful compile. The runtime
    // uses an atomic.Pointer swap so in-flight requests see a consistent
    // generation through to completion.
    Generation uint64
}

type CompiledRule struct {
    PolicyName string                          // for explain output
    Verbs      map[Verb]struct{}                // empty ⇒ all verbs
    Services   map[string]struct{}              // empty ⇒ all services
    Targets    CompiledSelector
}

type CompiledSelector struct {
    Hash      SelectorHash
    AreaSet   map[AreaSlug]struct{}            // post-expansion; empty ⇒ no area constraint
    ClassSet  map[string]struct{}              // empty ⇒ no class constraint
    EntitySet map[string]struct{}              // empty ⇒ no entity_id constraint
    MatchAny  bool                             // shortcut for AnyEntity
}
```

**Compile steps:**

1. Resolve role inheritance. Build the transitive role-set per user.
2. For every Pkl `Policy`, build a `CompiledRule` per `allow`/`deny` entry. Compute selector hash; reuse cached expansion if seen before this generation.
3. Walk `EntitySelector.areas`: for each declared area, expand to the area + all descendants from `AreaTree`. Stuff the result into `AreaSet`.
4. Build `ActionAllowlist`. For each role, walk its allow rules; if a rule's `Verbs` is empty, expand to `{read, call, write, admin}`; if `Services` is empty, expand to all services in the action catalog. The result is a tight set of `(role, verb) → {Service.Method}` allowed actions.
5. Validate: every action referenced in `ActionAllowlist` must exist in the C7 action catalog (`internal/api/actions.go`). Unknown actions fail `gohome config validate` — protects against typos like `EntityService.Lit`.
6. Atomically swap the `Compiled` artifact behind an `atomic.Pointer[Compiled]`. Bump `Generation`. Old generation is held for one extra compile cycle so in-flight requests finish cleanly.

**Selector intersection (token scope ∩ user policy)** is computed at token-issue time, **not** compile time. Tokens carry a `CompiledSelector` they were minted with. At request time the runtime evaluates token scope and user policy as separate gates — both must say yes.

### 6.4 Runtime: `Authorize(ctx, principal, action, target)`

`internal/policy/runtime.go`:

```go
func (r *Runtime) Authorize(ctx context.Context, p auth.Principal, a auth.Action, t auth.Target) error {
    if p.Kind == "system" {            // §7.1: system principals bypass
        return nil
    }
    c := r.compiled.Load()             // current generation; non-blocking
    roles := r.roles.For(p)            // O(1) cache; populated by identity store
    actionKey := a.Service + "." + a.Method

    // Fast reject: action allowlist (precomputed)
    permittedAction := false
    for role := range roles {
        if c.permitsAction(role, a.Verb, actionKey) {
            permittedAction = true
            break
        }
    }
    if !permittedAction {
        return ErrForbidden{Reason: "action_denied"}
    }

    // Token scope check (if Principal carries one): same matcher, applied to scope.
    if scope := tokenScopeFromCtx(ctx); scope != nil {
        if !scope.PermitsAction(a.Verb, actionKey) {
            return ErrForbidden{Reason: "token_action_denied"}
        }
        if t.Kind == "entity" && !scope.PermitsTarget(t) {
            return ErrForbidden{Reason: "token_target_denied"}
        }
    }

    // Target rules: walk allow rules until one matches; then walk deny rules; deny wins.
    if t.Kind == "entity" {
        allowed := false
        for role := range roles {
            for _, rule := range c.AllowRules[role] {
                if rule.matches(a, t) { allowed = true; break }
            }
            if allowed { break }
        }
        if !allowed {
            return ErrForbidden{Reason: "target_denied"}
        }
        for role := range roles {
            for _, rule := range c.DenyRules[role] {
                if rule.matches(a, t) {
                    return ErrForbidden{Reason: "explicit_deny", RuleName: rule.PolicyName}
                }
            }
        }
    }

    return nil
}
```

`rule.matches(a, t)` checks `verbs ⊇ a.Verb`, `services ⊇ a.Service`, and target ∈ selector (`AreaSet ∋ t.Area || ClassSet ∋ t.Class || EntitySet ∋ t.ID`).

**Performance:** dominated by map lookups; rule walks are bounded by the user's role count × policies per role (single-digit in any realistic homelab). Hot-path is sub-microsecond. No lock acquisition (atomic.Pointer); compile happens off the request path.

### 6.5 `gohome auth explain`

A debug command with real value. `gohome auth explain --user nora --action EntityService.CallCapability --target entity:lock.front_door` returns:

```
Decision:    DENIED
Reason:      explicit_deny
Matching rule:
  Policy: kids_bedrooms_only
  Rule:   deny[0]
  Verbs:  ["call"]
  Targets: classes=["Lock", "Alarm"]
Trace:
  - principal nora → roles [kids, guest]
  - action_allowlist[(kids, call)] permits EntityService.CallCapability ✓
  - allow_rules[kids]: kids_bedrooms_only.allow[0] matches (target.area=front_entry ∈ kids_floor descendants) ✓
  - deny_rules[kids]: kids_bedrooms_only.deny[0] matches (target.class=Lock ∈ {Lock, Alarm}) ✗
```

Backed by `internal/policy/explain.go`, which runs the same matcher with verbose tracing. Exposed as a Connect RPC (`AuthService.ExplainAuthorization`) too, for the web UI to surface in a "why did this fail?" panel.

### 6.6 Subscription filtering reuses the runtime

When an `EntityService.Subscribe` (or MCP `resources/subscribe`) opens, the streaming layer asks the policy runtime: *given this principal and this list of candidate entities (from the selector), which entities can the principal `read`?*

```go
allowed, denied := runtime.FilterEntities(ctx, principal, "read", entities)
```

In `policy_mode = "filter"` (default), the stream emits only `allowed`. In `policy_mode = "strict"`, the call returns `PERMISSION_DENIED` if `denied` is non-empty.

`FilterEntities` is the same `Authorize` evaluator looped over a candidate list; no new policy primitive. It also re-runs on entity-registry changes mid-stream (a new entity arriving in the selected area gets gated on add).

### 6.7 Reload semantics

- `gohome config apply` produces the new `PolicyConfig`, compiler builds a new `Compiled` artifact, atomic swap, `Generation` bumped.
- In-flight RPCs see the old generation through to completion. New RPCs see the new generation immediately.
- Open subscription streams: the streaming layer subscribes to the policy runtime's `OnReload` channel. On reload, every active stream re-evaluates its filter; entities that newly become denied receive a synthetic "removed" event in the stream; entities that newly become allowed receive an initial-state event. Symmetric for traces.
- Tokens are not re-validated on reload — they were minted under their own scope. A token issued under permissive user policy that the policy now revokes still has its scope, but the user's now-tighter policy is the second gate (intersection rule); if the user lost a permission, their tokens lose it too on the next request.

---

## 7. Enforcement Points

### 7.1 Connect `authenticate` interceptor (real chain replaces C7 stub)

`internal/auth/authn/chain.go` builds the real chain. Order matters: each authenticator either returns a `Principal`, returns `ErrNotApplicable` (try next), or returns `ErrUnauthenticated` (stop with that error).

```go
authenticator := authn.Chain(
    authn.LocalPeerCred{},                       // UDS only — system:local
    authn.BearerToken{Tokens: ts},               // Authorization: Bearer gohome_<id>_<secret>
    authn.SessionCookie{Sessions: ss, Key: k},   // gohome_access cookie (HMAC-validated)
    authn.RejectAll{},                           // explicit terminal
)
```

**`LocalPeerCred`** (existing from C7): unchanged. Returns `Principal{Kind: "system", ID: "system:local"}` on UDS.

**`BearerToken`** (new):

1. Header parse: `Authorization: Bearer gohome_<token_id>_<secret>`. Malformed → `ErrNotApplicable` (lets a cookie still match).
2. Lookup `auth_tokens.<token_id>`. Missing / `revoked_at != null` / past `expires_at` → `ErrUnauthenticated` reason `token_invalid` + audit `AuthEvent{kind: token_rejected}`.
3. `subtle.ConstantTimeCompare(sha256(secret), hash_b64)` — fail → `ErrUnauthenticated` reason `token_invalid` (same shape as missing token; no enumeration).
4. Update `last_used_at` (best-effort, batched flush; not on the hot path).
5. Build `Principal{ID: "user:" + user_slug, Kind: "user", Metadata: {"token_id": ..., "auth_method": "token"}}`.
6. Stash decoded `TokenScope` on context (`auth.WithTokenScope(ctx, scope)`) for the authorize step.

**`SessionCookie`** (new):

1. Read `gohome_access` cookie. Missing / mangled → `ErrNotApplicable`.
2. Validate HMAC over the claim. Bad signature → `ErrUnauthenticated` reason `session_invalid` (do NOT silently fall through — a tampered cookie should fail loud).
3. Check `exp`. Expired → `ErrUnauthenticated` reason `session_expired` (the browser sees this and triggers `AuthService.Refresh`).
4. Optionally consult `auth_sessions.<session_id>` to confirm the session row still exists (rejects sessions that were logged out from another device). Cached for `min(remaining_access_ttl, 30s)` to keep the hot path off SQLite.
5. Build `Principal{ID: "user:" + user_slug, Kind: "user", Metadata: {"session_id": ..., "auth_method": "passkey" | "password"}}`.

**`RejectAll`** (existing): returns `ErrUnauthenticated` reason `unauthenticated`. Hits when nothing above matched and the request isn't on a bypass route.

**Bypass routes** (unchanged from C7): `/healthz`, `/webhooks/{slug}`, `AuthService.Login`, `AuthService.StartWebAuthnChallenge`, `AuthService.Refresh`, `AuthService.RedeemEnrollmentToken`. These never go through `authenticate`. Audit is still recorded with `principal_id = "anonymous"`.

**`system:local` semantics:** `Authorize` short-circuits when `Principal.Kind == "system"` — system principals skip policy entirely. Audit events still record `principal_id = "system:local"`. Rationale: anyone with shell access can edit Pkl, read SQLite, restart the daemon, so policy can't meaningfully constrain them; matching the existing prosumer-tool convention (Docker, systemctl, postgres-via-unix-socket all behave this way).

### 7.2 Connect `authorize` interceptor (real authorizer replaces `AllowAll`)

The `authorize` interceptor reads `Principal` + `TokenScope` (if any) from context, looks up `(action, target)` from C7's per-method action catalog, calls `policy.Runtime.Authorize`. Failure → `connect.CodePermissionDenied` + `ErrorDetail.reason = "forbidden"` (sub-reason from `Authorize`'s `ErrForbidden`).

**The action catalog already exists** — C7 built per-service tables and a target-extractor registry. C9 just swaps the authorizer instance. Zero changes to handlers.

**On allow, no audit.** Authorize is called O(many) per request; emitting an event each time would drown the log. Denials get `AuthEvent{kind: policy_denied, …}` — those are noteworthy and rare-by-design.

### 7.3 Streaming subscription filter (Connect + MCP)

**Connect subscriptions** (`EntityService.Subscribe`, etc.): the handler accepts `policy_mode` from the request, computes the initial entity set from the selector, calls `policy.Runtime.FilterEntities(ctx, principal, "read", candidates)`:

- `filter` mode: emit only allowed entities; deny ones are silently absent.
- `strict` mode: if `denied` is non-empty, return `PERMISSION_DENIED` reason `subscription_filtered` with metadata `{denied_count, sample_denied_ids}` — never narrow silently.

The handler also subscribes to `policy.Runtime.OnReload` and `entityRegistry.OnChange`. On either signal, re-evaluate; emit `entity_added` / `entity_removed` synthetic events to keep the client's view consistent.

**MCP subscriptions** (`resources/subscribe` per C8): `internal/mcp/resources/entities.go` decodes `policy_mode` from the resource URI's query string (default `filter`), passes it through to the underlying Connect `EntityService.Subscribe` call. The MCP layer doesn't need its own policy code — Connect already enforces.

### 7.4 MCP per-tool / per-resource dispatch (closes C8's hand-off)

C8 already wired `auth.Authorize(principal, action, target)` calls into the MCP tool dispatch loop and the resource subscription dispatch — they currently call `AllowAll`. C9's only MCP-side change is **swapping the authorizer instance** at construction time:

```go
// internal/cli/cmd_mcp.go (sketch)
mcpServer := mcp.NewServer(mcp.Deps{
    Authorizer: deps.PolicyRuntime,   // was: auth.AllowAll{}
    // everything else unchanged
})
```

The action catalog C8 built (`internal/mcp/actions.go`) feeds straight into the policy runtime. Per-tool target extractors (entity ID for `get_state`, `{Kind: "config", ID: input.Path}` for filesystem tools, etc.) are unchanged.

The `--scope mcp` token shorthand from §5.3 populates `TokenScope.allow_tools = ["gohome__*"]`. An admin issuing a fully-locked-down agent token writes:

```bash
gohome auth tokens create --user fdatoo --label "claude-readonly" \
    --allow-tool gohome__get_state \
    --allow-tool gohome__list_entities \
    --allow-tool gohome__query_events \
    --allow-area common_areas
```

That token can dispatch only those three tools; even though `fdatoo` is admin, the token's intersection narrows.

### 7.5 MCP HTTP transport (`/mcp`)

C9 adds a Streamable HTTP route at `/mcp` on the C7 listener, mounted in `internal/api/listener/listener.go` alongside `/healthz` and `/webhooks/`. The handler is `internal/mcp/transport_http.go`, a thin adapter that:

- Dispatches `POST /mcp` (JSON-RPC requests) and `GET /mcp` (SSE upgrade) to the same `internal/mcp/server.go` handler set as the stdio transport.
- Reads `Authorization: Bearer <token>` from the incoming HTTP request; the standard C9 `BearerToken` authenticator runs in the C7 interceptor stack and produces a `Principal`. Same auth path as every other Connect RPC.
- Multiplexes per-session state by an `Mcp-Session-Id` request header (per the MCP spec's Streamable HTTP transport). The header is minted by the server on first request and echoed by the client thereafter; the daemon keeps an in-memory session map keyed by session id.
- Honors `MCPRouteConfig.max_concurrent_sessions` and `session_idle_timeout` (§10).
- Sets `x-gohome-source: mcp` and `x-gohome-mcp-session: <id>` on the synthetic Connect call objects so that the C8 metrics/audit pipeline lights up identically to the stdio path.

Tools and resources behave identically across stdio and HTTP. The same `internal/mcp/server.go` registers them once; only the transport adapter differs.

### 7.6 Registry visibility (entity / device / area listing)

`EntityService.List` and friends apply the policy filter at the projection-read level:

- `Authorize(principal, "read", {Kind: "list"})` checks the action allowlist.
- The registry query then runs unrestricted, but the result is post-filtered through `FilterEntities`. We do not push policy into SQL; the filter cost is bounded by the page size, and pushing into SQL would couple the projection schema to the policy compiler's selector form.
- `total_size` in the page response reflects the **post-filter** count for the user. Honest pagination beats a leaky count.

Same pattern for `DeviceService.List`, `AreaService.List`, etc. — for non-entity registries we use the natural target (`{Kind: "device", ID: ...}`); the area selector matches purely on slug + hierarchy.

### 7.7 What the user sees on a denial

| Surface | Failure shape |
|---|---|
| Connect unary | `PERMISSION_DENIED` + `ErrorDetail{reason: "forbidden", metadata: {sub_reason: "explicit_deny", rule: "kids_bedrooms_only"}}` |
| Connect stream subscribe (strict) | `PERMISSION_DENIED` + `ErrorDetail{reason: "subscription_filtered", metadata: {denied_count: 3, sample_denied: [...]}}` |
| Connect stream subscribe (filter) | Stream opens; denied entities never appear |
| MCP `tools/call` | `CallToolResult{isError: true, content: [{type: text, text: <error envelope JSON with reason: "forbidden", sub_reason, rule>}]}` (per C8 §7.1 mapping table) |
| MCP `resources/read` denied URI | MCP error reason `forbidden` (same envelope shape) |
| MCP `resources/subscribe` (filter) | Subscription opens narrowed; no notification |
| MCP `resources/subscribe` (strict) | MCP error reason `subscription_filtered` |
| Web UI | Component-level "you don't have permission" surfaces with a "why?" link that calls `AuthService.ExplainAuthorization` |

Surfacing the rule name in metadata is intentional — operators authoring a deny rule want to see it fire when expected. For end-user-facing UI, the front-end can mask the rule name behind a "why?" expansion if the operator wants to.

### 7.8 What does NOT need changing in C9

- C7's listener stack (already mounts the interceptor chain).
- C7's per-service action catalog (already populated; we just look it up).
- C8's MCP tool/resource dispatch (already calls Authorize through the seam).
- C6's automation / script execution (already runs as `system:auto`; subject to the §7.1 system-principal bypass).

The only handler-side change anywhere is `service_auth.go` — replacing the C7 `UNIMPLEMENTED` stubs with real implementations.

---

## 8. Audit Events

C7 reserved tag 11 in the `Payload` oneof for `AuthEvent`. C9 fills it in. All payloads live in `proto/gohome/event/v1/auth_event.proto`.

```proto
message AuthEvent {
  Identity identity = 1;
  oneof kind {
    LoginSucceeded         login_succeeded         = 10;
    LoginFailed            login_failed            = 11;
    Logout                 logout                  = 12;
    SessionRefreshed       session_refreshed       = 13;
    SessionReplayDetected  session_replay_detected = 14;
    PasswordChanged        password_changed        = 15;
    PasskeyRegistered      passkey_registered      = 16;
    PasskeyUnregistered    passkey_unregistered    = 17;
    EnrollmentTokenMinted   enrollment_token_minted   = 18;
    EnrollmentTokenRedeemed enrollment_token_redeemed = 19;
    TokenMinted            token_minted            = 20;
    TokenRevoked           token_revoked           = 21;
    TokenRejected          token_rejected          = 22;
    PolicyDenied           policy_denied           = 23;
    PolicyCompiled         policy_compiled         = 24;
  }
}

message Identity {
  string principal_id = 1;     // "user:fdatoo", "system:local", "anonymous"
  string source_ip    = 2;     // post-trusted-proxy resolution
  string user_agent   = 3;
  string request_id   = 4;     // matches slog
}
```

Per-kind highlights:

| Kind | Adds |
|---|---|
| `LoginSucceeded` | `auth_method` (`passkey`/`password`/`token`), `user_slug`, `session_id` (for cookie), `credential_id` (for passkey) |
| `LoginFailed` | `auth_method`, `attempted_user_slug` (may be empty), `reason` (`bad_credentials`/`password_not_available`/`throttled`/`sign_count_regression`/`passkey_assertion_invalid`) |
| `Logout` | `user_slug`, `session_id` |
| `SessionRefreshed` | `user_slug`, `session_id`, `new_session_id` (post-rotation) |
| `SessionReplayDetected` | `user_slug`, `session_id`, `revoked_session_count` |
| `PasswordChanged` | `user_slug`, `set_by` (`self`/`admin:slug`/`system:bootstrap`) |
| `PasskeyRegistered` / `Unregistered` | `user_slug`, `credential_id`, `label` |
| `EnrollmentTokenMinted` / `Redeemed` | `user_slug`, `intent`, `expires_at` |
| `TokenMinted` | `user_slug`, `token_id`, `label`, `scope_summary`, `ttl_seconds`, `issued_by_principal_id` |
| `TokenRevoked` | `token_id`, `revoked_by_principal_id`, `reason` (`user_request`/`admin_action`/`expired_cleanup`) |
| `TokenRejected` | `token_id_prefix` (first 8 chars only — log doesn't store full id of unknown tokens), `reason` |
| `PolicyDenied` | `action_service`, `action_method`, `action_verb`, `target_kind`, `target_id`, `sub_reason` (`action_denied`/`target_denied`/`explicit_deny`/`token_action_denied`/`token_target_denied`), `rule_name` (when sub_reason is `explicit_deny`) |
| `PolicyCompiled` | `generation`, `policy_count`, `compile_duration_ms`, `compiled_by_principal_id` |

**Emission discipline:**

- `Authenticate` failures: emit `LoginFailed` (or `TokenRejected` for the token path).
- `Authorize` allow: **no event**. Audit-on-success would multiply event volume by ~2 with no information value.
- `Authorize` deny: emit `PolicyDenied`.
- `PolicyCompiled` emitted by the compiler on every successful build, before the artifact swap.

`AuthEvent`s flow through the same `eventstore.Append` path as everything else, so they are cursor-ordered alongside `StateChanged`, `ConfigApplied`, etc. A timeline like "config applied at cursor 12340 → policy_compiled at 12341 → policy_denied at 12342" reads naturally end-to-end.

**No PII in audit beyond what already exists.** User slugs (already in the registry), credential IDs (opaque blobs), session IDs (random ULIDs). No password material, no full token bytes (only `token_id` for known tokens; only an 8-char prefix for unknown rejected tokens), no WebAuthn user-handle reuse outside the credential record.

---

## 9. CLI Surface

All under `internal/cli/cmd_auth*.go`. The CLI talks to the daemon via the same Connect client every other subcommand uses (UDS by default; `--endpoint` to override).

```
gohome auth bootstrap <user-slug> [--intent register_passkey | set_password] [--ttl 1h]
    Mint a one-time enrollment token for a Pkl-declared user.
    Requires UDS (system:local). Prints the token to stdout, expires per --ttl.

gohome auth login [--user <slug>] [--method passkey | password]
    Browser-style login from the CLI. Opens an HTTP loopback listener,
    redirects through the web UI's auth flow (passkey or password), captures
    the resulting session cookie, persists it in ~/.config/gohome/cli-session.
    Subsequent CLI invocations use the cookie via TCP transport.
    For UDS users, this is a no-op (the CLI is already system:local).

gohome auth logout
    Invalidates the local session cookie (calls AuthService.Logout, deletes
    the local file). UDS users get "no session to log out of."

gohome auth whoami
    Print current Principal: id, kind, roles, source (UDS/cookie/token),
    auth method, session expiry. Useful for "wait, who am I right now."

gohome auth tokens create --user <slug> --label <text>
                          [--ttl <duration>]
                          [--scope mcp]
                          [--allow-tool <name> ...] [--allow-service <name> ...]
                          [--allow-area <slug> ...] [--allow-class <name> ...]
                          [--allow-entity <id> ...]
    Mint an API token. Prints the token plaintext exactly once.
    Issuer must be authenticated; scope intersection rules from §5.3 apply.

gohome auth tokens list [--user <slug>]
    Show id, label, scope summary, ttl, last-used. Plaintext is gone forever.

gohome auth tokens revoke <token-id>
    Revoke. Requires admin verb on AuthService unless revoking own token.

gohome auth users list
    Show users from the registry projection: slug, display name, roles,
    active flag, has-password, passkey count, last-login.

gohome auth passkeys list [--user <slug>]
    Show the user's registered passkeys: credential-id (truncated), label,
    registered-at, last-used. Defaults to current user.

gohome auth passkeys remove <credential-id-prefix>
    Revoke a credential. Self-service for own credentials; admin verb for others.

gohome auth set-password <user-slug>
    Interactive prompt; sets/replaces the user's password hash.
    Self-service for own password; admin verb for others.
    Refuses when AuthSettings.password_login_enabled = false.

gohome auth hash-password
    Reads a password from stdin, prints the Argon2id-encoded hash to stdout.
    For populating bootstrap_password_hash in Pkl. Pure local computation; no daemon.

gohome auth explain --user <slug> --action <Service.Method> --target <kind:id> [--verb <verb>]
    Calls AuthService.ExplainAuthorization; prints the trace from §6.5.
    Operators reach for this when "why was X denied" is the question.

gohome auth policies list
    Show compiled policies grouped by role: rule count, action allowlist
    summary, last-compile timestamp, generation.

gohome auth policies inspect <policy-name>
    Pretty-print the compiled rule set for one policy with selector expansion.

gohome auth rotate-cookie-key
    Rotate the cookie HMAC key. Existing sessions become invalid; everyone re-logs in.
    Confirmation prompt before proceeding. Requires admin.
```

**Lipgloss styling** (per the repo convention from C8):

| Element | Style |
|---|---|
| Header bar (`AUTH — gohome <version>`) | `styles.HeaderBar` |
| User slug / token id / credential id | `styles.Identifier` |
| Role badges (`admin`, `member`, `guest`, `kids`) | `styles.BadgeRole` (color-cycled per role) — new in `styles_auth.go` |
| Verb badges (`read` / `call` / `write` / `admin`) | reused `styles.BadgeRead` / `BadgeCall` / `BadgeAdmin` from C8 + new `BadgeWrite` |
| Decision result (`ALLOWED` / `DENIED`) | `styles.BadgeOK` / `BadgeError` |
| Rule names | `styles.RuleName` (semibold + accent underline) — new |
| Selector summary (`areas: [kids_floor], classes: [Light]`) | `styles.SubtleText` |
| Section dividers | `styles.Divider` |
| Footer summary line | `styles.SubtleText` |
| Token plaintext at mint time | `styles.SecretBox` (boxed with warning border) — new; printed once with "STORE THIS NOW" affordance |

`styles_auth.go` houses the four new styles; everything else reuses C8 / existing palette. `--json` flag on every list / inspect / explain command emits unstyled JSON for machine consumption.

---

## 10. Configuration

### 10.1 `gohome.auth` Pkl module (new)

```pkl
module gohome.auth

class User {
  slug:         String(matches(Regex(#"^[a-z][a-z0-9_-]{1,31}$"#)))
  display_name: String(length > 0)
  roles:        List<Role>
  active:       Boolean = true
  password_allowed: Boolean = true
  passkey_allowed:  Boolean = true
  oidc_subject:     String? = null         // RESERVED for v1.x OIDC
  bootstrap_password_hash: String? = null
}

class Role {
  slug:         String(matches(Regex(#"^[a-z][a-z0-9_-]{1,31}$"#)))
  display_name: String(length > 0)
  inherits:     List<Role> = List()
}

class AuthSettings {
  // Method toggles
  password_login_enabled: Boolean = true
  passkey_login_enabled:  Boolean = true

  // WebAuthn relying party
  rp_id:                       String
  rp_display_name:             String = "gohome"
  rp_origins:                  List<String>
  webauthn_user_verification:  "required" | "preferred" | "discouraged" = "preferred"

  // Argon2id
  argon2id_time:        UInt = 3
  argon2id_memory_kib:  UInt = 65536       // 64 MiB
  argon2id_parallelism: UInt = 4

  // Session lifetimes
  access_cookie_ttl:  Duration = 15.min
  refresh_cookie_ttl: Duration = 30.d
  refresh_idle_ttl:   Duration = 14.d

  // Throttle
  failed_attempts_window:    Duration = 10.min
  failed_attempts_threshold: UInt     = 10
  failed_attempts_block:     Duration = 15.min

  // Tokens
  token_default_ttl:    Duration = 90.d
  token_max_ttl:        Duration = 365.d
  token_label_required: Boolean  = true

  // Cookie name overrides (rare)
  access_cookie_name:  String = "gohome_access"
  refresh_cookie_name: String = "gohome_refresh"

  // Reveal denied entity ids in explain output (filter mode never reveals).
  reveal_denied_in_explain: Boolean = true
}
```

### 10.2 `gohome.policy` Pkl module (new)

Full schema in §6.1.

### 10.3 `gohome.core` extensions

C7's `gohome.core.pkl` already has `Listener` and `WebhookConfig`. C9 adds the MCP HTTP route's runtime knob:

```pkl
class Listener {
  // ... existing fields ...
  mcp: MCPRouteConfig = new MCPRouteConfig {}
}

class MCPRouteConfig {
  // Whether to mount the /mcp Streamable HTTP route on the TCP listener.
  // Stdio MCP works regardless of this flag (it doesn't touch the listener).
  enabled: Boolean = true

  // Maximum concurrent MCP HTTP sessions.
  max_concurrent_sessions: UInt = 32

  // Idle timeout for an HTTP MCP session.
  session_idle_timeout: Duration = 30.min

  // Path. Almost nobody wants to override this.
  path: String = "/mcp"
}
```

### 10.4 Daemon-managed (not Pkl)

- **Cookie HMAC signing key** — `<data_dir>/auth/cookie.key`, mode 0600, generated on first start, rotated by `gohome auth rotate-cookie-key`.
- **Token secret-randomness source** — `crypto/rand`. No knob.
- **Argon2id pepper** — not used. (Adds little when daemon and DB live on the same host.)

This pattern matches how C7 handles TLS cert paths (Pkl points at the file; the file is operator-supplied).

### 10.5 Reload semantics for `AuthSettings`

| Field | Reload behavior |
|---|---|
| `password_login_enabled`, `passkey_login_enabled` | Effective immediately on next login attempt. Existing sessions/tokens unaffected. |
| `rp_id`, `rp_origins`, `rp_display_name` | Effective for new WebAuthn ceremonies. Existing registered passkeys keep working iff the new `rp_id` is the same registrable suffix. Compile-time validation checks this; if `rp_id` changed, `gohome config validate` warns about credentials that would orphan. |
| Argon2id params | Only used for newly-set passwords; existing hashes carry their own params. |
| Session TTLs | Apply to new sessions only; existing sessions keep their original TTLs. |
| Throttle params | Apply to the next attempt window. |
| Token TTLs | Apply to newly-minted tokens. |
| Cookie names | Apply to new cookies; old-name cookies are honored for one access-cookie TTL after reload, then ignored. |

The compiler emits a `PolicyCompiled` audit event on every successful build so the timeline carries reload provenance.

---

## 11. Testing Strategy

Three layers, mirroring C7 and C8.

### 11.1 Unit tests (per-package)

**`internal/auth/credentials/`:**

- Password: Argon2id encode/verify roundtrip, parameter-mismatch handling, silent re-hash trigger when params change, constant-time-compare paths.
- WebAuthn: registration ceremony with mocked `go-webauthn` library, attestation acceptance/rejection, sign-count regression detection, multi-credential add/remove.
- Tokens: format roundtrip (`gohome_<id>_<secret>`), hash storage (plaintext never touches the row), expiry enforcement, revocation flag flip, `last_used_at` batched flush.
- Enrollment: one-time semantics (consumed flag), expiry, intent-mismatch rejection.

**`internal/auth/sessions/`:**

- Cookie HMAC sign/verify, tampered-cookie rejection.
- Refresh rotation: happy path, replay detection (cookie presented twice → entire session revoked + audit emit).
- Cookie attributes: `HttpOnly`, `Secure`, `SameSite=Strict`, path, max-age.

**`internal/auth/authn/`:**

- Chain ordering: `LocalPeerCred` short-circuits on UDS, never on TCP; bearer wins over cookie if both present; reject-all is the terminal.
- Each authenticator's `ErrNotApplicable` vs `ErrUnauthenticated` distinction.

**`internal/policy/compiler.go`:**

- Role inheritance: transitive expansion, cycle detection, self-loop rejection.
- Selector hashing: identical structural inputs produce identical hashes; field-order doesn't affect hash.
- Hierarchical area expansion: parent → all descendants; multiple roots; orphan area handled.
- Action allowlist: wildcard service expansion against the C7 catalog; unknown action fails compile.
- Deny-with-targets sanity: rule referencing a non-entity action with a non-empty target fails compile.

**`internal/policy/runtime.go`:**

- Hybrid path: action allowlist fast-reject; allow-rule walk on permitted actions; deny wins; `system:local` bypass; token-scope intersection; `FilterEntities` for streaming.
- Reload: atomic swap; in-flight requests on old generation finish without races; new requests pick up new generation.
- `Authorize` contract under every `(action, target, principal)` combination from a generated truth table — table-driven against a fixture policy.

**`internal/policy/explain.go`:**

- Trace shape matches the §6.5 example for allow / deny-by-target / deny-by-explicit-rule / deny-by-token-scope.

### 11.2 SDK / handler-level integration (in-process, fake daemon stack)

`internal/api/service_auth_test.go` and `internal/api/interceptor_authn_test.go` spin up the listener stack with real-but-isolated dependencies (in-memory eventstore, ephemeral SQLite, real policy runtime, real session/token stores):

- Login flows: passkey roundtrip via `go-webauthn`'s test helpers; password happy path + throttle trip; token authentication with valid / expired / revoked / forged secrets.
- Cookie session: login → access-cookie issued → request with cookie → success → access expires → refresh → new cookies → request → success.
- Refresh replay: present old refresh cookie after rotation → entire session revoked + audit event in store.
- Bootstrap: pkl declares user → `bootstrap` mints enrollment token over UDS → enrollment token redeemed → passkey registered → audit chain in event store matches §4.4.
- Authorize end-to-end: per-method action catalog × policy runtime → expected allow/deny on representative RPCs from each service.
- Subscription policy: `EntityService.Subscribe` with `policy_mode=filter` narrows; `policy_mode=strict` returns `PERMISSION_DENIED`. Reload mid-stream re-evaluates filter and emits add/remove synthetic events.
- MCP HTTP transport: `POST /mcp` with bearer token → tool dispatch → policy enforced. Bearer missing → 401. Bearer revoked → 401 + audit `TokenRejected`.
- `system:local` bypass: UDS-originated request hits Authorize → permitted regardless of policy; audit logs principal.

### 11.3 End-to-end (`//go:build integration`)

A real daemon binary, real SQLite, real Pkl config, real listener, exercised via the gohome CLI and the official `modelcontextprotocol/go-sdk` MCP client. Walks the full operator journey:

1. Bring up daemon with bootstrap config (`fdatoo` + `nora` declared, no credentials).
2. `gohome auth bootstrap fdatoo` → enrollment token printed.
3. UI flow simulated: redeem token → register passkey (using a virtual authenticator from `go-webauthn`'s testing utilities) → audit chain present.
4. `gohome auth login` (CLI HTTP loopback flow) → cookie persisted to `~/.config/gohome/cli-session`.
5. `gohome auth tokens create --user fdatoo --label "claude-test" --scope mcp --allow-area kitchen` → token returned.
6. Spawn an MCP HTTP client with that token; drive via the MCP SDK client; assert tool dispatch is gated to kitchen entities only.
7. Apply a config update introducing the `kids_bedrooms_only` policy; assert `PolicyCompiled` event; assert `nora`'s subsequent `EntityService.CallCapability` on `lock.front_door` returns `PERMISSION_DENIED` reason `explicit_deny` rule `kids_bedrooms_only`.
8. `gohome auth explain --user nora --action EntityService.CallCapability --target entity:lock.front_door` → trace matches.
9. Revoke the agent token; confirm next MCP HTTP call returns 401 + `TokenRejected` audit.

### 11.4 Audit / metric assertions

Every integration test checks `gohome_auth_*` and `gohome_policy_*` metrics increment correctly:

| Metric | Type | Labels |
|---|---|---|
| `gohome_auth_login_attempts_total` | counter | `method` (`passkey`/`password`/`token`/`cookie`), `result` (`ok`/`bad_credentials`/`throttled`/`session_invalid`/`token_invalid`/...) |
| `gohome_auth_login_duration_seconds` | histogram | `method` |
| `gohome_auth_active_sessions` | gauge | (no labels) |
| `gohome_auth_active_tokens` | gauge | (no labels) |
| `gohome_auth_throttle_blocks_total` | counter | `method` |
| `gohome_policy_compile_duration_seconds` | histogram | (no labels) |
| `gohome_policy_compile_generation` | gauge | (no labels) |
| `gohome_policy_authorize_total` | counter | `result` (`allow`/`deny`), `sub_reason` |
| `gohome_policy_authorize_duration_seconds` | histogram | (no labels) |

These slot into C7's `gohome_api_*` family and C8's `gohome_mcp_*` family — same registry, same scrape endpoint.

### 11.5 Property-style tests

Two pieces of policy logic warrant generated input rather than table-driven coverage:

- **Selector matching.** Generated `(EntitySelector, Entity)` pairs against the matcher; assert the matcher agrees with a much simpler reference implementation that walks the predicates literally.
- **Token-scope intersection.** Generated `(user_policy, token_scope, action, target)` triples; assert `Authorize` permits iff the literal `user.allow ∧ token.allow ∧ ¬user.deny` boolean holds. The intersection rule is exactly the kind of property-test target where a bug ("token can widen!") is one regression away.

Use `gopter` or stdlib generators; corpus saved in `testdata/policy-properties/` for replay.

### 11.6 What's NOT exhaustively tested in C9

- Browser JS for the WebUI auth flow lands in C10. C9 ships only the server side. The integration test uses the WebAuthn library directly with a virtual authenticator.
- TLS lifecycle paths (cert rotation behavior under load, ACME handshake) — C13 territory; C9 just verifies that `Secure` cookies and HTTPS-required RP origins are enforced when configured.
- OIDC anything (deferred).

---

## 12. Implementation Order

Suggested sequencing (detailed in the C9 implementation plan):

1. Add `github.com/go-webauthn/webauthn` and `golang.org/x/crypto/argon2` Go dependencies (latter is already transitive via stdlib's testing; pin explicit version).
2. Define `proto/gohome/event/v1/auth_event.proto` payloads (kinds 10–24); regenerate.
3. Add the `gohome.auth` and `gohome.policy` Pkl modules.
4. Add SQLite migrations for the `auth_*` tables.
5. `internal/auth/identity/store.go` — projector from `ConfigApplied` populating `auth_users` and `auth_user_roles`.
6. `internal/auth/credentials/password.go` — Argon2id wrapper + tests.
7. `internal/auth/credentials/tokens.go` — issue / verify / hash / revoke + tests.
8. `internal/auth/credentials/enrollment.go` — one-time-use bootstrap tokens + tests.
9. `internal/auth/credentials/webauthn.go` — go-webauthn integration + storage adapter + tests.
10. `internal/auth/sessions/store.go` + `cookies.go` — server-side store + HMAC-signed cookies + rotation/replay handling + tests.
11. `internal/auth/throttle/throttle.go` — per-IP × per-method counter + tests.
12. `internal/auth/audit/recorder.go` — emits `AuthEvent` payloads; called from credentials, sessions, throttle, policy runtime.
13. `internal/auth/authn/bearer.go` + `cookie.go` + `chain.go` — real authenticator chain + tests.
14. `internal/policy/schema.go` — `Compiled` and friends.
15. `internal/policy/selector.go` — selector hashing + matching + tests.
16. `internal/policy/compiler.go` — Pkl `PolicyConfig` → `Compiled` + tests.
17. `internal/policy/intersect.go` — token-scope intersection helpers + tests.
18. `internal/policy/runtime.go` — `Authorize`, `FilterEntities`, `OnReload` + tests.
19. `internal/policy/explain.go` + `AuthService.ExplainAuthorization` impl.
20. `internal/api/interceptor_authn.go` — wire the real chain in place of the C7 stub.
21. `internal/api/interceptor_authz.go` — wire the policy runtime in place of `AllowAll`.
22. `internal/api/service_auth.go` — implement all `AuthService` RPCs (replaces C7 `UNIMPLEMENTED` stubs); add `MintEnrollmentToken`, `RedeemEnrollmentToken`, `ChangePassword`, `ExplainAuthorization`.
23. Subscription policy filter wiring on every `*Service.Subscribe` handler (Entity / Driver / Automation traces, etc.).
24. `internal/mcp/transport_http.go` — Streamable HTTP adapter.
25. Wire `transport_http` into `internal/api/listener/listener.go` at the `/mcp` mount; enable via `MCPRouteConfig`.
26. Swap `auth.AllowAll{}` for the policy runtime in the MCP server construction.
27. `internal/cli/cmd_auth.go` + sibling files — full CLI tree with lipgloss styling.
28. End-to-end integration tests (`//go:build integration`) walking the §11.3 journey.
29. README / docs update: bootstrap walkthrough, MCP HTTP setup snippets, token-scope examples.

---

## 13. Decision Record

| # | Decision | Rationale |
|---|---|---|
| D1 | One spec covering all of master §7.4 + the C8 hand-offs; OIDC carved out and deferred to v1.x | Auth and policy are tightly coupled (sessions need roles, MCP HTTP needs token + policy, audit needs both). OIDC has its own surface (issuer discovery, JWKS rotation, claim mapping, group sync) and is opt-in by design — most prosumers won't enable it day one. Cutting it lets the spec stay tight without splitting. |
| D2 | Allow + deny rules, deny wins, default-deny | Matches the master design's example one-to-one. Deny-wins is the well-trodden semantic operators expect (AWS IAM, Kubernetes RBAC). The "kids can't touch the lock" pattern is an obvious early use case that gets ugly under allow-only models. |
| D3 | Pkl owns identity (users, roles, policies); registry owns credentials (passwords, passkeys, tokens, sessions); CLI mints enrollment tokens | Pkl is git-versioned and human-edited — perfect for "fdatoo is an admin," actively wrong for "fdatoo's passkey credential id is 0x4f3a..." (binary blobs that change with every device re-enrollment). Bootstrap CLI needs only one job: mint an enrollment token for an already-declared user so they can attach credentials. |
| D4 | Tokens carry tool/RPC allowlist + EntitySelector, intersected with user policy (never widening) | Tool/RPC names are a closed enum — `--allow-tool gohome__get_state` is the natural ergonomics for issuing an agent token. EntitySelector reuse is cheap. Strict intersection means an admin issuing a token can't accidentally grant more than the admin already has, and a token's effective surface only ever shrinks as boundaries change. |
| D5 | `policy_mode = "filter" \| "strict"` knob on subscriptions, default `filter` | "Filter" is the right default for a dashboard subscribing to "all entities in my home" — a kid's kitchen view shouldn't 403 because the kitchen contains a denied entity. "Strict" exists for callers that want loud failure. Knob added because both modes have legitimate use cases. |
| D6 | MCP HTTP transport uses Streamable HTTP + bearer tokens; OAuth 2.1 deferred | MCP spec's OAuth section is actively churning; building to it now risks shipping a stale shape. Bearer tokens are stable, ubiquitous, and exactly the credential a Q4 token already is. Major MCP clients accept bearer-token configuration today. |
| D7 | `system:local` bypasses Authorize entirely; audit still records it | Anyone with shell access can edit Pkl, read SQLite, restart the daemon — policy can't meaningfully constrain them. Matches existing prosumer-tool conventions (Docker, systemctl, postgres-via-unix-socket). The honest threat-model answer. |
| D8 | Selector grammar = areas (hierarchical) + classes + entity_ids; no globs, tags, or predicate language in v1 | Hierarchical areas are the single biggest ergonomic win — without them every "main floor" rule expands by hand into a list of rooms that drifts as the home changes. C4 already has the hierarchy; the compiler walking it costs almost nothing. Globs / tags / predicate languages can be added additively later if real users ask. |
| D9 | Hybrid policy runtime: precomputed `(role × verb) → action allowlist` + runtime selector evaluation | Action space is closed and small (~12 services × ~5 verbs); precomputing per-role action allowlists is cheap and gives O(1) reject on every unauthorized call. Target space is unbounded and dynamic (entities come/go); precomputing target sets would mean recompiling on every entity-registry change, coupling policy reload to driver activity in ugly ways. |
| D10 | Cookie HMAC key, token randomness, and Argon2id parameters managed daemon-side; not in Pkl | Pattern matches how C7 handles TLS cert paths. Pkl declares posture; the daemon manages cryptographic material. Operator can rotate via `gohome auth rotate-cookie-key`. |
| D11 | Refresh rotation with single-use semantics: replayed cookie revokes the entire session | Standard rotating-refresh-token pattern. Replay detection surfaces compromise into the audit log instead of letting an attacker share access invisibly. Cost: legitimate browsers must keep their refresh cookie consistent across tabs, which the cookie scope already enforces. |
| D12 | Failed-auth: soft per-IP × per-method throttle, no account lockout | Lockouts on a homelab cause more pain than they prevent (someone fat-fingering a household member's password from the kitchen tablet shouldn't lock that household member out of their home). Throttle slows brute-force; admins still log in. |
| D13 | `bootstrap_password_hash` field in Pkl as the one identity↔credentials bridge; consumed once and ignored thereafter | Lets headless setups pre-stage a password without forcing a UDS interaction. The "consumed-once" semantics keep Pkl from accidentally re-asserting a stale credential on every reload. |
| D14 | `AuthEvent` audit on auth/authz failure and on policy compile, not on every authorize-allow | Authorize is called O(many) per request; emitting on every allow would multiply event volume without information value. Denials are noteworthy and rare-by-design. PolicyCompiled gives reload provenance. |

---

## 14. Explicit Deferrals

Named here so the design doc acknowledges them without blocking:

- **OIDC integration** — deferred to v1.x. The Pkl `User.oidc_subject` field and the `internal/auth/authn` chain position are reserved.
- **TLS lifecycle** (ACME, cert rotation, mTLS, self-signed generation) — owned by C13. C9 consumes whatever C7 already accepts via `Listener.TLSConfig`.
- **OAuth 2.1 MCP transport** — deferred; bearer tokens cover v1.
- **Two-factor auth beyond passkeys** — passkeys are inherently multi-factor; layered TOTP not in v1.
- **Per-policy time windows / schedules** — expressed via Starlark in C6 automations, not in policy.
- **Group sync from external IdPs** — bound up with OIDC, deferred together.
- **Account lockout policy beyond simple per-IP throttle** — ops-tooling concern.
- **Browser-side WebAuthn UI** — C10 owns the React flows.
- **Strict-mode enforcement on non-streaming Connect RPCs** — `policy_mode` only applies to subscriptions; point-target RPCs are either permitted or denied.
- **Selector grammar extensions** — entity-id globs, tags, predicate languages — add additively when concrete demand appears.
- **Argon2id pepper** — adds little when daemon and DB live on the same host.
- **MCP server-initiated sampling and prompts** — same as the C8 deferral; revisit when a use case appears.

---

*End of C9 design document.*
