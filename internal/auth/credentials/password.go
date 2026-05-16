package credentials

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// Argon2idParams are the tunable password-hashing parameters.
type Argon2idParams struct {
	Time        uint32
	MemoryKiB   uint32
	Parallelism uint8
}

// DefaultArgon2idParams returns the daemon's target password-hashing cost.
func DefaultArgon2idParams() Argon2idParams {
	return Argon2idParams{Time: 3, MemoryKiB: 64 * 1024, Parallelism: 4}
}

// Password stores and verifies Argon2id password credentials.
type Password struct {
	db     *sql.DB
	params Argon2idParams
}

// NewPassword returns a password store backed by db.
func NewPassword(db *sql.DB, p Argon2idParams) *Password {
	return &Password{db: db, params: p}
}

// Set stores or replaces the user's password hash. setBy carries audit
// provenance ("self", "admin:<slug>", "system:bootstrap").
func (p *Password) Set(ctx context.Context, userSlug, plaintext, setBy string) error {
	encoded, err := p.encode(plaintext)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO auth_passwords (user_slug, argon2id_hash, set_at, set_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_slug) DO UPDATE SET
			argon2id_hash = excluded.argon2id_hash,
			set_at        = excluded.set_at,
			set_by        = excluded.set_by`,
		userSlug, encoded, time.Now().Unix(), setBy)
	return err
}

// BootstrapHash stores a precomputed password hash only when the user has no
// password yet. It is intended for config-defined bootstrap credentials.
func (p *Password) BootstrapHash(ctx context.Context, userSlug, encoded, setBy string) error {
	if _, err := decode(encoded); err != nil {
		return err
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO auth_passwords (user_slug, argon2id_hash, set_at, set_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_slug) DO NOTHING`,
		userSlug, encoded, time.Now().Unix(), setBy)
	return err
}

// Delete removes the user's password credential if one exists.
func (p *Password) Delete(ctx context.Context, userSlug string) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM auth_passwords WHERE user_slug = ?`, userSlug)
	return err
}

// Verify returns (ok, needsRehash, err). ok is false on missing user, missing
// hash, or wrong password — indistinguishable to the caller (no enumeration).
// needsRehash is true when the stored hash uses parameters that differ from
// the current target.
func (p *Password) Verify(ctx context.Context, userSlug, plaintext string) (bool, bool, error) {
	var encoded string
	err := p.db.QueryRowContext(ctx,
		`SELECT argon2id_hash FROM auth_passwords WHERE user_slug = ?`, userSlug).
		Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	parsed, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	candidate := argon2.IDKey([]byte(plaintext), parsed.salt,
		parsed.params.Time, parsed.params.MemoryKiB, parsed.params.Parallelism, uint32(len(parsed.hash)))
	if subtle.ConstantTimeCompare(candidate, parsed.hash) != 1 {
		return false, false, nil
	}
	needsRehash := parsed.params != p.params
	return true, needsRehash, nil
}

func (p *Password) encode(plaintext string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(plaintext), salt,
		p.params.Time, p.params.MemoryKiB, p.params.Parallelism, 32)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.params.MemoryKiB, p.params.Time, p.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

type parsed struct {
	params Argon2idParams
	salt   []byte
	hash   []byte
}

func decode(encoded string) (parsed, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return parsed{}, errors.New("credentials: invalid hash format")
	}
	var ver int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &ver); err != nil || ver != argon2.Version {
		return parsed{}, errors.New("credentials: unsupported argon2 version")
	}
	var p parsed
	paramKVs := strings.Split(parts[3], ",")
	for _, kv := range paramKVs {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			return parsed{}, errors.New("credentials: malformed params")
		}
		n, err := strconv.Atoi(kv[eq+1:])
		if err != nil {
			return parsed{}, err
		}
		switch kv[:eq] {
		case "m":
			p.params.MemoryKiB = uint32(n)
		case "t":
			p.params.Time = uint32(n)
		case "p":
			p.params.Parallelism = uint8(n)
		default:
			return parsed{}, fmt.Errorf("credentials: unknown param %q", kv[:eq])
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return parsed{}, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return parsed{}, err
	}
	p.salt = salt
	p.hash = hash
	return p, nil
}
