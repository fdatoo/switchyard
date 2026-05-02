package credentials_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth/credentials"
)

func TestEnrollment_MintThenRedeem(t *testing.T) {
	db := setupAuthDB(t)
	e := credentials.NewEnrollment(db)
	ctx := context.Background()

	plaintext, err := e.Mint(ctx, "fdatoo", credentials.IntentRegisterPasskey, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, plaintext)

	lookup, err := e.Redeem(ctx, plaintext)
	require.NoError(t, err)
	require.Equal(t, "fdatoo", lookup.UserSlug)
	require.Equal(t, credentials.IntentRegisterPasskey, lookup.Intent)

	_, err = e.Redeem(ctx, plaintext)
	require.ErrorIs(t, err, credentials.ErrEnrollmentConsumed)
}

func TestEnrollment_Redeem_Expired(t *testing.T) {
	db := setupAuthDB(t)
	e := credentials.NewEnrollment(db)
	ctx := context.Background()

	plaintext, err := e.Mint(ctx, "fdatoo", credentials.IntentSetPassword, -time.Second)
	require.NoError(t, err)

	_, err = e.Redeem(ctx, plaintext)
	require.ErrorIs(t, err, credentials.ErrEnrollmentExpired)
}

func TestEnrollment_Redeem_Unknown(t *testing.T) {
	db := setupAuthDB(t)
	e := credentials.NewEnrollment(db)
	ctx := context.Background()

	_, err := e.Redeem(ctx, "totally-bogus")
	require.ErrorIs(t, err, credentials.ErrEnrollmentInvalid)
}
