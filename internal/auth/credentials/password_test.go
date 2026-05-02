package credentials_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func setupAuthDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	_, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	return db
}

func TestPassword_SetThenVerify(t *testing.T) {
	db := setupAuthDB(t)
	p := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
	ctx := context.Background()
	require.NoError(t, p.Set(ctx, "fdatoo", "correct horse battery staple", "self"))
	ok, needsRehash, err := p.Verify(ctx, "fdatoo", "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, needsRehash)
}

func TestPassword_Verify_WrongReturnsOkFalse(t *testing.T) {
	db := setupAuthDB(t)
	p := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
	ctx := context.Background()
	require.NoError(t, p.Set(ctx, "fdatoo", "secret", "self"))
	ok, _, err := p.Verify(ctx, "fdatoo", "wrong")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPassword_Verify_UnknownUserReturnsOkFalse(t *testing.T) {
	db := setupAuthDB(t)
	p := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
	ok, _, err := p.Verify(context.Background(), "ghost", "anything")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPassword_NeedsRehash_OnParamMismatch(t *testing.T) {
	db := setupAuthDB(t)
	weak := credentials.Argon2idParams{Time: 1, MemoryKiB: 16384, Parallelism: 1}
	strong := credentials.Argon2idParams{Time: 3, MemoryKiB: 65536, Parallelism: 4}
	pWeak := credentials.NewPassword(db, weak)
	require.NoError(t, pWeak.Set(context.Background(), "fdatoo", "secret", "self"))

	pStrong := credentials.NewPassword(db, strong)
	ok, needsRehash, err := pStrong.Verify(context.Background(), "fdatoo", "secret")
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, needsRehash)
}

func TestPassword_Delete_RemovesRow(t *testing.T) {
	db := setupAuthDB(t)
	p := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
	ctx := context.Background()
	require.NoError(t, p.Set(ctx, "fdatoo", "x", "self"))
	require.NoError(t, p.Delete(ctx, "fdatoo"))
	ok, _, err := p.Verify(ctx, "fdatoo", "x")
	require.NoError(t, err)
	require.False(t, ok)
}
