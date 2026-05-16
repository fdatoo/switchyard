package credentials

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"github.com/oklog/ulid/v2"
)

// ErrPasskeyUnknown means the credential id is not registered.
var ErrPasskeyUnknown = errors.New("credentials: passkey unknown")

// ErrSignCountRegression means the authenticator reported a cloned credential risk.
var ErrSignCountRegression = errors.New("credentials: passkey sign-count regression")

// Passkeys stores and verifies WebAuthn passkey credentials.
type Passkeys struct {
	db *sql.DB
	w  *wa.WebAuthn
}

// NewPasskeys returns a WebAuthn credential store backed by db.
func NewPasskeys(db *sql.DB, w *wa.WebAuthn) *Passkeys {
	return &Passkeys{db: db, w: w}
}

// Passkey is a registered WebAuthn credential.
type Passkey struct {
	CredentialID []byte
	UserSlug     string
	PublicKey    []byte
	SignCount    uint32
	Label        string
	RegisteredAt time.Time
	LastUsedAt   *time.Time
}

// userAdapter implements wa.User for a slug-keyed user backed by auth_passkeys.
type userAdapter struct {
	slug        string
	displayName string
	creds       []wa.Credential
}

func (u *userAdapter) WebAuthnID() []byte                   { return []byte(u.slug) }
func (u *userAdapter) WebAuthnName() string                 { return u.slug }
func (u *userAdapter) WebAuthnDisplayName() string          { return u.displayName }
func (u *userAdapter) WebAuthnCredentials() []wa.Credential { return u.creds }

func (p *Passkeys) loadUser(ctx context.Context, slug string) (*userAdapter, error) {
	displayName := slug
	err := p.db.QueryRowContext(ctx,
		`SELECT display_name FROM auth_users WHERE slug = ?`, slug).Scan(&displayName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	creds, err := p.loadCredentials(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &userAdapter{slug: slug, displayName: displayName, creds: creds}, nil
}

func (p *Passkeys) loadCredentials(ctx context.Context, slug string) ([]wa.Credential, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT credential_id, public_key, sign_count
		FROM auth_passkeys
		WHERE user_slug = ?`, slug)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []wa.Credential
	for rows.Next() {
		var (
			id, pk    []byte
			signCount int64
		)
		if err := rows.Scan(&id, &pk, &signCount); err != nil {
			return nil, err
		}
		out = append(out, wa.Credential{
			ID:        id,
			PublicKey: pk,
			Authenticator: wa.Authenticator{
				SignCount: uint32(signCount),
			},
		})
	}
	return out, rows.Err()
}

// BeginRegistration starts a registration ceremony for a slug, returning the
// creation options to send to the client and session data to persist for the
// finish step.
func (p *Passkeys) BeginRegistration(ctx context.Context, slug, displayName string) (*protocol.CredentialCreation, *wa.SessionData, error) {
	user, err := p.loadUser(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	if displayName != "" {
		user.displayName = displayName
	}
	opts := []wa.RegistrationOption{
		wa.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		wa.WithExclusions(wa.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
	}
	return p.w.BeginRegistration(user, opts...)
}

// FinishRegistration verifies the parsed registration response, persists the
// resulting credential, and returns the stored Passkey.
func (p *Passkeys) FinishRegistration(ctx context.Context, slug, label string, sd *wa.SessionData, response *protocol.ParsedCredentialCreationData) (Passkey, error) {
	if sd == nil {
		return Passkey{}, errors.New("credentials: nil session data")
	}
	user, err := p.loadUser(ctx, slug)
	if err != nil {
		return Passkey{}, err
	}
	cred, err := p.w.CreateCredential(user, *sd, response)
	if err != nil {
		return Passkey{}, fmt.Errorf("create credential: %w", err)
	}

	now := time.Now()
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO auth_passkeys (credential_id, user_slug, public_key, sign_count, label, registered_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		cred.ID, slug, cred.PublicKey, int64(cred.Authenticator.SignCount), label, now.Unix())
	if err != nil {
		return Passkey{}, err
	}

	return Passkey{
		CredentialID: cred.ID,
		UserSlug:     slug,
		PublicKey:    cred.PublicKey,
		SignCount:    cred.Authenticator.SignCount,
		Label:        label,
		RegisteredAt: now,
	}, nil
}

// BeginLogin starts a discoverable (resident-key) login ceremony.
func (p *Passkeys) BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, *wa.SessionData, error) {
	_ = ctx
	return p.w.BeginDiscoverableLogin()
}

// FinishLogin validates the parsed assertion against the stored credential,
// enforces sign-count regression, and bumps last_used_at + sign_count.
// Returns the user slug owning the credential.
func (p *Passkeys) FinishLogin(ctx context.Context, sd *wa.SessionData, response *protocol.ParsedCredentialAssertionData) (string, error) {
	if sd == nil {
		return "", errors.New("credentials: nil session data")
	}

	// userHandle is set to []byte(slug) by WebAuthnID() during registration;
	// using it here avoids a separate pre-fetch and an inconsistency window.
	var slug string
	handler := func(_, userHandle []byte) (wa.User, error) {
		slug = string(userHandle)
		return p.loadUser(ctx, slug)
	}

	_, cred, err := p.w.ValidatePasskeyLogin(handler, *sd, response)
	if err != nil {
		return "", err
	}

	if cred.Authenticator.CloneWarning {
		return "", ErrSignCountRegression
	}

	now := time.Now().Unix()
	_, err = p.db.ExecContext(ctx, `
		UPDATE auth_passkeys
		SET sign_count = ?, last_used_at = ?
		WHERE credential_id = ?`,
		int64(cred.Authenticator.SignCount), now, response.RawID)
	if err != nil {
		return "", err
	}

	return slug, nil
}

// List returns every Passkey registered for slug, oldest first.
func (p *Passkeys) List(ctx context.Context, slug string) ([]Passkey, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT credential_id, user_slug, public_key, sign_count, label, registered_at, last_used_at
		FROM auth_passkeys
		WHERE user_slug = ?
		ORDER BY registered_at ASC`, slug)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Passkey
	for rows.Next() {
		var (
			pk           Passkey
			signCount    int64
			registeredAt int64
			lastUsedAt   sql.NullInt64
			label        sql.NullString
		)
		if err := rows.Scan(&pk.CredentialID, &pk.UserSlug, &pk.PublicKey,
			&signCount, &label, &registeredAt, &lastUsedAt); err != nil {
			return nil, err
		}
		pk.SignCount = uint32(signCount)
		pk.Label = label.String
		pk.RegisteredAt = time.Unix(registeredAt, 0)
		if lastUsedAt.Valid {
			t := time.Unix(lastUsedAt.Int64, 0)
			pk.LastUsedAt = &t
		}
		out = append(out, pk)
	}
	return out, rows.Err()
}

// Remove deletes the passkey with the given credential ID. It is not an error
// if no such row exists.
func (p *Passkeys) Remove(ctx context.Context, credID []byte) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM auth_passkeys WHERE credential_id = ?`, credID)
	return err
}

// ChallengeStore holds per-session WebAuthn challenges between
// StartWebAuthnChallenge and Login. In-process only; expiry by TTL.
type ChallengeStore struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]challengeEntry // key: sessionID + ":" + id
}

type challengeEntry struct {
	payload []byte
	expires time.Time
}

// ErrChallengeNotFound is returned when a challenge id is missing, expired,
// or belongs to a different session.
var ErrChallengeNotFound = errors.New("webauthn: challenge not found or expired")

// NewChallengeStore creates a new store with the given TTL per challenge.
func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	return &ChallengeStore{ttl: ttl, items: make(map[string]challengeEntry)}
}

// Store records a challenge against a session id and returns an opaque id
// the client echoes back on Login.
func (s *ChallengeStore) Store(_ context.Context, sessionID string, payload []byte) (string, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	s.items[sessionID+":"+id] = challengeEntry{payload: payload, expires: time.Now().Add(s.ttl)}
	return id, nil
}

// Consume retrieves and removes a challenge. Returns ErrChallengeNotFound on
// any mismatch (session, id, or expiry) so the caller cannot distinguish
// missing from expired (timing safe).
func (s *ChallengeStore) Consume(_ context.Context, sessionID, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionID + ":" + id
	entry, ok := s.items[key]
	if !ok || time.Now().After(entry.expires) {
		delete(s.items, key)
		return nil, ErrChallengeNotFound
	}
	delete(s.items, key)
	return entry.payload, nil
}

func (s *ChallengeStore) gcLocked() {
	now := time.Now()
	for k, v := range s.items {
		if now.After(v.expires) {
			delete(s.items, k)
		}
	}
}
