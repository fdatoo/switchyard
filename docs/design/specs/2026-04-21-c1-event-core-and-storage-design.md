# C1 — Event Core & Storage Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-21
**Status:** Draft → Implementation-ready once approved

---

## 1. Scope and Deliverables

C1 delivers the foundation that every subsequent module builds on: the event store, the state cache, the registry projector, the foundational SQLite tables, project scaffolding, logging, and a read-only CLI.

### Packages (in `github.com/fynn-labs/gohome`)

- `internal/eventstore` — append, subscribe, snapshot, projector dispatch
- `internal/state` — copy-on-write state cache (HAMT-based)
- `internal/registry` — registry projector + device/entity/driver query API
- `internal/observability` — slog setup, Prometheus registry, metric helpers, tracing stubs
- `internal/storage` — SQLite open, PRAGMAs, `storage.Tx` abstraction, migrations (goose)
- `internal/cli` — Cobra command tree, lipgloss-styled rendering
- `internal/testutil` — shared test helpers (in-memory DB, event builders, fixture loaders)
- `cmd/gohomed/main.go` — daemon entry point
- `cmd/gohome/main.go` — CLI entry point

### What C1 does NOT include

Deferred to later child docs, in dependency order:

| Scope | Doc |
|-------|-----|
| Carport driver host + gRPC | C2 |
| Pkl config loader + validator | C3 |
| Connect-RPC API surface + MCP | C4 |
| Starlark automation engine | C5 |
| Area/zone/scene tables + logic | C6 |
| Auth (passkeys, Argon2id, OIDC, policies) | C7 |
| Web UI (React + dashboards) | C8 |
| `gohome-edge` agent | C9 |
| Surgical repair CLI (`events delete`, `projector rebuild`, `events verify`) | C13 |
| OTel bridge (span stubs ship in C1) | C13 |
| Backup/restore tooling | C13 |
| Migration from Home Assistant | C14 |

### Deliverable binaries

- `gohomed` — runs, opens DB, applies migrations, exposes `/metrics`, emits a `SystemEvent{kind:"startup"}`, idles cleanly until SIGTERM.
- `gohome` — read-only CLI over local SQLite (daemon RPC transport comes in C4; C1 uses direct SQLite reads + a minimal UNIX socket for mutative ops like `snapshot create`).

### Success criteria

C1 is done when:

- `go build ./cmd/gohomed && go build ./cmd/gohome` produce working binaries.
- `gohomed` starts, applies migrations, exposes `/metrics`, emits a `SystemEvent{kind:"startup"}`, idles until SIGTERM.
- `gohome events tail` (against a running daemon) streams the startup event live.
- `gohome registry list entities` works (empty list until C2 drivers register anything).
- Golden replay tests for five fixtures pass.
- `kill -9 gohomed` mid-Append leaves the eventstore consistent on next start.
- `gohome snapshot create --owner=state_cache` writes a snapshot row; a subsequent restart uses it.
- Line coverage in `eventstore`, `state`, `registry` above 85%.
- CI pipeline (lint, test, `test:integration`, `test:race`) is green.

---

## 2. Repository Layout

Single git repo at `github.com/fynn-labs/gohome`. Daemon + CLI share proto and internal types, must version together; they travel as one Go module. Edge agent, drivers, web UI, docs, and Pkl modules live in their own repos as the master doc specifies.

```
github.com/fynn-labs/gohome          # repo + go module path

gohome/
├── .github/workflows/ci.yml         # lint, test, build — matrix linux/amd64, linux/arm64, darwin/arm64
├── .golangci.yml
├── Taskfile.yml                     # canonical build entry
├── buf.yaml
├── buf.gen.yaml
├── go.mod                           # module github.com/fynn-labs/gohome
├── go.sum
├── README.md
├── LICENSE
│
├── cmd/
│   ├── gohomed/main.go              # daemon binary
│   └── gohome/main.go               # CLI binary
│
├── internal/
│   ├── eventstore/
│   │   ├── store.go                 # Store type, Append, AppendBatch, Subscribe, Start
│   │   ├── tailer.go                # central tailer goroutine + sync.Cond fanout
│   │   ├── snapshot.go              # snapshot read/write, zstd, cadence
│   │   ├── projector.go             # Projector interface + registration
│   │   ├── filter.go                # Filter matching
│   │   ├── replay.go                # startup replay loop
│   │   └── store_test.go
│   ├── state/
│   │   ├── cache.go                 # COW cache over immutable.Map
│   │   └── cache_test.go
│   ├── registry/
│   │   ├── projector.go             # RegistryProjector implementing eventstore.Projector
│   │   ├── queries.go               # read API: GetDevice, ListEntities, etc.
│   │   ├── migrations/              # goose migrations for devices/entities/driver_instances/event_subscriptions
│   │   └── projector_test.go
│   ├── storage/
│   │   ├── open.go                  # OpenDB(path) — PRAGMAs, WAL mode, migrations
│   │   ├── tx.go                    # storage.Tx interface
│   │   ├── migrations/              # goose migrations for events/snapshots/projection_cursors/skipped_events
│   │   └── lockfile.go              # daemon lockfile check
│   ├── observability/
│   │   ├── logging.go               # slog.Init — charmbracelet TTY handler / JSON non-TTY
│   │   ├── metrics.go               # Prometheus registry, common metric helpers
│   │   ├── metrics_server.go        # /metrics HTTP handler
│   │   └── tracing.go               # OTel-ready no-op span stubs
│   ├── cli/
│   │   ├── root.go                  # cobra root, global flags
│   │   ├── events.go                # events query/tail/inspect/export
│   │   ├── state.go                 # state get/dump
│   │   ├── registry.go              # registry list/show
│   │   ├── snapshot.go              # snapshot create/list
│   │   └── styles.go                # shared lipgloss theme, table renderers
│   └── testutil/
│       ├── fixtures.go              # golden-file replay helpers
│       ├── sqlite.go                # in-memory SQLite factory
│       ├── store.go                 # NewTestStore helper
│       └── events.go                # well-formed Event builders
│
├── proto/
│   ├── buf.yaml
│   └── gohome/
│       ├── event/v1/
│       │   ├── event.proto          # Event envelope + Payload oneof + Filter + SystemEvent
│       │   └── snapshot.proto       # StateCacheSnapshot, EntityState
│       └── entity/v1/
│           └── attributes.proto     # oneof per entity type (capabilities + live attributes)
│
├── gen/                             # buf-generated Go code (committed)
│   └── gohome/event/v1/*.pb.go
│
├── testdata/
│   └── fixtures/                    # .jsonl golden fixtures + .golden.json expected outputs
│
└── docs/
    └── architecture.md              # link back to design docs in parent container
```

### Notable choices

- **`cmd/` holds both binaries** in the same module; shared proto and internal types.
- **No `pkg/`.** Everything is `internal/` unless we explicitly decide to expose a Go API. For C1, nothing is exposed.
- **Migrations co-located with the module that owns them.** `internal/storage/migrations/` for event-core tables; `internal/registry/migrations/` for registry tables. Each uses `embed.FS` so the binaries ship migrations inside.
- **`proto/` at repo root**, `buf` module root; generated code lands in `gen/` (committed to git).
- **`Taskfile.yml`** is the canonical build entry for humans and CI.

---

## 3. SQLite Schema

Two migration sets, each embedded in its owning package.

### `internal/storage/migrations/` — owned by eventstore

```sql
-- 0001_events.sql
CREATE TABLE events (
  position        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts              INTEGER NOT NULL,                -- unix nanos
  kind            TEXT    NOT NULL,
  entity          TEXT,                            -- nullable; not all events are entity-scoped
  source          TEXT    NOT NULL,
  correlation_id  BLOB,                            -- 16-byte UUID; groups a workflow
  cause_position  INTEGER,                         -- nullable; unenforced FK to events.position
  payload         BLOB    NOT NULL                 -- protobuf-encoded Event.payload oneof
);
CREATE INDEX events_ts          ON events(ts);
CREATE INDEX events_entity_ts   ON events(entity, ts)        WHERE entity IS NOT NULL;
CREATE INDEX events_kind_ts     ON events(kind, ts);
CREATE INDEX events_correlation ON events(correlation_id)    WHERE correlation_id IS NOT NULL;
CREATE INDEX events_cause       ON events(cause_position)    WHERE cause_position IS NOT NULL;

-- 0002_snapshots.sql
CREATE TABLE snapshots (
  position    INTEGER PRIMARY KEY,
  ts          INTEGER NOT NULL,
  owner       TEXT    NOT NULL,                    -- "state_cache" | "<projector_name>"
  encoding    TEXT    NOT NULL,                    -- "protobuf+zstd"
  state       BLOB    NOT NULL,
  meta        BLOB                                 -- optional: reason, schema version
);
CREATE INDEX snapshots_owner ON snapshots(owner, position DESC);

-- 0003_projection_cursors.sql
CREATE TABLE projection_cursors (
  name        TEXT PRIMARY KEY,
  position    INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);

-- 0004_skipped_events.sql
CREATE TABLE skipped_events (
  position     INTEGER NOT NULL,
  projector    TEXT    NOT NULL,
  skipped_at   INTEGER NOT NULL,
  skipped_by   TEXT    NOT NULL,
  reason       TEXT    NOT NULL,
  PRIMARY KEY (position, projector)
);
```

### PRAGMAs (runtime, applied at open)

```
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
PRAGMA cache_size=-64000;        -- 64 MB
PRAGMA temp_store=MEMORY;
PRAGMA mmap_size=268435456;      -- 256 MB
```

### `internal/registry/migrations/` — owned by registry projector

```sql
-- 0001_driver_instances.sql
CREATE TABLE driver_instances (
  id             TEXT PRIMARY KEY,
  driver_name    TEXT    NOT NULL,
  display_name   TEXT    NOT NULL,
  transport      TEXT    NOT NULL CHECK(transport IN ('local_subprocess','remote_grpc')),
  endpoint       TEXT    NOT NULL,
  config_hash    TEXT    NOT NULL,
  status         TEXT    NOT NULL CHECK(status IN ('starting','running','failed','stopped')),
  last_error     TEXT,
  started_at     INTEGER,
  last_heartbeat INTEGER,
  created_at     INTEGER NOT NULL
);

-- 0002_devices.sql
CREATE TABLE devices (
  id                  TEXT PRIMARY KEY,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  friendly_name       TEXT NOT NULL,
  manufacturer        TEXT,
  model               TEXT,
  sw_version          TEXT,
  metadata            BLOB,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX devices_driver ON devices(driver_instance_id);

-- 0003_entities.sql
CREATE TABLE entities (
  id                  TEXT PRIMARY KEY,             -- dotted-path, e.g. "light.living_room_ceiling"
  device_id           TEXT REFERENCES devices(id) ON DELETE SET NULL,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  entity_type         TEXT NOT NULL,
  friendly_name       TEXT NOT NULL,
  capabilities        BLOB NOT NULL,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX entities_type   ON entities(entity_type);
CREATE INDEX entities_device ON entities(device_id) WHERE device_id IS NOT NULL;
CREATE INDEX entities_driver ON entities(driver_instance_id);

-- 0004_event_subscriptions.sql
CREATE TABLE event_subscriptions (
  name         TEXT PRIMARY KEY,
  cursor       INTEGER NOT NULL,
  filter       BLOB,
  created_at   INTEGER NOT NULL,
  last_active  INTEGER NOT NULL
);
```

### Notes

- The `entity` column in `events` is NOT a foreign key — events may reference entities that were later deleted or existed before registration. The event log is ground truth; the `entities` table is a view.
- Foreign keys inside registry tables (devices → driver_instances, entities → devices) ARE enforced; the registry projector controls that consistency.
- `correlation_id` stored as 16-byte BLOB; rendered as UUID string only at display time.
- `cause_position` is intentionally unenforced. Events are never deleted in normal operation.

---

## 4. Core APIs and Types

### `storage.Tx`

```go
package storage

// Tx is the minimal transactional surface projectors work against.
// *sql.Tx satisfies this interface directly; fakes can substitute for tests.
type Tx interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

Projectors never import `database/sql`. `sql.Rows` and `sql.Row` stay concrete as result cursors — full abstraction there would be hexagonal-architecture creep for C1's goals.

### `eventstore.Store`

```go
package eventstore

type Store struct {
    // unexported: db, tailer, projectors, cache, logger, metrics, cond, subscribers
}

type Config struct {
    DBPath              string
    SnapshotEveryEvents int            // default 10_000
    SnapshotEveryPeriod time.Duration  // default 1 * time.Hour
    MaxSubscriberBuffer int            // default 256
    SnapshotRetainPerOwner int         // default 3
}

func Open(ctx context.Context, cfg Config, logger *slog.Logger, metrics *observability.Metrics) (*Store, error)
func (s *Store) Close(ctx context.Context) error

// Append writes a single event; synchronous projectors run in the same transaction.
func (s *Store) Append(ctx context.Context, e Event) (position uint64, err error)

// AppendBatch writes multiple events atomically.
func (s *Store) AppendBatch(ctx context.Context, events []Event) (positions []uint64, err error)

// Subscribe returns a live subscription starting at opts.FromPosition.
func (s *Store) Subscribe(ctx context.Context, opts SubscribeOptions) (Subscription, error)

// RegisterProjector adds a projector. Must be called before Start.
func (s *Store) RegisterProjector(p Projector, mode ProjectorMode) error

// Start runs migrations, restores snapshots, replays events, launches tailer and async projectors.
func (s *Store) Start(ctx context.Context) error

// Query reads historical events (CLI / future API). Read-only.
func (s *Store) Query(ctx context.Context, q QueryOptions) ([]Event, error)

func (s *Store) LatestPosition(ctx context.Context) (uint64, error)

// SnapshotNow forces an immediate snapshot for the named owner (or all).
func (s *Store) SnapshotNow(ctx context.Context, owner string) (uint64, error)
```

### `Event`

```go
type Event struct {
    Position       uint64           // 0 on input to Append; set by store on return
    Timestamp      time.Time
    Kind           string
    Entity         string           // empty if not entity-scoped
    Source         string
    CorrelationID  uuid.UUID        // zero if none
    CausePosition  uint64           // 0 if none
    Payload        *eventv1.Payload // generated protobuf oneof
}
```

### `Projector` and registration

```go
type ProjectorMode int
const (
    ProjectorModeSync ProjectorMode = iota
    ProjectorModeAsync
)

type Projector interface {
    Name() string
    Apply(ctx context.Context, tx storage.Tx, e Event) error
    Snapshot(ctx context.Context, tx storage.Tx) error
    Restore(ctx context.Context, tx storage.Tx) (resumeFrom uint64, err error)
}

// Embeddable base for projectors that don't snapshot.
type NoSnapshot struct{}
func (NoSnapshot) Snapshot(context.Context, storage.Tx) error { return nil }
func (NoSnapshot) Restore(context.Context, storage.Tx) (uint64, error) { return 0, nil }
```

### `Subscribe`

```go
type SubscribeOptions struct {
    FromPosition  uint64
    Filter        Filter
    Durable       bool
    Name          string  // required if Durable
    ChannelBuffer int     // 0 = use Config default
}

type Filter struct {
    Kinds          []string
    Entities       []string
    Sources        []string
    CorrelationIDs []uuid.UUID
    MinTs, MaxTs   time.Time
}

type Subscription interface {
    C() <-chan Event
    Ack(position uint64) error   // no-op for non-durable; debounced persist for durable
    Close() error
    Stats() SubscriptionStats
}

type SubscriptionStats struct {
    Delivered uint64
    Dropped   uint64
    LagEvents uint64
    Buffered  int
}
```

### `state.Cache`

```go
package state

type EntityID = string

type State struct {
    EntityID   EntityID
    UpdatedAt  time.Time
    UpdatedBy  string
    Attributes *entityv1.Attributes
}

type Cache struct {
    // unexported: atomic.Pointer[*immutable.Map[EntityID, State]]
}

func New() *Cache
func (c *Cache) View() *immutable.Map[EntityID, State]  // O(1); caller holds for consistent reads
func (c *Cache) Get(id EntityID) (State, bool)          // convenience: View + lookup
func (c *Cache) Len() int

// eventstore.Projector (sync mode):
func (c *Cache) Name() string
func (c *Cache) Apply(ctx context.Context, tx storage.Tx, e Event) error
func (c *Cache) Snapshot(ctx context.Context, tx storage.Tx) error
func (c *Cache) Restore(ctx context.Context, tx storage.Tx) (uint64, error)
```

### `registry.Registry`

```go
package registry

type Registry struct {
    // unexported: *sql.DB
}

func New(db *sql.DB) *Registry

// eventstore.Projector (sync mode):
func (r *Registry) Name() string
func (r *Registry) Apply(ctx context.Context, tx storage.Tx, e Event) error
// Snapshot/Restore are no-ops via NoSnapshot; registry state IS its SQL tables.

// Read API:
func (r *Registry) GetDriverInstance(ctx context.Context, id string) (DriverInstance, error)
func (r *Registry) ListDriverInstances(ctx context.Context) ([]DriverInstance, error)
func (r *Registry) GetDevice(ctx context.Context, id string) (Device, error)
func (r *Registry) ListDevices(ctx context.Context, filter DeviceFilter) ([]Device, error)
func (r *Registry) GetEntity(ctx context.Context, id string) (Entity, error)
func (r *Registry) ListEntities(ctx context.Context, filter EntityFilter) ([]Entity, error)
```

---

## 5. Event Lifecycle: the Append Path

The single most important flow in C1 — every correctness guarantee hinges on it.

```
Caller
  │
  ▼
1. Validate: kind non-empty, payload non-nil, source set.
   Fill CorrelationID if zero (generate UUID).
   Fill Timestamp if zero (time.Now).
  │
  ▼
2. Serialize payload (proto.Marshal).
  │
  ▼
3. BEGIN IMMEDIATE transaction.
  │
  ▼
4. INSERT INTO events RETURNING position. event.Position set.
  │
  ▼
5. For each SYNC projector, in registration order:
     err := projector.Apply(ctx, tx, event)
     if err != nil → ROLLBACK, return err.
   Also: state_cache.Apply builds the new immutable.Map into a tx-local slot
   (does NOT publish to atomic.Pointer yet).
   Update projection_cursors for each synced projector.
  │
  ▼
6. COMMIT.
  │
  ▼
7. Promote tx-local cache slot to active atomic.Pointer.
   cond.Broadcast() — wakes tailer.
   metrics: events_appended_total++.
   Return event.Position.
```

### Why `BEGIN IMMEDIATE`

SQLite's default is `DEFERRED`, which upgrades to a write lock lazily — can deadlock under contention. `IMMEDIATE` acquires the reserved lock immediately; we fail fast instead of partway through the projector chain.

### Why state cache is promoted after commit, not during

Building the new `immutable.Map` happens inside `state_cache.Apply` (step 5), so the in-transaction work stays consistent. But the atomic pointer swap happens AFTER commit (step 7). Rationale: if step 5 built the new cache and step 6 failed, the cache would reflect an un-committed event. Two-phase (build during tx, publish post-commit) keeps cache and log in lockstep.

### Failure handling

- SQL error on INSERT → retry with exponential backoff (100ms → 200ms → 400ms → 800ms → abort), then return error to caller and refuse further writes (degraded mode).
- Sync projector returns error → rollback, return error. No partial state.
- SQL error on COMMIT → rollback, retry path as INSERT.
- Post-COMMIT error → impossible by construction (in-memory only); if encountered, log ERROR but still return success to caller (event IS committed).

### AppendBatch

Same shape; step 4 loops over batch (many INSERTs, one tx), step 5's Apply loop runs per-event nested inside. All-or-nothing.

### Async projectors

Not called in step 5. Tailer goroutine dispatches to them in their own goroutines, each with its own `BEGIN`/`COMMIT`. Their cursors advance independently in `projection_cursors`. If an async projector fails, it retries with backoff; persistent failure → metric fires, projector stops advancing, operator investigates.

---

## 6. Tailer, Fanout, and Subscription Lifecycle

### Tailer loop

```go
func (s *Store) runTailer(ctx context.Context) {
    cursor := s.startupPosition
    for {
        s.cond.L.Lock()
        for cursor >= s.latestPosition && ctx.Err() == nil {
            s.cond.Wait()
        }
        target := s.latestPosition
        s.cond.L.Unlock()
        if ctx.Err() != nil { return }

        events, err := s.loadEventsAfter(ctx, cursor, target)
        if err != nil { /* log + backoff + continue */ }

        for _, e := range events {
            s.dispatch(ctx, e)
            cursor = e.Position
        }
    }
}
```

### Fanout

```go
func (s *Store) dispatch(ctx context.Context, e Event) {
    s.subMu.RLock()
    subs := s.subs
    s.subMu.RUnlock()

    for _, sub := range subs {
        if !sub.filter.Matches(e) { continue }
        select {
        case sub.ch <- e:
            sub.delivered.Add(1)
        default:
            sub.dropped.Add(1)
            s.metrics.SubscriberDropped.WithLabelValues(sub.name).Inc()
            s.logger.Warn("subscriber dropped, closing", "name", sub.name)
            go s.closeSubscriber(sub)
        }
    }

    for _, p := range s.asyncProjectors {
        select {
        case p.queue <- e:
        default:
            p.switchToCatchup()    // reads from SQL, not channel; never drops events
        }
    }
}
```

### Subscription lifecycle

1. **`Subscribe()` arrives.** Caller specifies `FromPosition` and optional durable name.
2. **Durable resolution.** If `Durable=true`: look up `event_subscriptions.cursor` by name; if absent, INSERT a row with `cursor=FromPosition`.
3. **Catchup phase.** Subscriber NOT yet registered with tailer. Dedicated goroutine queries `events` from cursor to `LatestPosition()` in pages (1000/page), pushes to subscriber channel, advances cursor.
4. **Live handoff.** Once catchup reaches latest, take `subMu`, re-check `latestPosition`. If still matching, register atomically. If not (events arrived during catchup), loop step 3 with a shorter page. Guarantees no event missed, no duplicate delivered.
5. **Live phase.** Subscriber receives from tailer via fanout.
6. **`Ack(position)`.** For durable: UPDATE `event_subscriptions.cursor`, debounced to at most once per second per subscriber. For non-durable: no-op.
7. **`Close()`.** Unregister from tailer, close channel, update `last_active`. Durable rows stay.
8. **Slow-consumer eviction.** Channel full on send → tailer drops subscriber: close channel, unregister. Caller detects via closed channel, reconnects from last ack position.

### Backpressure matrix

| Who's slow | Consequence |
|------------|-------------|
| SQLite commits | Append returns error → caller's problem |
| Sync projector | Same — abort Append |
| Async projector | Switches to SQL catchup mode; never loses events |
| Subscriber | Dropped from fanout; reconnects via Ack |
| Tailer itself | Impossible to starve — cond.Wait wakes on any Append |

---

## 7. Snapshot Mechanism

### Ownership

| Owner         | Content                                            | Encoding        |
|---------------|----------------------------------------------------|-----------------|
| `state_cache` | Full `immutable.Map[EntityID, State]` serialized   | protobuf + zstd |
| `registry`    | No-op — state IS the SQL tables; `Restore()` reads cursor from `projection_cursors`, `Snapshot()` is a no-op | — |
| Future async projectors | Per-module state                         | protobuf + zstd |

Snapshot rows live in the shared `snapshots` table, distinguished by `owner`.

### Cadence

A dedicated `snapshotter` goroutine ticks every minute:

```go
for _, entry := range s.snapshotEntries {
    if entry.shouldSnapshot() {   // events-since-last ≥ 10k OR time-since-last ≥ 1h
        s.runSnapshot(ctx, entry)
    }
}
```

`SnapshotNow(owner)` bypasses cadence — used by `gohome snapshot create` and pre-shutdown.

### Write path

```
1. BEGIN DEFERRED  (snapshots don't contend with events — separate table, WAL allows readers).
2. owner.Snapshot(tx)
     - state_cache: serialize atomic.Pointer snapshot → protobuf → zstd → INSERT.
     - registry: no-op.
3. COMMIT.
4. Metrics: snapshot_duration_seconds, snapshot_size_bytes, snapshot_last_position.
5. Prune: keep `Config.SnapshotRetainPerOwner` most recent per owner (default 3). DELETE older.
```

### State cache snapshot proto

```protobuf
// proto/gohome/event/v1/snapshot.proto
message StateCacheSnapshot {
  uint64 position = 1;
  int64  ts       = 2;
  repeated EntityState entities = 3;
}

message EntityState {
  string entity_id                       = 1;
  int64  updated_at                      = 2;
  string updated_by                      = 3;
  gohome.entity.v1.Attributes attributes = 4;
}
```

### Restore path (startup)

Per-owner, during `Store.Start`:

- **state_cache.Restore:** SELECT most recent row WHERE owner='state_cache' ORDER BY position DESC LIMIT 1. If found: zstd-decompress, proto-unmarshal, build `immutable.Map`, store in atomic.Pointer, return position. If no row: return 0.
- **registry.Restore:** SELECT position FROM projection_cursors WHERE name='registry'. If found: return position. If absent: return 0.

### Corruption handling

If `Restore` fails on decode (zstd checksum, proto parse error):
1. Log with snapshot position.
2. Metric: `gohome_snapshot_corruption_total{owner}`.
3. Try the next-older snapshot.
4. If all fail: fall through to full replay from 0, log loudly.
5. If event replay also fails: recovery mode (§8).

---

## 8. Startup Recovery

The boot sequence, designed to survive `kill -9` during any prior step.

### Phase 1: Cold open

1. Parse config from flags + environment variables. (Pkl config loader lands in C3; C1 surface: `--data-dir`, `--log-level`, `--log-format`, `--admin-port`, `--snapshot-every-events`, `--snapshot-every-period`.)
2. Initialize observability: slog (charmbracelet TTY / JSON non-TTY); Prometheus registry; start `/metrics` HTTP server on admin port.
3. Acquire `<data_dir>/gohomed.lock` (PID + start time). If lockfile exists and PID alive: abort. If stale: remove.
4. Open SQLite at `<data_dir>/gohome.db`. Apply PRAGMAs. Run goose migrations (both embedded sets).

### Phase 2: Restore

5. For each registered projector + state_cache: `resumeFrom[owner] = owner.Restore(ctx, tx)`.
6. `cursor := min(resumeFrom[owner])` over all owners.

### Phase 3: Replay (batched)

```
7.  latestAtStart := SELECT MAX(position) FROM events
8.  while cursor < latestAtStart:
      batch := SELECT * FROM events WHERE position > cursor
               ORDER BY position LIMIT 1000
      if len(batch) == 0: break
      BEGIN IMMEDIATE
      for event in batch:
        for owner in sync_owners_needing_event(event.Position):
          if skipped_events has (position, owner): continue with WARN log
          if err := owner.Apply(ctx, tx, event):
            ROLLBACK
            enter_recovery_mode(err, event.Position)
            return
        update projection_cursors where cursor < event.Position
      COMMIT
      cursor = batch[last].Position
```

During replay: no fanout, no async projectors, no subscribers. Async projectors catch up in Phase 5.

### Phase 4: Live transition

9. `startupPosition := latestAtStart`.
10. Start tailer goroutine.
11. Start snapshotter goroutine.
12. Start async projectors (each reads its own cursor, catches up via SQL, flips to live).
13. Mark store Started — `Append` and `Subscribe` now permitted.

### Phase 5: Daemon readiness

14. Open Carport host (no-op stub in C1; drivers launch in C2).
15. Emit `SystemEvent{kind:"startup", data: <version, commit, migration_versions>}` via `store.Append`. The "hello world" event.
16. Open API server (no-op stub in C1; Connect-RPC + MCP land in C4).
17. Signal ready (`/health` returns 200).
18. Block on ctx.Done() (SIGTERM/SIGINT handler).

### Recovery mode

Triggered by Phase 3 failure (sync projector Apply returns error).

- Log at ERROR with event position and projector name.
- Do NOT start tailer, snapshotter, async projectors, drivers, API.
- DO start restricted admin HTTP server:

```
GET  /health                         → {status: "recovery", reason, failed_position}, HTTP 503
GET  /metrics                        → Prometheus
GET  /events?position=N&limit=K      → inspect events near failure
GET  /projection-cursors             → show stuck projector positions
POST /events/:position/skip          → mark (position, projector) as skipped
POST /shutdown                       → clean exit
```

- Log loudly every 30s.
- `skipped_events` table is operator-guarded surgery; actual repair CLI lands in C13.

### Kill -9 safety

| Killed during | Next startup |
|---------------|--------------|
| Phase 1 | Clean restart; nothing persistent changed |
| Phase 2 | Clean restart; `Restore` is idempotent (reads only) |
| Phase 3 | Resume from last committed batch; cursors advanced per batch COMMIT |
| Phase 4-5 | Normal replay of anything post-startup (should be empty, but handled) |

### Readiness signal

`/health`:
- Phase 1-4: `{status: "starting", phase: N}`, HTTP 503.
- Phase 5 success: `{status: "ready"}`, HTTP 200.
- Recovery: `{status: "recovery", reason, failed_position}`, HTTP 503.

---

## 9. Observability

### Logging (`internal/observability/logging.go`)

```go
type LogConfig struct {
    Level  slog.Level
    Format string       // "auto" | "tty" | "json"
    Output io.Writer
}

func Init(cfg LogConfig) *slog.Logger
```

- `format=auto`: TTY-detect on `Output`; charmbracelet/log TTY, stdlib JSONHandler otherwise.
- `format=tty`: always charmbracelet/log.
- `format=json`: always stdlib `slog.NewJSONHandler`.

**Level theme (TTY):**
- DEBUG → dim gray
- INFO → blue
- WARN → yellow
- ERROR → bold red

**Standard structured attributes:**
`event_position`, `event_kind`, `entity`, `correlation_id`, `projector`, `driver_instance`, `subscription`, `phase`.

Logger propagates via context (`observability.WithLogger(ctx, logger)`); scoped attributes added by handlers/commands (e.g., `cmd="events query"`).

### Metrics catalog

```
# Append path
gohome_events_appended_total{kind}
gohome_events_append_duration_seconds
gohome_events_append_retries_total
gohome_events_append_failures_total{stage="insert|projector|commit"}

# Projectors
gohome_projector_apply_duration_seconds{projector,mode}
gohome_projector_failures_total{projector,mode}
gohome_projector_lag_events{projector}
gohome_projector_catchup_mode{projector}

# Tailer
gohome_tailer_lag_events
gohome_tailer_batch_size

# Subscriptions
gohome_subscription_active{name}
gohome_subscription_delivered_total{name}
gohome_subscription_dropped_total{name}
gohome_subscription_buffered{name}
gohome_subscription_catchup_duration_seconds{name}

# Snapshots
gohome_snapshot_duration_seconds{owner}
gohome_snapshot_size_bytes{owner}
gohome_snapshot_last_position{owner}
gohome_snapshot_corruption_total{owner}

# Storage
gohome_sqlite_wal_bytes
gohome_sqlite_events_total
gohome_sqlite_busy_retries_total

# Startup
gohome_startup_phase
gohome_startup_duration_seconds
gohome_replay_events_processed_total
gohome_recovery_mode_entered_total

# Health
gohome_build_info{version,commit,goversion}
gohome_uptime_seconds
```

### Tracing (OTel-ready, no-op in C1)

```go
type Span interface {
    End()
    SetAttr(key string, value any)
    RecordError(err error)
}

func StartSpan(ctx context.Context, name string) (context.Context, Span)
```

C1 ships no-op implementation. C13 replaces it with OTel bridge; every existing call site becomes a real span with zero code changes.

**Instrumented sites in C1:**
- `eventstore.Append`
- `eventstore.AppendBatch` (with `batch_size` attr)
- `projector.Apply` (child span per projector)
- `state.Cache.Apply`, `registry.Registry.Apply`
- Startup phases (one span per phase)
- Subscription catchup

### Policy

- **Log** unusual or human-relevant events (startup phases, projector failures, subscription drops, recovery mode).
- **Meter** counts and durations.
- **Trace** causal flows.

Do not log what metrics already capture (don't log every Append; `gohome_events_appended_total` is there).

---

## 10. CLI

Cobra command tree, TTY auto-detection, lipgloss styling. Daemon RPC transport deferred to C4; C1 uses direct SQLite reads + minimal UNIX socket for mutative ops.

### Commands shipping in C1

```
gohome
├── events
│   ├── query   [flags]             historical query with filters
│   ├── tail    [flags]             live streaming
│   ├── inspect <position>          one event in full detail
│   └── export  <range> --output=   JSONL dump
├── state
│   ├── get     <entity_id>
│   └── dump    [--format=json]
├── registry
│   ├── list    (devices|entities|drivers)
│   └── show    <id>
├── snapshot
│   ├── create  [--owner=] [--reason=]
│   └── list    [--owner=]
└── version
```

### Global flags

```
--data-dir string       default ~/.local/share/gohome
--format string         "auto" (TTY→table, pipe→json) | "table" | "json" | "yaml"
--no-color              disable ANSI styling
--log-level string      error|warn|info|debug
-v, --verbose           shortcut for --log-level=debug
```

(A `--config` flag is intentionally absent in C1; Pkl config loader lands in C3.)

### Transport in C1

1. Acquire `SHARED` SQLite lock (reader mode) — never `EXCLUSIVE`.
2. WAL mode allows reads to coexist with the daemon's writes.
3. Mutative ops (`snapshot create`):
   - If daemon alive: connect to `<data_dir>/gohomed.sock`, call minimal RPC `Snapshot(owner, reason) → position`.
   - If daemon absent: open store directly in CLI process, call `SnapshotNow`, close.

C4 retrofits CLI to Connect-RPC primary; the UNIX-socket stub goes away then.

### Styling

`internal/cli/styles.go` owns the theme (lipgloss):

```go
var (
    styleHeader      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C7CFF"))
    styleEntityID    = lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0"))
    styleKind        = lipgloss.NewStyle().Foreground(lipgloss.Color("#DCDCAA"))
    styleTimestamp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
    styleCorrelation = lipgloss.NewStyle().Foreground(lipgloss.Color("#C586C0"))
    styleError       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C")).Bold(true)
    styleSuccess     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AC776"))
    styleDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
```

Tables rendered via `charmbracelet/lipgloss/table`. Auto-sized columns, truncate + ellipsis in narrow terminals. Payload rendered with `protojson.Marshal` (stable, oneof-aware).

Pipe-detect via `termenv` strips ANSI automatically.

### Example rendering

`gohome events tail` (TTY):
```
15:03:41.213  state_changed    light.living_room      driver:hue-1
15:03:41.289  command_issued   switch.kitchen_fan     automation:evening_wind_down   corr=a1b2..
15:03:41.412  command_ack      switch.kitchen_fan     driver:zigbee2mqtt             cause=1042
```

`gohome events inspect 1042` (TTY): framed box with labeled fields, relative+absolute timestamps, pretty-printed payload.

`gohome registry list entities` (TTY): table with Entity ID, Type, Name, Driver, Status columns; footer row with totals.

### Progress

Long operations use `charmbracelet/huh` spinners + row counters. `--quiet` suppresses.

---

## 11. Testing Strategy

### Infrastructure (`internal/testutil`)

```go
func NewTestDB(t *testing.T) *sql.DB                     // in-memory SQLite + migrations
func NewTestStore(t *testing.T, opts ...StoreOption) *eventstore.Store
func StateChanged(entity string, opts ...EventOption) Event   // + other builders
func LoadFixture(t *testing.T, name string) []Event
func ReplayAndSnapshot(t *testing.T, events []Event) GoldenSnapshot
func AssertGolden(t *testing.T, path string, got GoldenSnapshot)  // supports -update
```

### Test layers

**Unit (`*_test.go` alongside code):**
- `eventstore/filter_test.go` — table-driven Filter.Matches.
- `state/cache_test.go` — immutable.Map behavior, concurrent reads during writes, Snapshot stability.
- `eventstore/store_test.go` — monotonic positions, AppendBatch atomicity, Subscribe ordering, durable Subscribe resume.
- `registry/projector_test.go` — EntityRegistered → row appears; EntityUnregistered → disabled=1; idempotent re-apply.

**Golden replay (`testdata/fixtures/`):**

Five required fixtures for C1:
- `basic_state_flow` — 20 events: register entity, state changes, unregister.
- `scene_apply` — 100 events with correlation_id clustering.
- `driver_restart` — driver goes offline, entities disable, comes back.
- `snapshot_roundtrip` — events up to a snapshot, more events, restore verifies.
- `correlation_walk` — cause_position chains.

Each is a `.jsonl` of `protojson`-encoded events + a `.golden.json` canonical output:
```json
{
  "state_cache": {"light.living_room": {...}},
  "registry": {
    "driver_instances": [...],
    "devices":          [...],
    "entities":         [...]
  },
  "projection_cursors": {"state_cache": 20, "registry": 20}
}
```

**Property (`testing/quick` + custom generators):**
- Live == Replay: any event sequence fed live equals same sequence replayed from 0.
- Append monotonicity under concurrent callers.
- Filter symmetry: `Filter.Matches(e)` iff Subscribe with filter delivers `e`.
- Snapshot round-trip invariance.
- Cause-chain reachability: follow chain to an event with `cause_position=0`.

**Fuzz (Go 1.18+):**
- `FuzzEventDecode` — random BLOBs; no panics.
- `FuzzFilterMatch` — random Filter and Event; no panics.
- `FuzzFixtureParse` — protojson.Unmarshal random bytes; no panics.

Corpus seeded from `testdata/fixtures`.

**Crash-safety (`//go:build integration`):**
- `TestAppend_ProcessKillMidTransaction` — subprocess Appends and sleeps, kill -9, reopen, verify.
- `TestReplay_InterruptedResumes` — cancel ctx mid-batch, reopen, verify completion.
- `TestSnapshot_CorruptedFallsBack` — write 3 snapshots, corrupt newest, verify fallback.

Real on-disk SQLite (not `:memory:`) to exercise WAL + fsync.

### Coverage targets

- `eventstore`, `state`, `registry` > 85% lines.
- ≥ 5 golden fixtures.
- ≥ 1 property test per public eventstore API.
- 3 crash-safety scenarios above.

### CI

```yaml
# Taskfile.yml
tasks:
  test:                go test ./...
  test:integration:    go test -tags=integration ./...
  test:fuzz:           go test -fuzz=Fuzz -fuzztime=30s ./internal/eventstore ./internal/registry
  test:race:           go test -race ./...
  test:update-golden:  go test ./... -update
```

`.github/workflows/ci.yml` runs `test`, `test:integration`, `test:race` on PRs. Fuzz runs weekly on a scheduled workflow.

---

## 12. Decision Record

| # | Decision | Choice |
|---|----------|--------|
| 1 | Scope boundary | eventstore + state + registry + foundational tables; drivers/API/UI/auth deferred |
| 2 | Repo structure | Single `github.com/fynn-labs/gohome`, both binaries, one Go module |
| 3 | Logging | stdlib `slog` + charmbracelet/log TTY / JSON non-TTY |
| 4 | Storage engine | SQLite (modernc.org/sqlite) with WAL |
| 5 | Causation | UUID `correlation_id` + position `cause_position` |
| 6 | Batch append | `Append` + `AppendBatch` methods |
| 7 | Idempotency | No dedup at store layer |
| 8 | Snapshot scope | Per-projector independent snapshots |
| 9 | Snapshot encoding | Protobuf + zstd |
| 10 | Snapshot cadence | Every 10 000 events OR 1 hour |
| 11 | State cache | COW with `github.com/benbjohnson/immutable` HAMT |
| 12 | Projector model | `Projector` interface, sync default, async opt-in |
| 13 | Tx abstraction | `storage.Tx` wraps `*sql.Tx`; projectors don't import `database/sql` |
| 14 | Entity IDs | HA-style dotted path (`light.living_room`) |
| 15 | State persistence | Cache only, not SQL table; event log is source of truth |
| 16 | Capabilities schema | Protobuf oneof per entity type |
| 17 | Replay batch | 1000 events/tx |
| 18 | Runtime failures | Retry SQLite with backoff → refuse writes; sync projector fail → abort Append; slow subscriber → drop |
| 19 | Fanout pattern | Central tailer + `cond.Broadcast` + typed channels + SQL catchup |
| 20 | Startup failure | Recovery mode (read-only admin HTTP + `/metrics`) |
| 21 | CLI in C1 | Direct SQLite reads + UNIX-socket `SnapshotNow` stub; retrofits to Connect-RPC in C4 |
| 22 | Observability | slog + Prometheus now, OTel-ready span stubs, OTel bridge in C13 |
| 23 | Testing | Unit + golden replay + property + fuzz + crash-safety |
| 24 | Config in C1 | Flags + env vars only; Pkl loader lands in C3 |

---

## 13. External Dependencies

### Go modules

```
modernc.org/sqlite                              // pure-Go SQLite driver
github.com/pressly/goose/v3                     // migrations
google.golang.org/protobuf                      // proto codec
github.com/google/uuid                          // correlation IDs
github.com/benbjohnson/immutable                // HAMT for state cache
github.com/klauspost/compress/zstd              // snapshot compression
github.com/prometheus/client_golang             // metrics
github.com/charmbracelet/log                    // TTY slog handler
github.com/charmbracelet/lipgloss               // CLI styling
github.com/charmbracelet/huh                    // progress spinners (light touch)
github.com/spf13/cobra                          // CLI command tree
github.com/muesli/termenv                       // terminal capability detection
```

### Dev/build tools

```
github.com/bufbuild/buf                         // proto lint + build
google.golang.org/protobuf/cmd/protoc-gen-go    // proto codegen
github.com/golangci/golangci-lint               // linter
github.com/go-task/task                         // Taskfile runner
```

### Deferred (added later)

- `go.starlark.net` — C5 (automations).
- `apple.github.io/pkl/go` — C3 (config loader).
- `connectrpc.com/connect` + `protoc-gen-connect-go` — C4 (API).
- OpenTelemetry Go SDK — C13 (tracing bridge).

---

## 14. What C2 Inherits from C1

- A running `gohomed` that can append events and fan them out.
- Registry tables ready to receive `EntityRegistered` / driver-lifecycle events.
- A `Projector` interface for Carport to register its own (e.g., in-flight command tracking).
- `storage.Tx` abstraction for Carport's projectors to use.
- Logging and metrics helpers.
- Event proto envelope with `DriverEvent`, `EntityRegistered`, `EntityUnregistered` payload kinds already defined.
- UNIX-socket stub in daemon (C2 can extend it temporarily until C4 lands Connect-RPC).
