package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	authMigrations "github.com/fdatoo/switchyard/internal/auth/migrations"
	"github.com/fdatoo/switchyard/internal/storage"
)

var ErrNotFound = errors.New("identity: user not found")

// User represents a user from the Pkl config, with roles expanded into a flat list.
type User struct {
	Slug            string
	DisplayName     string
	Active          bool
	PasswordAllowed bool
	PasskeyAllowed  bool
	Roles           []string
}

// Snapshot is the complete state of users and roles to apply atomically.
type Snapshot struct {
	Users []User
}

// Store is the read API and projector for identity (users + roles).
type Store struct {
	db *sql.DB
}

// New returns a Store attached to an already-open DB and runs auth migrations.
func New(ctx context.Context, db *sql.DB) (*Store, error) {
	if err := storage.Migrate(ctx, db, authMigrations.FS, "auth"); err != nil {
		return nil, fmt.Errorf("auth migrations: %w", err)
	}
	return &Store{db: db}, nil
}

// ApplySnapshot atomically replaces auth_users and auth_user_roles with the given snapshot.
func (s *Store) ApplySnapshot(ctx context.Context, snap Snapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing users and roles.
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_user_roles`); err != nil {
		return fmt.Errorf("clear auth_user_roles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_users`); err != nil {
		return fmt.Errorf("clear auth_users: %w", err)
	}

	// Insert new users.
	for _, u := range snap.Users {
		active := 0
		if u.Active {
			active = 1
		}
		passwordAllowed := 0
		if u.PasswordAllowed {
			passwordAllowed = 1
		}
		passkeyAllowed := 0
		if u.PasskeyAllowed {
			passkeyAllowed = 1
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO auth_users (slug, display_name, active, password_allowed, passkey_allowed)
			VALUES (?, ?, ?, ?, ?)`,
			u.Slug, u.DisplayName, active, passwordAllowed, passkeyAllowed,
		)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", u.Slug, err)
		}

		// Insert roles for this user.
		for _, role := range u.Roles {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO auth_user_roles (user_slug, role_slug)
				VALUES (?, ?)`,
				u.Slug, role,
			)
			if err != nil {
				return fmt.Errorf("insert role %s for user %s: %w", role, u.Slug, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Get returns a user by slug, or ErrNotFound if not present.
func (s *Store) Get(ctx context.Context, slug string) (User, error) {
	var u User
	row := s.db.QueryRowContext(ctx, `
		SELECT slug, display_name, active, password_allowed, passkey_allowed
		FROM auth_users
		WHERE slug = ?`,
		slug,
	)
	var active, passwordAllowed, passkeyAllowed int
	err := row.Scan(&u.Slug, &u.DisplayName, &active, &passwordAllowed, &passkeyAllowed)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	u.Active = active != 0
	u.PasswordAllowed = passwordAllowed != 0
	u.PasskeyAllowed = passkeyAllowed != 0

	// Load roles.
	rows, err := s.db.QueryContext(ctx, `
		SELECT role_slug
		FROM auth_user_roles
		WHERE user_slug = ?
		ORDER BY role_slug`,
		slug,
	)
	if err != nil {
		return User{}, fmt.Errorf("query roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return User{}, fmt.Errorf("scan role: %w", err)
		}
		u.Roles = append(u.Roles, role)
	}
	if err := rows.Err(); err != nil {
		return User{}, fmt.Errorf("rows error: %w", err)
	}

	return u, nil
}

// RolesFor returns the list of roles for a user, or an empty slice if the user is unknown.
// Unlike Get, this does not error if the user does not exist.
func (s *Store) RolesFor(ctx context.Context, slug string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT role_slug
		FROM auth_user_roles
		WHERE user_slug = ?
		ORDER BY role_slug`,
		slug,
	)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return roles, nil
}

// ListUsers returns all users in the store, with their roles populated.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT slug, display_name, active, password_allowed, passkey_allowed
		FROM auth_users
		ORDER BY slug`,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		var active, passwordAllowed, passkeyAllowed int
		if err := rows.Scan(&u.Slug, &u.DisplayName, &active, &passwordAllowed, &passkeyAllowed); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Active = active != 0
		u.PasswordAllowed = passwordAllowed != 0
		u.PasskeyAllowed = passkeyAllowed != 0

		// Load roles for this user.
		roleRows, err := s.db.QueryContext(ctx, `
			SELECT role_slug
			FROM auth_user_roles
			WHERE user_slug = ?
			ORDER BY role_slug`,
			u.Slug,
		)
		if err != nil {
			return nil, fmt.Errorf("query roles for user %s: %w", u.Slug, err)
		}
		for roleRows.Next() {
			var role string
			if err := roleRows.Scan(&role); err != nil {
				_ = roleRows.Close()
				return nil, fmt.Errorf("scan role: %w", err)
			}
			u.Roles = append(u.Roles, role)
		}
		_ = roleRows.Close()
		if err := roleRows.Err(); err != nil {
			return nil, fmt.Errorf("roles rows error: %w", err)
		}

		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}
