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

const IntentRegisterPasskey = "register_passkey"
const IntentSetPassword = "set_password"

var ErrEnrollmentInvalid = errors.New("credentials: enrollment token invalid")
var ErrEnrollmentExpired = errors.New("credentials: enrollment token expired")
var ErrEnrollmentConsumed = errors.New("credentials: enrollment token already used")

type Enrollment struct{ db *sql.DB }

func NewEnrollment(db *sql.DB) *Enrollment {
	return &Enrollment{db: db}
}

type EnrollmentLookup struct {
	UserSlug string
	Intent   string
}

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

func (e *Enrollment) Sweep(ctx context.Context, cutoff time.Time) error {
	cutoffUnix := cutoff.Unix()
	_, err := e.db.ExecContext(ctx, `
		DELETE FROM auth_enrollment_tokens
		WHERE expires_at < ? OR (consumed_at IS NOT NULL AND consumed_at < ?)`,
		cutoffUnix, cutoffUnix)
	return err
}
