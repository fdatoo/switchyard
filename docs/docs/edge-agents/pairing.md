# Pairing an Edge Agent

!!! status-wip "In development"
    The edge agent is designed but partially implemented. The API may change.

Before `gohome-edge` can connect to your primary daemon, you pair them. Pairing establishes a mutual TLS identity: the primary mints a certificate for the edge, and both sides verify each other on every subsequent connection. There is no external certificate authority — `gohomed` runs its own internal CA.

The pairing token is **single-use** with a default TTL of 1 hour (configurable up to 24 hours with `--ttl`). Once used or expired, the token cannot be reused.

---

## Step 1 — Declare the edge in Pkl

Edge agents are first-class Pkl objects. Before minting a pairing token, declare the edge in your primary's config:

```pkl
// edges.pkl
import "@gohome/edge.pkl"

new edge.EdgeAgent {
  slug        = "garage-pi"
  description = "Z-Wave host in the garage"
  drivers     = ["zwave-garage"]
}
```

Apply the config:

```
gohome config apply
```

The Pkl validator enforces that each `EdgeAgent.slug` is unique and that no driver instance appears in more than one edge's `drivers` list (a driver can only run in one place).

---

## Step 2 — Mint an enrollment token on the primary

Run this on the **primary host**, over the local Unix socket (`system:local` access only — it cannot be called remotely):

```
$ gohome edge mint-enrollment garage-pi --ttl 1h

Enrollment token (single use, expires 2026-04-27T16:13:00Z):

   gohome-edge-tok_a3f9b2c1d8e5f6a7...

Pair the edge with:

   gohome-edge pair --primary tls://gohomed.lan:7443 --token <above>
```

The token carries a fingerprint of the primary's CA certificate embedded as a checksum suffix. This lets the edge verify it is talking to the right primary on first connect — no trust-on-first-use.

The plaintext token is **never logged or stored on the primary** — only its SHA-256 hash is persisted. Show it once, copy it securely.

---

## Step 3 — Run the pairing command on the edge host

Copy the token to the edge host (SSH, secure paste, or similar — your choice of channel). Then run:

```
$ gohome-edge pair \
    --primary tls://gohomed.lan:7443 \
    --token gohome-edge-tok_a3f9b2c1...
```

What happens under the hood:

1. The edge dials TCP to `gohomed.lan:7443`.
2. The edge performs a TLS handshake (without a client cert at this stage). It verifies the primary's server certificate against the CA fingerprint embedded in the token. A fingerprint mismatch aborts immediately.
3. The edge sends `EdgeService.RedeemEnrollmentToken` over that TLS connection, providing the token and a Certificate Signing Request for a freshly generated Ed25519 keypair.
4. The primary validates the token (hash, expiry, intent), signs the CSR with its internal CA (90-day certificate lifetime), marks the token consumed, and records an `EdgeAgentEnrolled` event in the audit log.
5. The primary returns the signed certificate, the CA certificate, and the primary endpoint.
6. The edge stores `cert + key + ca_cert + endpoint` in `/var/lib/gohome-edge/` (directory `0700`, private key `0600`). The edge's private key never leaves the edge host — the pairing flow uses a CSR.
7. The command exits 0 and prints a confirmation.

---

## Step 4 — Verify registration

Back on the primary:

```
$ gohome edge ls

SLUG        STATE          CERT EXPIRY         DRIVERS
garage-pi   never-connected  2026-07-26        zwave-garage
```

---

## Step 5 — Start the edge agent

On the edge host:

```
gohome-edge run
```

For production deployments, run it under systemd. A ready-made unit file is provided in the release package:

```ini
[Unit]
Description=gohome Edge Agent
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/gohome-edge run
Restart=always
RestartSec=5
StateDirectory=gohome-edge
User=gohome-edge

[Install]
WantedBy=multi-user.target
```

Enable and start:

```
sudo systemctl enable --now gohome-edge
```

---

## Certificate lifecycle

The CA is generated once by `gohomed` on first boot and persisted in the auth registry, encrypted with the daemon's at-rest key. It has a 10-year lifetime.

Edge certificates have a **90-day lifetime** and renew automatically:

- On every reconnect, the edge checks whether its certificate expires within 30 days.
- If so, it calls `EdgeService.RenewCert` over the existing mTLS connection, signs a fresh CSR, and atomically replaces the stored certificate.
- An `EdgeAgentCertRotated` event is written to the audit log.
- No operator action is required for normal renewals.

```
Pair  ──→  90-day cert  ──→  (≤30d to expiry)  RenewCert  ──→  90-day cert
                         ──→  (Revoked)          CRL hit; reconnect blocked
                         ──→  (Manual rotate)    gohome edge rotate → re-pair
```

---

## Revoking or rotating an edge

To decommission an edge or recover from a compromised host:

```
gohome edge revoke garage-pi
```

This adds the certificate serial to the CRL, immediately drops any active connections from that slug, and rejects future connections. The edge cannot reconnect without a fresh pairing. An `EdgeAgentRevoked` event is audited.

To rotate credentials while keeping the edge in service (forces a re-pair):

```
gohome edge rotate garage-pi
```

This revokes the current certificate and mints a fresh enrollment token for you to copy to the edge host.

---

## CLI reference

### Primary (`gohome` CLI)

| Command | Description |
|---|---|
| `gohome edge mint-enrollment <slug> [--ttl 1h]` | Mint a one-time pairing token for a Pkl-declared edge |
| `gohome edge ls` | List all declared edges and their connection state |
| `gohome edge show <slug>` | Detailed status: cert expiry, drivers, buffer telemetry |
| `gohome edge rotate <slug>` | Revoke current cert and mint a fresh enrollment token |
| `gohome edge revoke <slug>` | Permanently revoke; edge cannot reconnect without re-pair |

### Edge host (`gohome-edge` CLI)

| Command | Description |
|---|---|
| `gohome-edge pair --primary <addr> --token <tok>` | Initial pairing |
| `gohome-edge run` | Start the supervisor (default subcommand) |
| `gohome-edge status` | Show local connection state, drivers, buffer summary |
| `gohome-edge rotate-cert` | Force an immediate certificate renewal round-trip |
