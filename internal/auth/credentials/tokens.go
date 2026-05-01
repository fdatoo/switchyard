package credentials

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var ErrTokenInvalid = errors.New("credentials: token invalid")
var ErrTokenRevoked = errors.New("credentials: token revoked")
var ErrTokenExpired = errors.New("credentials: token expired")

type Tokens struct{ db *sql.DB }

func NewTokens(db *sql.DB) *Tokens { return &Tokens{db: db} }

type IssueTokenInput struct {
	UserSlug string
	Label    string
	IssuedBy string
	Scope    []byte
	TTL      time.Duration // 0 = never expires; negative = born-expired (for testing)
}

type Lookup struct {
	TokenID  string
	UserSlug string
	Label    string
	Scope    []byte
	IssuedBy string
}

func (t *Tokens) Issue(ctx context.Context, in IssueTokenInput) (plaintext, tokenID string, err error) {
	secret := make([]byte, 24)
	if _, err = rand.Read(secret); err != nil {
		return "", "", err
	}

	tokenID = ulid.Make().String()
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	plaintext = "switchyard_" + tokenID + "_" + encoded

	sum := sha256.Sum256(secret)
	hashHex := hex.EncodeToString(sum[:])

	now := time.Now().Unix()
	var expiresAt *int64
	if in.TTL != 0 {
		exp := time.Now().Add(in.TTL).Unix()
		expiresAt = &exp
	}

	_, err = t.db.ExecContext(ctx, `
		INSERT INTO auth_tokens
			(token_id, user_slug, label, hash_b64, scope_blob, issued_at, issued_by, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tokenID, in.UserSlug, in.Label, hashHex, in.Scope, now, in.IssuedBy, expiresAt)
	if err != nil {
		return "", "", err
	}
	return plaintext, tokenID, nil
}

func (t *Tokens) Verify(ctx context.Context, plaintext string) (Lookup, error) {
	parts := strings.SplitN(plaintext, "_", 3)
	if len(parts) != 3 || parts[0] != "switchyard" {
		return Lookup{}, ErrTokenInvalid
	}
	tokenID := parts[1]
	encodedSecret := parts[2]

	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encodedSecret)
	if err != nil {
		return Lookup{}, ErrTokenInvalid
	}

	var (
		lk         Lookup
		storedHash string
		revokedAt  *int64
		expiresAt  *int64
	)
	err = t.db.QueryRowContext(ctx, `
		SELECT token_id, user_slug, label, hash_b64, scope_blob, issued_by, revoked_at, expires_at
		FROM auth_tokens
		WHERE token_id = ?`,
		tokenID).Scan(
		&lk.TokenID, &lk.UserSlug, &lk.Label, &storedHash, &lk.Scope, &lk.IssuedBy,
		&revokedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Lookup{}, ErrTokenInvalid
	}
	if err != nil {
		return Lookup{}, err
	}

	if revokedAt != nil {
		return Lookup{}, ErrTokenRevoked
	}
	if expiresAt != nil && *expiresAt < time.Now().Unix() {
		return Lookup{}, ErrTokenExpired
	}

	sum := sha256.Sum256(secret)
	storedBytes, err := hex.DecodeString(storedHash)
	if err != nil {
		return Lookup{}, ErrTokenInvalid
	}
	if subtle.ConstantTimeCompare(sum[:], storedBytes) != 1 {
		return Lookup{}, ErrTokenInvalid
	}

	return lk, nil
}

func (t *Tokens) Revoke(ctx context.Context, tokenID, byPrincipal string) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE auth_tokens SET revoked_at = ? WHERE token_id = ? AND revoked_at IS NULL`,
		time.Now().Unix(), tokenID)
	return err
}

func (t *Tokens) TouchLastUsed(ctx context.Context, tokenID string) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE auth_tokens SET last_used_at = ? WHERE token_id = ?`,
		time.Now().Unix(), tokenID)
	return err
}

type ListedToken struct {
	TokenID    string
	UserSlug   string
	Label      string
	IssuedAt   time.Time
	IssuedBy   string
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	Scope      []byte
}

func (t *Tokens) List(ctx context.Context, userSlug string) ([]ListedToken, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if userSlug == "" {
		rows, err = t.db.QueryContext(ctx, `
			SELECT token_id, user_slug, label, issued_at, issued_by,
			       expires_at, revoked_at, last_used_at, scope_blob
			FROM auth_tokens
			ORDER BY issued_at DESC`)
	} else {
		rows, err = t.db.QueryContext(ctx, `
			SELECT token_id, user_slug, label, issued_at, issued_by,
			       expires_at, revoked_at, last_used_at, scope_blob
			FROM auth_tokens
			WHERE user_slug = ?
			ORDER BY issued_at DESC`,
			userSlug)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tokens []ListedToken
	for rows.Next() {
		var (
			lt         ListedToken
			issuedAt   int64
			expiresAt  *int64
			revokedAt  *int64
			lastUsedAt *int64
		)
		if err := rows.Scan(
			&lt.TokenID, &lt.UserSlug, &lt.Label, &issuedAt, &lt.IssuedBy,
			&expiresAt, &revokedAt, &lastUsedAt, &lt.Scope,
		); err != nil {
			return nil, err
		}
		lt.IssuedAt = time.Unix(issuedAt, 0)
		if expiresAt != nil {
			v := time.Unix(*expiresAt, 0)
			lt.ExpiresAt = &v
		}
		if revokedAt != nil {
			v := time.Unix(*revokedAt, 0)
			lt.RevokedAt = &v
		}
		if lastUsedAt != nil {
			v := time.Unix(*lastUsedAt, 0)
			lt.LastUsedAt = &v
		}
		tokens = append(tokens, lt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}
