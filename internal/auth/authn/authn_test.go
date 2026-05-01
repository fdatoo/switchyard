package authn_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/authn"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func setupAuthnDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	_, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	return db
}

func newSessionStore(db *sql.DB) *sessions.Store {
	return sessions.New(db, sessions.Config{
		Key:         []byte("test-hmac-key-32-bytes-long-xxxx"),
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  30 * 24 * time.Hour,
		RefreshIdle: 14 * 24 * time.Hour,
		AccessName:  "gohome_access",
		RefreshName: "gohome_refresh",
	})
}

func TestBearer_HappyPath(t *testing.T) {
	db := setupAuthnDB(t)
	ts := credentials.NewTokens(db)
	plaintext, _, err := ts.Issue(context.Background(), credentials.IssueTokenInput{UserSlug: "fdatoo", Scope: []byte{}})
	require.NoError(t, err)

	a := authn.NewBearer(ts)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	p, err := a.Authenticate(context.Background(), authn.RequestFromHTTP(req))
	require.NoError(t, err)
	require.Equal(t, "user:fdatoo", p.ID)
	require.Equal(t, "user", p.Kind)
	require.Equal(t, "token", p.Metadata["auth_method"])
}

func TestBearer_MalformedReturnsNotApplicable(t *testing.T) {
	db := setupAuthnDB(t)
	a := authn.NewBearer(credentials.NewTokens(db))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Basic something")
	_, err := a.Authenticate(context.Background(), authn.RequestFromHTTP(req))
	require.ErrorIs(t, err, auth.ErrNotApplicable)
}

func TestBearer_ExpiredReturnsUnauthenticated(t *testing.T) {
	db := setupAuthnDB(t)
	ts := credentials.NewTokens(db)
	plaintext, _, err := ts.Issue(context.Background(), credentials.IssueTokenInput{UserSlug: "fdatoo", Scope: []byte{}, TTL: -time.Second})
	require.NoError(t, err)

	a := authn.NewBearer(ts)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	_, err = a.Authenticate(context.Background(), authn.RequestFromHTTP(req))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestCookie_HappyPath(t *testing.T) {
	db := setupAuthnDB(t)
	ss := newSessionStore(db)
	rec := httptest.NewRecorder()
	_, err := ss.Issue(context.Background(), rec, sessions.IssueInput{UserSlug: "fdatoo", AuthMethod: "passkey"})
	require.NoError(t, err)

	a := authn.NewSessionCookie(ss)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	p, err := a.Authenticate(context.Background(), authn.RequestFromHTTP(req))
	require.NoError(t, err)
	require.Equal(t, "user:fdatoo", p.ID)
	require.Equal(t, "passkey", p.Metadata["auth_method"])
}

func TestChain_BearerWinsOverCookie(t *testing.T) {
	db := setupAuthnDB(t)
	ts := credentials.NewTokens(db)
	plaintext, _, err := ts.Issue(context.Background(), credentials.IssueTokenInput{UserSlug: "fdatoo", Scope: []byte{}})
	require.NoError(t, err)
	ss := newSessionStore(db)
	rec := httptest.NewRecorder()
	_, err = ss.Issue(context.Background(), rec, sessions.IssueInput{UserSlug: "milo", AuthMethod: "passkey"})
	require.NoError(t, err)

	chain := auth.Chain(authn.NewBearer(ts), authn.NewSessionCookie(ss), auth.RejectAll{})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	p, err := chain.Authenticate(context.Background(), authn.RequestFromHTTP(req))
	require.NoError(t, err)
	require.Equal(t, "user:fdatoo", p.ID) // bearer wins
}
