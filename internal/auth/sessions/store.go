// Package sessions stores cookie-based browser sessions with HMAC-signed
// access cookies and rotating refresh cookies. Replay-detection on refresh
// revokes the entire session.
package sessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

// Config holds tunable parameters for the session store.
type Config struct {
	Key         []byte
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	RefreshIdle time.Duration
	AccessName  string
	RefreshName string
}

// Store manages browser sessions backed by the auth_sessions table.
type Store struct {
	db  *sql.DB
	cfg Config
}

// New returns a Store using the given DB and config. Migrations must already be applied.
func New(db *sql.DB, cfg Config) *Store { return &Store{db: db, cfg: cfg} }

// CookieName returns the name of the access cookie this store reads.
func (s *Store) CookieName() string { return s.cfg.AccessName }

// IssueInput carries the caller-supplied context for creating a new session.
type IssueInput struct {
	UserSlug   string
	AuthMethod string
	RemoteIP   string
	UserAgent  string
}

// SessionData is returned after a successful issue or refresh.
type SessionData struct {
	SessionID  string
	UserSlug   string
	AuthMethod string
}

// Principal is returned by VerifyAccess on success.
type Principal struct {
	UserSlug   string
	SessionID  string
	AuthMethod string
}

// Issue creates a new session row, writes access and refresh cookies, and returns SessionData.
func (s *Store) Issue(ctx context.Context, w http.ResponseWriter, in IssueInput) (SessionData, error) {
	sid := ulid.Make().String()
	secret := newSecret()
	sum := sha256.Sum256([]byte(secret))
	now := time.Now()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (session_id, user_slug, auth_method, refresh_hash, issued_at, refresh_ttl_at, refresh_idle_at, user_agent, remote_ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sid, in.UserSlug, in.AuthMethod, hex.EncodeToString(sum[:]),
		now.Unix(), deadline(now, s.cfg.RefreshTTL), deadline(now, s.cfg.RefreshIdle),
		in.UserAgent, in.RemoteIP); err != nil {
		return SessionData{}, err
	}
	s.writeCookies(w, sid, in.UserSlug, in.AuthMethod, secret, now)
	return SessionData{SessionID: sid, UserSlug: in.UserSlug, AuthMethod: in.AuthMethod}, nil
}

// VerifyAccess validates the access cookie and confirms the session exists in the DB.
func (s *Store) VerifyAccess(ctx context.Context, r *http.Request) (Principal, error) {
	c, err := r.Cookie(s.cfg.AccessName)
	if err != nil {
		return Principal{}, ErrSessionInvalid
	}
	claim, err := decodeAccessCookie(c.Value, s.cfg.Key)
	if err != nil {
		return Principal{}, err
	}
	if time.Now().Unix() >= claim.Exp {
		return Principal{}, ErrSessionExpired
	}
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM auth_sessions WHERE session_id = ?`, claim.SessionID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrSessionInvalid
		}
		return Principal{}, err
	}
	return Principal{UserSlug: claim.UserSlug, SessionID: claim.SessionID, AuthMethod: claim.AuthMethod}, nil
}

// Refresh validates the refresh cookie, rotates the secret, and writes new cookies.
func (s *Store) Refresh(ctx context.Context, w http.ResponseWriter, r *http.Request) (SessionData, error) {
	c, err := r.Cookie(s.cfg.RefreshName)
	if err != nil {
		return SessionData{}, ErrSessionInvalid
	}
	rc, err := decodeRefreshCookie(c.Value)
	if err != nil {
		return SessionData{}, err
	}
	sum := sha256.Sum256([]byte(rc.RefreshSecret))
	presented := hex.EncodeToString(sum[:])

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionData{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		userSlug      string
		authMethod    string
		stored        string
		ttlAt, idleAt int64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT user_slug, auth_method, refresh_hash, refresh_ttl_at, refresh_idle_at
		FROM auth_sessions WHERE session_id = ?`, rc.SessionID).
		Scan(&userSlug, &authMethod, &stored, &ttlAt, &idleAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionData{}, ErrSessionInvalid
	}
	if err != nil {
		return SessionData{}, err
	}
	now := time.Now()
	if now.Unix() >= ttlAt || now.Unix() >= idleAt {
		return SessionData{}, ErrSessionExpired
	}
	if subtle.ConstantTimeCompare([]byte(presented), []byte(stored)) != 1 {
		// Replay detected — revoke entire session.
		if _, derr := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE session_id = ?`, rc.SessionID); derr != nil {
			return SessionData{}, derr
		}
		if err := tx.Commit(); err != nil {
			return SessionData{}, err
		}
		return SessionData{}, ErrSessionReplay
	}
	// Rotate refresh secret + idle deadline.
	newSecretStr := newSecret()
	newSum := sha256.Sum256([]byte(newSecretStr))
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_sessions
		SET refresh_hash = ?, refresh_idle_at = ?
		WHERE session_id = ?`,
		hex.EncodeToString(newSum[:]),
		deadline(now, s.cfg.RefreshIdle),
		rc.SessionID); err != nil {
		return SessionData{}, err
	}
	if err := tx.Commit(); err != nil {
		return SessionData{}, err
	}
	s.writeCookies(w, rc.SessionID, userSlug, authMethod, newSecretStr, now)
	return SessionData{SessionID: rc.SessionID, UserSlug: userSlug, AuthMethod: authMethod}, nil
}

// Logout deletes the session row and clears both cookies.
func (s *Store) Logout(ctx context.Context, w http.ResponseWriter, sessionID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	s.clearCookies(w)
	return nil
}

func (s *Store) writeCookies(w http.ResponseWriter, sid, user, authMethod, secret string, now time.Time) {
	accessVal := encodeAccessCookie(AccessClaim{
		SessionID: sid, UserSlug: user, AuthMethod: authMethod, Exp: deadline(now, s.cfg.AccessTTL),
	}, s.cfg.Key)
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.AccessName, Value: accessVal, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		MaxAge: int(s.cfg.AccessTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.RefreshName, Value: encodeRefreshCookie(RefreshCookie{SessionID: sid, RefreshSecret: secret}),
		Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		MaxAge: int(s.cfg.RefreshTTL.Seconds()),
	})
}

func (s *Store) clearCookies(w http.ResponseWriter) {
	for _, name := range []string{s.cfg.AccessName, s.cfg.RefreshName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	}
}

func newSecret() string {
	raw := make([]byte, 24)
	_, _ = rand.Read(raw)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}
