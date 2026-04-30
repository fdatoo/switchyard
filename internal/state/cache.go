// Package state owns the in-memory copy-on-write cache of entity state.
// The cache is a materialized projection of the event log; the log is
// the source of truth.
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/immutable"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/storage"
)

type EntityID = string

type State struct {
	EntityID   EntityID
	UpdatedAt  time.Time
	UpdatedBy  string
	Attributes *entityv1.Attributes
}

// Cache holds entity state via an immutable HAMT behind atomic.Pointer.
// The store calls Apply inside an Append transaction to build the next
// snapshot, then calls Promote after the transaction commits. Reads go
// through View/Get against the currently promoted snapshot.
type Cache struct {
	current atomic.Pointer[immutable.Map[EntityID, State]]

	// pending is the tx-local buffer being mutated during Apply.
	// Only one Append runs at a time (SQLite serializes writers), so
	// a single pending slot is sufficient; a mutex enforces the invariant.
	mu      sync.Mutex
	pending *immutable.Map[EntityID, State]
}

func New() *Cache {
	c := &Cache{}
	empty := immutable.NewMap[EntityID, State](nil)
	c.current.Store(empty)
	return c
}

func (c *Cache) View() *immutable.Map[EntityID, State] {
	return c.current.Load()
}

func (c *Cache) Get(id EntityID) (State, bool) {
	m := c.View()
	v, ok := m.Get(id)
	return v, ok
}

func (c *Cache) Len() int {
	return c.View().Len()
}

// Name implements eventstore.Projector.
func (c *Cache) Name() string { return "state_cache" }

// Apply mutates the pending HAMT. Callers MUST call Promote after the
// enclosing transaction commits (or Discard on rollback).
func (c *Cache) Apply(_ context.Context, _ storage.Tx, e eventstore.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pending == nil {
		c.pending = c.current.Load()
	}

	switch payload := e.Payload.GetKind().(type) {
	case *eventv1.Payload_StateChanged:
		s := State{
			EntityID:   e.Entity,
			UpdatedAt:  e.Timestamp,
			UpdatedBy:  e.Source,
			Attributes: proto.Clone(payload.StateChanged.GetAttributes()).(*entityv1.Attributes),
		}
		c.pending = c.pending.Set(e.Entity, s)

	case *eventv1.Payload_EntityRegistered:
		// Registration seeds capability-level state so reads before first
		// state_changed still return a meaningful Attributes envelope.
		if _, exists := c.pending.Get(e.Entity); !exists {
			c.pending = c.pending.Set(e.Entity, State{
				EntityID:   e.Entity,
				UpdatedAt:  e.Timestamp,
				UpdatedBy:  e.Source,
				Attributes: proto.Clone(payload.EntityRegistered.GetCapabilities()).(*entityv1.Attributes),
			})
		}

	case *eventv1.Payload_EntityUnregistered:
		c.pending = c.pending.Delete(e.Entity)

	default:
		// Events that don't affect cache — ignore.
	}
	return nil
}

// Promote swaps the pending HAMT into current. Called by the store after
// the Append transaction commits.
func (c *Cache) Promote() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending == nil {
		return
	}
	c.current.Store(c.pending)
	c.pending = nil
}

// Discard drops the pending HAMT (used if the transaction rolls back).
func (c *Cache) Discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = nil
}

// Snapshot serialises the current cache state to the snapshots table using
// protobuf+zstd encoding.
func (c *Cache) Snapshot(ctx context.Context, tx storage.Tx) error {
	m := c.View()
	snap := &eventv1.StateCacheSnapshot{
		Ts:       time.Now().UnixNano(),
		Entities: make([]*eventv1.EntityState, 0, m.Len()),
	}

	iter := m.Iterator()
	for !iter.Done() {
		_, v, _ := iter.Next()
		snap.Entities = append(snap.Entities, &eventv1.EntityState{
			EntityId:   v.EntityID,
			UpdatedAt:  v.UpdatedAt.UnixNano(),
			UpdatedBy:  v.UpdatedBy,
			Attributes: v.Attributes,
		})
	}

	// Read MAX(position) so the snapshot row has a meaningful position.
	// Use 1 as fallback when no events exist to ensure a non-zero position.
	var pos int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(position), 1) FROM events").Scan(&pos); err != nil {
		return fmt.Errorf("read max position: %w", err)
	}
	snap.Position = uint64(pos)

	raw, err := proto.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return fmt.Errorf("zstd writer: %w", err)
	}
	compressed := enc.EncodeAll(raw, nil)
	_ = enc.Close()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (position, ts, owner, encoding, state)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(position) DO UPDATE SET
			ts = excluded.ts, owner = excluded.owner,
			encoding = excluded.encoding, state = excluded.state`,
		pos, snap.Ts, c.Name(), "protobuf+zstd", compressed,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// Restore reads the latest snapshot for this cache from the snapshots table
// and populates the current map. Returns the snapshot position (0 if none).
func (c *Cache) Restore(ctx context.Context, tx storage.Tx) (uint64, error) {
	var (
		pos        int64
		encoding   string
		compressed []byte
	)
	err := tx.QueryRowContext(ctx, `
		SELECT position, encoding, state FROM snapshots
		WHERE owner = ? ORDER BY position DESC LIMIT 1`,
		c.Name(),
	).Scan(&pos, &encoding, &compressed)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read snapshot row: %w", err)
	}
	if encoding != "protobuf+zstd" {
		return 0, fmt.Errorf("unknown snapshot encoding %q", encoding)
	}

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return 0, fmt.Errorf("zstd reader: %w", err)
	}
	raw, err := dec.DecodeAll(compressed, nil)
	dec.Close()
	if err != nil {
		return 0, fmt.Errorf("zstd decode: %w", err)
	}

	var snapProto eventv1.StateCacheSnapshot
	if err := proto.Unmarshal(raw, &snapProto); err != nil {
		return 0, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	b := immutable.NewMapBuilder[EntityID, State](nil)
	for _, es := range snapProto.Entities {
		b.Set(es.EntityId, State{
			EntityID:   es.EntityId,
			UpdatedAt:  time.Unix(0, es.UpdatedAt),
			UpdatedBy:  es.UpdatedBy,
			Attributes: es.Attributes,
		})
	}
	c.current.Store(b.Map())
	return uint64(pos), nil
}
