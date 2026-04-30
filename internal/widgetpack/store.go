package widgetpack

import (
	"context"
	"errors"
	"sync"
)

// ErrPackNotFound is returned when a pack is not found.
var ErrPackNotFound = errors.New("widgetpack: not found")

// InstalledPack describes an installed widget pack.
type InstalledPack struct {
	Name            string
	Version         string
	SHA256          string
	SignatureStatus string // "verified", "unsigned", "invalid", "expired"
}

// Store manages the on-disk widget pack registry.
// The current implementation is in-memory; production will use SQLite.
type Store struct {
	mu    sync.RWMutex
	packs map[string]*InstalledPack // key: name@version
}

// NewStore creates a new in-memory store.
func NewStore() *Store {
	return &Store{packs: make(map[string]*InstalledPack)}
}

// Add registers an installed pack.
func (s *Store) Add(_ context.Context, pack InstalledPack) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packs[pack.Name+"@"+pack.Version] = &pack
	return nil
}

// Get retrieves an installed pack by name and version.
func (s *Store) Get(_ context.Context, name, version string) (*InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.packs[name+"@"+version]
	if !ok {
		return nil, ErrPackNotFound
	}
	return p, nil
}

// List returns all installed packs.
func (s *Store) List(_ context.Context) ([]InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InstalledPack, 0, len(s.packs))
	for _, p := range s.packs {
		out = append(out, *p)
	}
	return out, nil
}

// Remove unregisters a pack.
func (s *Store) Remove(_ context.Context, name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := name + "@" + version
	if _, ok := s.packs[key]; !ok {
		return ErrPackNotFound
	}
	delete(s.packs, key)
	return nil
}
