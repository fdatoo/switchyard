package credentials

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"time"
)

// IntentRegisterPasskey enrolls a new WebAuthn credential for a user.
const IntentRegisterPasskey = "register_passkey"

// IntentSetPassword enrolls or replaces a user's password credential.
const IntentSetPassword = "set_password"

// ErrEnrollmentInvalid means the presented enrollment token cannot be decoded or found.
var ErrEnrollmentInvalid = errors.New("credentials: enrollment token invalid")

// ErrEnrollmentExpired means the enrollment token existed but passed its expiry.
var ErrEnrollmentExpired = errors.New("credentials: enrollment token expired")

// ErrEnrollmentConsumed means the token has already been redeemed.
var ErrEnrollmentConsumed = errors.New("credentials: enrollment token already used")

// Enrollment stores and redeems one-time credential enrollment tokens.
type Enrollment struct{ db *sql.DB }

// NewEnrollment returns an enrollment-token store backed by db.
func NewEnrollment(db *sql.DB) *Enrollment {
	return &Enrollment{db: db}
}

// EnrollmentLookup is the identity and action unlocked by a redeemed token.
type EnrollmentLookup struct {
	UserSlug string
	Intent   string
}

// Mint creates a one-time plaintext token and stores only its hash.
func (e *Enrollment) Mint(ctx context.Context, userSlug, intent string, ttl time.Duration) (string, error) {
	if intent != IntentRegisterPasskey && intent != IntentSetPassword {
		return "", errors.New("credentials: invalid intent")
	}

	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}

	plaintext := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)

	sum := sha256.Sum256(secret)
	tokenHash := hex.EncodeToString(sum[:])

	expiresAt := time.Now().Add(ttl).Unix()

	_, err := e.db.ExecContext(ctx, `
		INSERT INTO auth_enrollment_tokens (token_hash, user_slug, intent, expires_at)
		VALUES (?, ?, ?, ?)`,
		tokenHash, userSlug, intent, expiresAt)
	if err != nil {
		return "", err
	}

	return plaintext, nil
}

// Redeem validates a plaintext token, marks it consumed, and returns its lookup data.
func (e *Enrollment) Redeem(ctx context.Context, plaintext string) (EnrollmentLookup, error) {
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(plaintext)
	if err != nil {
		return EnrollmentLookup{}, ErrEnrollmentInvalid
	}

	sum := sha256.Sum256(secret)
	tokenHash := hex.EncodeToString(sum[:])

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return EnrollmentLookup{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		lk         EnrollmentLookup
		expiresAt  int64
		consumedAt *int64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT user_slug, intent, expires_at, consumed_at
		FROM auth_enrollment_tokens
		WHERE token_hash = ?`,
		tokenHash).Scan(&lk.UserSlug, &lk.Intent, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EnrollmentLookup{}, ErrEnrollmentInvalid
	}
	if err != nil {
		return EnrollmentLookup{}, err
	}

	if consumedAt != nil {
		return EnrollmentLookup{}, ErrEnrollmentConsumed
	}

	if expiresAt < time.Now().Unix() {
		return EnrollmentLookup{}, ErrEnrollmentExpired
	}

	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `
		UPDATE auth_enrollment_tokens SET consumed_at = ? WHERE token_hash = ?`,
		now, tokenHash)
	if err != nil {
		return EnrollmentLookup{}, err
	}

	if err := tx.Commit(); err != nil {
		return EnrollmentLookup{}, err
	}

	return lk, nil
}

// Sweep deletes expired or long-consumed enrollment tokens before cutoff.
func (e *Enrollment) Sweep(ctx context.Context, cutoff time.Time) error {
	cutoffUnix := cutoff.Unix()
	_, err := e.db.ExecContext(ctx, `
		DELETE FROM auth_enrollment_tokens
		WHERE expires_at < ? OR (consumed_at IS NOT NULL AND consumed_at < ?)`,
		cutoffUnix, cutoffUnix)
	return err
}
