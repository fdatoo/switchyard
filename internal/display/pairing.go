// Package display owns the DisplayService gRPC implementation, pairing flow,
// and fidelity recommender for ambient wall displays.
package display

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const (
	pairCodeLen = 6
	pairCodeTTL = 5 * time.Minute
)

// pairEntry holds an unresolved pairing code.
type pairEntry struct {
	expiresAt time.Time
}

// PairCodeStore is a thread-safe in-memory store of 6-digit pairing codes.
// Entries expire lazily on read and via a background sweep.
type PairCodeStore struct {
	mu      sync.Mutex
	entries map[string]pairEntry
}

// NewPairCodeStore returns an initialised store. Call Sweep() in a goroutine
// to remove expired entries periodically.
func NewPairCodeStore() *PairCodeStore {
	return &PairCodeStore{entries: make(map[string]pairEntry)}
}

// Issue generates a new 6-digit zero-padded code, stores it with TTL, and
// returns the code and its expiry time.
func (s *PairCodeStore) Issue() (code string, expiresAt time.Time, err error) {
	code, err = generateCode()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().Add(pairCodeTTL)
	s.mu.Lock()
	s.entries[code] = pairEntry{expiresAt: expiresAt}
	s.mu.Unlock()
	return code, expiresAt, nil
}

// Redeem validates and atomically removes a code. Returns errCodeNotFound if
// the code is absent, errCodeExpired if it has expired.
func (s *PairCodeStore) Redeem(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[code]
	if !ok {
		return errCodeNotFound
	}
	if time.Now().After(e.expiresAt) {
		delete(s.entries, code)
		return errCodeExpired
	}
	delete(s.entries, code)
	return nil
}

// Sweep removes all expired entries. Intended to be called periodically.
func (s *PairCodeStore) Sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, code)
		}
	}
}

// generateCode returns a cryptographically random 6-digit zero-padded string.
func generateCode() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("display: generate pair code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
