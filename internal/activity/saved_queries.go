package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "github.com/fdatoo/switchyard/gen/switchyard/activity/v1"
)

// savedQueryRecord is the on-disk JSON representation of a saved query.
type savedQueryRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Filter    string    `json:"filter"`
	Cron      string    `json:"cron,omitempty"`
	LastRun   time.Time `json:"last_run,omitempty"`
	NextRun   time.Time `json:"next_run,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (r savedQueryRecord) toProto() *activityv1.SavedQuery {
	q := &activityv1.SavedQuery{
		Id:        r.ID,
		Name:      r.Name,
		Filter:    r.Filter,
		Cron:      r.Cron,
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
	if !r.LastRun.IsZero() {
		q.LastRun = timestamppb.New(r.LastRun)
	}
	if !r.NextRun.IsZero() {
		q.NextRun = timestamppb.New(r.NextRun)
	}
	return q
}

// SavedQueryStore is a simple file-backed store for saved queries.
// Each query is stored as a JSON file in a dedicated directory.
type SavedQueryStore struct {
	dir string
	mu  sync.RWMutex
}

// NewSavedQueryStore creates a SavedQueryStore backed by the given directory.
// The directory is created on first use.
func NewSavedQueryStore(dir string) *SavedQueryStore {
	return &SavedQueryStore{dir: dir}
}

func (s *SavedQueryStore) ensureDir() error {
	return os.MkdirAll(s.dir, 0o700)
}

func (s *SavedQueryStore) filePath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Save persists a new saved query and returns it.
func (s *SavedQueryStore) Save(_ context.Context, name, filter, cron string) (*activityv1.SavedQuery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return nil, fmt.Errorf("saved queries: mkdir: %w", err)
	}

	rec := savedQueryRecord{
		ID:        uuid.New().String(),
		Name:      name,
		Filter:    filter,
		Cron:      cron,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("saved queries: marshal: %w", err)
	}

	if err := os.WriteFile(s.filePath(rec.ID), data, 0o600); err != nil {
		return nil, fmt.Errorf("saved queries: write: %w", err)
	}

	return rec.toProto(), nil
}

// List returns all saved queries in creation order.
func (s *SavedQueryStore) List(_ context.Context) ([]*activityv1.SavedQuery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.ensureDir(); err != nil {
		return nil, fmt.Errorf("saved queries: mkdir: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("saved queries: readdir: %w", err)
	}

	var queries []*activityv1.SavedQuery
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue // skip corrupt entries
		}

		var rec savedQueryRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		queries = append(queries, rec.toProto())
	}

	return queries, nil
}

// Delete removes a saved query by ID.
func (s *SavedQueryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.filePath(id))
	if os.IsNotExist(err) {
		return fmt.Errorf("saved_query %q not found", id)
	}
	return err
}
