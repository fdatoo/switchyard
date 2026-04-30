package identity_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/auth/identity"
	"github.com/fdatoo/gohome/internal/testutil"
)

func TestStore_ApplySnapshot_PopulatesUsersAndRoles(t *testing.T) {
	db := testutil.NewTestDB(t)
	s, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	snap := identity.Snapshot{
		Users: []identity.User{
			{Slug: "fdatoo", DisplayName: "Fynn", Active: true,
				PasswordAllowed: true, PasskeyAllowed: true,
				Roles: []string{"admin"}},
			{Slug: "nora", DisplayName: "Nora", Active: true,
				PasswordAllowed: false, PasskeyAllowed: true,
				Roles: []string{"kids"}},
		},
	}
	require.NoError(t, s.ApplySnapshot(context.Background(), snap))
	fd, err := s.Get(context.Background(), "fdatoo")
	require.NoError(t, err)
	require.Equal(t, []string{"admin"}, fd.Roles)
	require.True(t, fd.PasswordAllowed)
	nora, err := s.Get(context.Background(), "nora")
	require.NoError(t, err)
	require.False(t, nora.PasswordAllowed)
}

func TestStore_ApplySnapshot_RemovesAbsentUsers(t *testing.T) {
	db := testutil.NewTestDB(t)
	s, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	require.NoError(t, s.ApplySnapshot(context.Background(), identity.Snapshot{
		Users: []identity.User{{Slug: "old", DisplayName: "Old", Active: true, Roles: []string{"admin"}}},
	}))
	require.NoError(t, s.ApplySnapshot(context.Background(), identity.Snapshot{
		Users: []identity.User{{Slug: "new", DisplayName: "New", Active: true, Roles: []string{"admin"}}},
	}))
	_, err = s.Get(context.Background(), "old")
	require.ErrorIs(t, err, identity.ErrNotFound)
	_, err = s.Get(context.Background(), "new")
	require.NoError(t, err)
}

func TestStore_RolesFor_UnknownUserReturnsEmpty(t *testing.T) {
	db := testutil.NewTestDB(t)
	s, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	roles, err := s.RolesFor(context.Background(), "ghost")
	require.NoError(t, err)
	require.Empty(t, roles)
}
