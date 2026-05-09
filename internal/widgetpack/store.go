package widgetpack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrPackNotFound = errors.New("widgetpack: not found")

// InstalledPack describes an installed widget pack.
type InstalledPack struct {
	Name            string
	Version         string
	SHA256          string
	SignatureStatus string // "verified", "unsigned", "invalid"
	SignerIdentity  string
	Classes         []string
	Description     string
	Homepage        string
	License         string
	InstalledAt     time.Time
}

// WatchEvent carries an install/uninstall notification to a Subscribe channel.
// Exactly one of Installed or Uninstalled is non-nil.
type WatchEvent struct {
	Installed   *InstalledPack
	Uninstalled *struct{ Name, Version string }
}

// Store manages the on-disk widget pack registry.
type Store struct {
	root string // <DataDir>/widgets

	mu          sync.RWMutex
	packs       map[string]*InstalledPack // key: name@version
	subscribers map[chan WatchEvent]struct{}
}

// NewStore creates a Store rooted at root. Caller must invoke Load before use.
func NewStore(root string) *Store {
	return &Store{
		root:        root,
		packs:       make(map[string]*InstalledPack),
		subscribers: make(map[chan WatchEvent]struct{}),
	}
}

// Root returns the on-disk root for installed packs.
func (s *Store) Root() string { return s.root }

// Load reads .registry.json and prunes any entries whose pack directory is missing.
func (s *Store) Load(_ context.Context) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", s.root, err)
	}
	regPath := filepath.Join(s.root, ".registry.json")
	data, err := os.ReadFile(regPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", regPath, err)
	}
	var on disk
	if err := json.Unmarshal(data, &on); err != nil {
		return fmt.Errorf("parse %s: %w", regPath, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stale := false
	for _, p := range on.Packs {
		if !s.dirExists(p.Name, p.Version) {
			stale = true
			slog.Warn("widgetpack: pruning stale registry entry", "name", p.Name, "version", p.Version)
			continue
		}
		pp := p
		s.packs[p.Name+"@"+p.Version] = &pp
	}
	if stale {
		return s.persistLocked()
	}
	return nil
}

func (s *Store) dirExists(name, version string) bool {
	info, err := os.Stat(filepath.Join(s.root, name, version))
	return err == nil && info.IsDir()
}

// Add registers a pack and persists. Fires an install event to subscribers.
func (s *Store) Add(_ context.Context, pack InstalledPack) error {
	s.mu.Lock()
	if pack.InstalledAt.IsZero() {
		pack.InstalledAt = time.Now().UTC()
	}
	stored := pack // distinct copy for the map
	s.packs[pack.Name+"@"+pack.Version] = &stored
	if err := s.persistLocked(); err != nil {
		delete(s.packs, pack.Name+"@"+pack.Version)
		s.mu.Unlock()
		return err
	}
	subs := make([]chan WatchEvent, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	snap := pack // distinct allocation for the event payload
	for _, ch := range subs {
		select {
		case ch <- WatchEvent{Installed: &snap}:
		default:
		}
	}
	return nil
}

// Remove unregisters and persists. Fires an uninstall event.
func (s *Store) Remove(_ context.Context, name, version string) error {
	s.mu.Lock()
	key := name + "@" + version
	old, ok := s.packs[key]
	if !ok {
		s.mu.Unlock()
		return ErrPackNotFound
	}
	delete(s.packs, key)
	if err := s.persistLocked(); err != nil {
		s.packs[key] = old
		s.mu.Unlock()
		return err
	}
	subs := make([]chan WatchEvent, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	un := &struct{ Name, Version string }{Name: name, Version: version}
	for _, ch := range subs {
		select {
		case ch <- WatchEvent{Uninstalled: un}:
		default:
		}
	}
	return nil
}

// Get returns a pack snapshot or ErrPackNotFound.
func (s *Store) Get(_ context.Context, name, version string) (*InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.packs[name+"@"+version]
	if !ok {
		return nil, ErrPackNotFound
	}
	cp := *p
	return &cp, nil
}

// List returns all installed packs (snapshots).
func (s *Store) List(_ context.Context) ([]InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InstalledPack, 0, len(s.packs))
	for _, p := range s.packs {
		out = append(out, *p)
	}
	return out, nil
}

// Subscribe registers ch to receive install/uninstall events. Returns an
// unsubscribe func; sends to a full ch are dropped (non-blocking).
func (s *Store) Subscribe(ch chan WatchEvent) func() {
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		s.mu.Unlock()
	}
}

// PackClass is a snapshot of one widget class for the dashboard catalog.
type PackClass struct {
	Name       string
	BundleURL  string
	BundleHash string
}

// PackView is a snapshot of one installed pack's contributions to the catalog.
type PackView struct {
	Name    string
	Version string
	Classes []PackClass
}

// ClassesView returns a snapshot of all installed packs in a shape suitable
// for joining into the dashboard catalog. Caller must not mutate the result.
func (s *Store) ClassesView() []PackView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PackView, 0, len(s.packs))
	for _, p := range s.packs {
		classes := make([]PackClass, 0, len(p.Classes))
		for _, c := range p.Classes {
			classes = append(classes, PackClass{
				Name:       c,
				BundleURL:  "/widgets/" + p.Name + "/" + p.Version + "/bundle.js?h=" + p.SHA256,
				BundleHash: p.SHA256,
			})
		}
		out = append(out, PackView{Name: p.Name, Version: p.Version, Classes: classes})
	}
	return out
}

// persistLocked writes .registry.json atomically. Caller holds s.mu.
func (s *Store) persistLocked() error {
	on := disk{Packs: make([]InstalledPack, 0, len(s.packs))}
	for _, p := range s.packs {
		on.Packs = append(on.Packs, *p)
	}
	data, err := json.MarshalIndent(on, "", "  ")
	if err != nil {
		return err
	}
	regPath := filepath.Join(s.root, ".registry.json")
	tmp := regPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, regPath)
}

type disk struct {
	Packs []InstalledPack `json:"packs"`
}
