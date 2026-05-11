package display

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sixDigitRE = regexp.MustCompile(`^\d{6}$`)

func TestPairCodeStore_IssueFormat(t *testing.T) {
	s := NewPairCodeStore()
	code, exp, err := s.Issue()
	require.NoError(t, err)
	assert.Regexp(t, sixDigitRE, code, "code must be 6 digits")
	assert.True(t, exp.After(time.Now()), "expiry must be in the future")
	assert.True(t, exp.Before(time.Now().Add(6*time.Minute)), "expiry must be within 6 minutes")
}

func TestPairCodeStore_RedeemOK(t *testing.T) {
	s := NewPairCodeStore()
	code, _, err := s.Issue()
	require.NoError(t, err)
	assert.NoError(t, s.Redeem(code))
}

func TestPairCodeStore_DoubleRedeem(t *testing.T) {
	s := NewPairCodeStore()
	code, _, err := s.Issue()
	require.NoError(t, err)
	require.NoError(t, s.Redeem(code))
	// Second redeem must fail with not-found (code was removed).
	err = s.Redeem(code)
	assert.ErrorIs(t, err, errCodeNotFound)
}

func TestPairCodeStore_ExpiredCode(t *testing.T) {
	s := NewPairCodeStore()
	code, err := generateCode()
	require.NoError(t, err)
	// Insert with an already-expired entry.
	s.mu.Lock()
	s.entries[code] = pairEntry{expiresAt: time.Now().Add(-time.Second)}
	s.mu.Unlock()
	err = s.Redeem(code)
	assert.ErrorIs(t, err, errCodeExpired)
}

func TestPairCodeStore_NotFoundCode(t *testing.T) {
	s := NewPairCodeStore()
	err := s.Redeem("000000")
	assert.ErrorIs(t, err, errCodeNotFound)
}

func TestPairCodeStore_Sweep(t *testing.T) {
	s := NewPairCodeStore()
	code, err := generateCode()
	require.NoError(t, err)
	s.mu.Lock()
	s.entries[code] = pairEntry{expiresAt: time.Now().Add(-time.Second)}
	s.mu.Unlock()

	s.Sweep()

	s.mu.Lock()
	_, ok := s.entries[code]
	s.mu.Unlock()
	assert.False(t, ok, "sweep should have removed expired entry")
}

func TestPairCodeStore_MultipleCodes(t *testing.T) {
	s := NewPairCodeStore()
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		code, _, err := s.Issue()
		require.NoError(t, err)
		assert.Regexp(t, sixDigitRE, code, fmt.Sprintf("code %d must be 6 digits", i))
		seen[code] = true
	}
	// All codes should be present in store.
	s.mu.Lock()
	count := len(s.entries)
	s.mu.Unlock()
	assert.Equal(t, len(seen), count)
}
