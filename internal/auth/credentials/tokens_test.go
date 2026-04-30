package credentials_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/auth/credentials"
)

func TestTokens_IssueThenVerify(t *testing.T) {
	db := setupAuthDB(t)
	tok := credentials.NewTokens(db)
	ctx := context.Background()

	plaintext, tokenID, err := tok.Issue(ctx, credentials.IssueTokenInput{
		UserSlug: "fdatoo",
		Label:    "claude-desktop",
		IssuedBy: "system:test",
		Scope:    []byte{1, 2, 3},
		TTL:      time.Hour,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(plaintext, "gohome_"), "plaintext must start with gohome_")
	require.Contains(t, plaintext, tokenID)

	lk, err := tok.Verify(ctx, plaintext)
	require.NoError(t, err)
	require.Equal(t, "fdatoo", lk.UserSlug)
	require.Equal(t, "claude-desktop", lk.Label)
}

func TestTokens_Verify_RejectsRevoked(t *testing.T) {
	db := setupAuthDB(t)
	tok := credentials.NewTokens(db)
	ctx := context.Background()

	plaintext, tokenID, err := tok.Issue(ctx, credentials.IssueTokenInput{
		UserSlug: "fdatoo",
		Label:    "test",
		IssuedBy: "system:test",
		Scope:    []byte{},
		TTL:      time.Hour,
	})
	require.NoError(t, err)

	require.NoError(t, tok.Revoke(ctx, tokenID, "system:test"))

	_, err = tok.Verify(ctx, plaintext)
	require.ErrorIs(t, err, credentials.ErrTokenRevoked)
}

func TestTokens_Verify_RejectsExpired(t *testing.T) {
	db := setupAuthDB(t)
	tok := credentials.NewTokens(db)
	ctx := context.Background()

	// negative TTL means it expired immediately
	plaintext, _, err := tok.Issue(ctx, credentials.IssueTokenInput{
		UserSlug: "fdatoo",
		Label:    "test",
		IssuedBy: "system:test",
		Scope:    []byte{},
		TTL:      -time.Second,
	})
	require.NoError(t, err)

	_, err = tok.Verify(ctx, plaintext)
	require.ErrorIs(t, err, credentials.ErrTokenExpired)
}

func TestTokens_Verify_RejectsBadSecret(t *testing.T) {
	db := setupAuthDB(t)
	tok := credentials.NewTokens(db)
	ctx := context.Background()

	plaintext, _, err := tok.Issue(ctx, credentials.IssueTokenInput{
		UserSlug: "fdatoo",
		Label:    "test",
		IssuedBy: "system:test",
		Scope:    []byte{},
		TTL:      time.Hour,
	})
	require.NoError(t, err)

	// tamper last 4 chars
	tampered := plaintext[:len(plaintext)-4] + "XXXX"

	_, err = tok.Verify(ctx, tampered)
	require.ErrorIs(t, err, credentials.ErrTokenInvalid)
}

func TestTokens_Verify_RejectsUnknownPrefix(t *testing.T) {
	db := setupAuthDB(t)
	tok := credentials.NewTokens(db)
	ctx := context.Background()

	_, err := tok.Verify(ctx, "wat-not-a-token")
	require.ErrorIs(t, err, credentials.ErrTokenInvalid)
}
