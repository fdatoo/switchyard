package eventstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
)

// QueryOptions bounds and filters a historical event query.
type QueryOptions struct {
	FromPosition uint64
	ToPosition   uint64
	Filter       Filter
	Limit        int
}

// Query reads historical events. Simple range + filter.
func (s *Store) Query(ctx context.Context, q QueryOptions) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT position, ts, kind, entity, source, correlation_id, cause_position, payload
		FROM events
		WHERE position > ? AND (? = 0 OR position <= ?)
		ORDER BY position
		LIMIT ?`,
		q.FromPosition, q.ToPosition, q.ToPosition, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		if q.Filter.Matches(e) {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}

// QueryBySourceSubstring returns events whose source column contains the
// given substring. Used by the automation trace handler to find CommandIssued
// and CommandAck events whose source is stamped as "automation:<id>#<corr>".
func (s *Store) QueryBySourceSubstring(ctx context.Context, substring string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT position, ts, kind, entity, source, correlation_id, cause_position, payload
		FROM events
		WHERE source LIKE ?
		ORDER BY position`,
		"%"+substring+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("query events by source: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanEvent(r interface {
	Scan(...any) error
}) (Event, error) {
	var (
		pos       int64
		tsNanos   int64
		kind      string
		entity    sql.NullString
		source    string
		corrBytes []byte
		cause     sql.NullInt64
		payload   []byte
	)
	if err := r.Scan(&pos, &tsNanos, &kind, &entity, &source, &corrBytes, &cause, &payload); err != nil {
		return Event{}, err
	}
	e := Event{
		Position:  uint64(pos),
		Timestamp: time.Unix(0, tsNanos),
		Kind:      kind,
		Entity:    entity.String,
		Source:    source,
	}
	if len(corrBytes) == 16 {
		_ = e.CorrelationID.UnmarshalBinary(corrBytes)
	}
	if cause.Valid {
		e.CausePosition = uint64(cause.Int64)
	}
	e.Payload = &eventv1.Payload{}
	if err := proto.Unmarshal(payload, e.Payload); err != nil {
		return Event{}, fmt.Errorf("unmarshal payload: %w", err)
	}
	return e, nil
}
