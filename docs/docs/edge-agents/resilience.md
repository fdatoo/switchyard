# Edge Agent Resilience

!!! status-wip "In development"
    The edge agent is designed but partially implemented. The API may change.

`switchyard-edge` is designed to survive the conditions common in home networks: flaky WAN links, reboots in the wrong order, power cuts that take down the primary before the edge (or vice versa). This page describes how the edge handles each of those situations.

---

## Local event buffering

When the edge loses its connection to the primary, it does not stop the drivers or drop their state events. Instead, it switches each affected driver connection to **buffering mode**:

- Driver subprocesses keep running. From a driver's perspective, nothing changes — it continues emitting state events on its local Unix socket.
- The edge agent writes every `RunUp` event to a per-driver append-only log file on local disk. Records are length-prefixed protobuf; segments rotate at 8 MB.
- Events are buffered until the connection is restored.

### Buffer limits

Three caps apply per driver (whichever hits first):

| Cap | Default |
|---|---|
| Maximum on-disk size | 100 MB |
| Maximum event count | 100,000 events |
| Maximum event age | 24 hours |

On overflow, the oldest segment is dropped and an `EdgeBufferOverflowed` event is recorded. That overflow event itself drains to the primary on reconnect, so the audit log shows exactly when and how much was lost. Drivers keep running through overflow — real-time control is preserved at the cost of the oldest buffered history.

You can tune these limits per edge agent in Pkl:

```pkl
new edge.EdgeAgent {
  slug    = "garage-pi"
  drivers = ["zwave-garage"]
  buffer {
    max_bytes         = 200 * 1024 * 1024  // 200 MB
    max_events        = 200_000
    max_age_hours     = 48
    fsync_interval_ms = 1000               // default
  }
}
```

### Durability guarantee

Buffer writes are batched-fsynced every 1000 ms (or 256 records, whichever comes first). A crash or power loss can result in losing up to one fsync interval worth of events — approximately one second at default settings. If you need stronger durability (at the cost of write throughput on SD cards), set `fsync_interval_ms = 0` to force a synchronous fsync on every record.

---

## Reconnection behavior

When the connection drops, the edge immediately schedules a reconnect attempt using **exponential backoff**:

| Attempt | Delay |
|---|---|
| 1 | 1 s |
| 2 | 2 s |
| 3 | 4 s |
| 4 | 8 s |
| … | … |
| cap | 60 s |

Each delay has ±20% jitter applied to prevent thundering herds when multiple edges reconnect simultaneously after a primary restart.

---

## Drain on reconnect

When the connection to the primary is restored, the edge enters **drain mode** before resuming live forwarding:

1. The edge replays all buffered events in order, each tagged with its `edge_seq` sequence number.
2. The primary appends them to the event store. Deduplication is keyed on `(driver_instance_slug, edge_seq)` — if a network hiccup causes the same event to arrive twice, the second append is a silent no-op.
3. The primary periodically sends `IngestAck{last_durably_appended_seq}` back to the edge. The edge advances its trim cursor and frees the corresponding buffer space.
4. Once the buffer is empty, the edge transitions to live forwarding and the drain is complete.

Fresh events emitted by the driver during a drain continue to accumulate behind the drain pointer — they are replayed in order after the buffered events, preserving strict ordering.

---

## Driver restart behavior on the edge

If a driver subprocess on the edge crashes, the edge agent handles it **independently**, without coordinating with the primary:

- The edge restarts the driver using the same supervisor policy `switchyardd` uses for local drivers (exponential backoff, same states: `spawning`, `awaiting_handshake`, `running`, `backoff`, `quarantined`).
- The per-driver buffer is preserved across the driver restart. Any events emitted before the crash and not yet drained to the primary remain in the buffer.
- The primary's view: the driver's health probe times out, marking it unhealthy. When the driver restarts on the edge and the connection re-establishes, the primary sees it come back healthy — identical to a local driver restart.

---

## Boot-offline operation

The edge caches the last-known driver assignment from the primary in an encrypted file (`/var/lib/switchyard-edge/cache/assignment.bin`). On boot:

```
switchyard-edge starts
  │
  ├─ Try connecting to the primary
  │     ├─ Success → fetch fresh assignment, spawn drivers, enter steady state
  │     │
  │     └─ Failure → load encrypted cache
  │           ├─ Cache valid → spawn drivers per cached assignment,
  │           │                start buffering immediately,
  │           │                reconnect loop in background
  │           │
  │           └─ No cache / decryption failed → log "awaiting primary",
  │                                              wait in reconnect loop;
  │                                              do NOT spawn anything
```

This means a power cut that brings the edge up before the primary is not a problem — drivers start from cache and buffer their events until the primary comes back.

The cache is encrypted with a key derived from the edge's mTLS private key. If you re-pair (which generates a new key), the cache is treated as invalid and the edge waits for a fresh assignment from the primary.

---

## Assignment updates while connected

The edge keeps a persistent server-streaming connection open to `EdgeService.WatchAssignment`. When the primary's Pkl config is reloaded — adding a driver, changing a driver's config, removing a driver from the edge — the primary pushes the updated assignment bundle to the edge immediately. The edge reconciles:

- **New driver** → spawn subprocess.
- **Removed driver** → graceful Carport `Shutdown`, terminate subprocess.
- **Config change** → graceful `Shutdown`, respawn with new config.

No polling. No operator action on the edge.

---

## Multi-edge scenarios

Multiple edge agents can connect to one primary simultaneously. Each edge is independently paired, independently managed, and independently buffered. The primary maintains a separate connection state and buffer telemetry entry for each edge slug.

Edges do not communicate with each other. Cross-edge coordination (if ever needed) goes through the primary.

A driver instance can only be assigned to **one** edge at a time. The Pkl validator rejects configs where the same driver instance appears in more than one `EdgeAgent.drivers` list.

---

## Health monitoring

### On the primary

```
$ switchyard edge ls

SLUG          STATE              DRIVERS   BUFFER
garage-pi     connected          1/1       0 B
basement-pi   offline-buffering  2/2       14 MB
vacation-hub  never-connected    3         —
```

```
$ switchyard edge show garage-pi

Edge:       garage-pi
State:      connected (since 2026-04-27 08:12:31)
Cert:       serial 3a:b2:... · expires 2026-07-26 (89 days)

Drivers:
  INSTANCE       STATE     BUFFER     OLDEST EVENT
  zwave-garage   live      0 B        —

Last assignment synced: 2026-04-27 08:12:31 (hash: a1b2c3d4)
```

Color coding: green = healthy, amber = degraded (offline-buffering, cert expiry < 30 days), red = critical (cert expired, revoked, buffer overflow).

### On the edge host

```
$ switchyard-edge status

Edge:    garage-pi  →  tls://switchyardd.lan:7443
State:   connected

Drivers:
  INSTANCE       STATE   BUFFER
  zwave-garage   live    0 B

Cert expires: 2026-07-26 (89 days)
```

---

## Metrics

Edge-side metrics are exposed on an optional `:9090` Prometheus endpoint (`switchyard-edge run --metrics-addr :9090`):

| Metric | Type | Description |
|---|---|---|
| `edge_buffer_bytes{driver_slug}` | gauge | On-disk buffer size per driver |
| `edge_buffer_events{driver_slug}` | gauge | Buffered event count per driver |
| `edge_buffer_oldest_age_seconds{driver_slug}` | gauge | Age of the oldest buffered event |
| `edge_buffer_dropped_events_total{driver_slug}` | counter | Events dropped due to buffer overflow |
| `edge_connection_state{driver_slug}` | gauge | 0 = down, 1 = draining, 2 = live |
| `edge_reconnect_attempts_total{driver_slug, result}` | counter | Reconnect attempt outcomes |
| `edge_cert_expires_in_seconds` | gauge | Time until the edge certificate expires |

Primary-side metrics (in `switchyardd`'s `/metrics` endpoint):

| Metric | Type | Description |
|---|---|---|
| `carport_edge_connections_active{edge_slug}` | gauge | 1 if the edge has any active driver connection |
| `carport_edge_drivers_connected{edge_slug}` | gauge | Number of active driver connections |
| `carport_edge_ingest_lag_seconds{edge_slug, driver_slug}` | gauge | Time since the last event received from this driver |
| `carport_edge_handshake_total{edge_slug, result}` | counter | Handshake outcomes (ok, cert_invalid, revoked, …) |

---

## Audit events

All edge lifecycle events land on the primary's event log alongside `StateChanged` and config events:

| Event | When |
|---|---|
| `EdgeAgentEnrolled` | Successful pairing |
| `EdgeAgentConnected` | Edge established a new connection |
| `EdgeAgentDisconnected` | Edge disconnected (clean, timeout, error, or revoked) |
| `EdgeAgentCertRotated` | Automatic or manual certificate renewal |
| `EdgeAgentRevoked` | Operator revoked the edge |
| `EdgeBufferOverflowed` | Buffer cap exceeded; events dropped |
| `EdgeAssignmentSynced` | Primary pushed a new assignment bundle to the edge |
