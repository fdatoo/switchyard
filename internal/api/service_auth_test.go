package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
	"github.com/fdatoo/switchyard/internal/auth/throttle"
	"github.com/fdatoo/switchyard/internal/testutil"
)

// nopAppender discards all audit events (for tests).
type nopAppender struct{}

func (nopAppender) AppendAuth(_ context.Context, _ *eventv1.AuthEvent) error { return nil }

// newAuthTestDeps sets up a full AuthDeps with a real SQLite DB for tests.
func newAuthTestDeps(t *testing.T) (api.AuthDeps, *sql.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	identityStore, err := identity.New(context.Background(), db)
	require.NoError(t, err)

	sessStore := sessions.New(db, sessions.Config{
		Key:         []byte("test-hmac-key-32-bytes-long-xxxx"),
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
		RefreshIdle: 24 * time.Hour,
		AccessName:  "gohome_access",
		RefreshName: "gohome_refresh",
	})
	throttleStore := throttle.New(db, throttle.Config{
		Window:    15 * time.Minute,
		Threshold: 5,
		Block:     5 * time.Minute,
	})
	auditRec := audit.New(nopAppender{})

	return api.AuthDeps{
		Identity:   identityStore,
		Password:   credentials.NewPassword(db, credentials.DefaultArgon2idParams()),
		Tokens:     credentials.NewTokens(db),
		Sessions:   sessStore,
		Enrollment: credentials.NewEnrollment(db),
		Throttle:   throttleStore,
		Audit:      auditRec,
		Policy:     nil,
	}, db
}

// seedUser inserts a test user and sets their password.
func seedUser(t *testing.T, db *sql.DB, identityStore *identity.Store, slug, password string) {
	t.Helper()
	ctx := context.Background()
	err := identityStore.ApplySnapshot(ctx, identity.Snapshot{
		Users: []identity.User{
			{Slug: slug, DisplayName: slug, Active: true, PasswordAllowed: true},
		},
	})
	require.NoError(t, err)
	pw := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
	require.NoError(t, pw.Set(ctx, slug, password, "test"))
}

// ctxWithRW adds a ResponseWriter and HTTPRequest to the context (simulating listener middleware).
func ctxWithRW(w http.ResponseWriter, r *http.Request) context.Context {
	ctx := api.WithResponseWriter(context.Background(), w)
	ctx = api.WithHTTPRequest(ctx, r)
	return ctx
}

func TestAuthService_Login_Success(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "alice", "correct-password")

	svc := api.NewAuthService(deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := ctxWithRW(rec, req)

	resp, err := svc.Login(ctx, connect.NewRequest(&authpb.LoginRequest{
		Username: "alice",
		Password: "correct-password",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.SessionToken)

	// Cookies should have been set.
	cookies := rec.Result().Cookies()
	names := make([]string, 0, len(cookies))
	for _, c := range cookies {
		names = append(names, c.Name)
	}
	require.Contains(t, names, "gohome_access")
	require.Contains(t, names, "gohome_refresh")
}

func TestAuthService_Login_BadPassword(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "bob", "correct-password")

	svc := api.NewAuthService(deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := ctxWithRW(rec, req)

	_, err := svc.Login(ctx, connect.NewRequest(&authpb.LoginRequest{
		Username: "bob",
		Password: "wrong-password",
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeUnauthenticated, ce.Code())
}

func TestAuthService_Login_Throttled(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "charlie", "correct-password")

	// Use a very aggressive throttle for the test.
	deps.Throttle = throttle.New(db, throttle.Config{
		Window:    15 * time.Minute,
		Threshold: 1, // block after 1 failure
		Block:     1 * time.Hour,
	})
	svc := api.NewAuthService(deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	// First bad attempt — records a failure.
	ctx := ctxWithRW(rec, req)
	_, err := svc.Login(ctx, connect.NewRequest(&authpb.LoginRequest{
		Username: "charlie",
		Password: "wrong",
	}))
	require.Error(t, err)

	// Second attempt — should be throttled.
	rec2 := httptest.NewRecorder()
	ctx2 := ctxWithRW(rec2, req)
	_, err = svc.Login(ctx2, connect.NewRequest(&authpb.LoginRequest{
		Username: "charlie",
		Password: "correct-password", // right password, but throttled
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeUnauthenticated, ce.Code())
}

func TestAuthService_CreateToken(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "dave", "pass")

	svc := api.NewAuthService(deps)
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		ID: "user:dave", Kind: "user",
	})

	resp, err := svc.CreateToken(ctx, connect.NewRequest(&authpb.CreateTokenRequest{
		DisplayName: "my-token",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Token)
	require.NotEmpty(t, resp.Msg.TokenId)
}

func TestAuthService_RevokeToken(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "eve", "pass")

	svc := api.NewAuthService(deps)
	principal := auth.Principal{ID: "user:eve", Kind: "user"}
	ctx := auth.WithPrincipal(context.Background(), principal)

	// Issue a token first.
	createResp, err := svc.CreateToken(ctx, connect.NewRequest(&authpb.CreateTokenRequest{
		DisplayName: "to-revoke",
	}))
	require.NoError(t, err)
	tokenID := createResp.Msg.TokenId

	// Revoke it.
	_, err = svc.RevokeToken(ctx, connect.NewRequest(&authpb.RevokeTokenRequest{
		TokenId: tokenID,
	}))
	require.NoError(t, err)

	// Verify it's now revoked via Tokens.Verify.
	_, verifyErr := deps.Tokens.Verify(context.Background(), createResp.Msg.Token)
	require.ErrorIs(t, verifyErr, credentials.ErrTokenRevoked)
}

func TestAuthService_ExplainAuthorization_NilPolicy(t *testing.T) {
	deps, db := newAuthTestDeps(t)
	seedUser(t, db, deps.Identity, "frank", "pass")

	svc := api.NewAuthService(deps) // Policy is nil
	ctx := context.Background()

	_, err := svc.ExplainAuthorization(ctx, connect.NewRequest(&authpb.ExplainAuthorizationRequest{
		UserSlug:      "frank",
		ActionService: "AuthService",
		ActionMethod:  "Login",
		ActionVerb:    "write",
		TargetKind:    "",
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeUnimplemented, ce.Code())
}

func TestAuthService_ListUsers(t *testing.T) {
	deps, _ := newAuthTestDeps(t)

	// Seed two users in a single snapshot (ApplySnapshot replaces all users).
	ctx := context.Background()
	err := deps.Identity.ApplySnapshot(ctx, identity.Snapshot{
		Users: []identity.User{
			{Slug: "grace", DisplayName: "grace", Active: true, PasswordAllowed: true},
			{Slug: "hank", DisplayName: "hank", Active: true, PasswordAllowed: true},
		},
	})
	require.NoError(t, err)

	svc := api.NewAuthService(deps)

	resp, err := svc.ListUsers(ctx, connect.NewRequest(&authpb.ListUsersRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Users, 2)
}

func TestAuthService_MintAndRedeemEnrollmentToken(t *testing.T) {
	deps, _ := newAuthTestDeps(t)

	svc := api.NewAuthService(deps)
	ctx := context.Background()

	// Mint.
	mintResp, err := svc.MintEnrollmentToken(ctx, connect.NewRequest(&authpb.MintEnrollmentTokenRequest{
		UserSlug:   "grace",
		Intent:     credentials.IntentSetPassword,
		TtlSeconds: 3600,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, mintResp.Msg.Token)

	// Redeem.
	redeemResp, err := svc.RedeemEnrollmentToken(ctx, connect.NewRequest(&authpb.RedeemEnrollmentTokenRequest{
		Token: mintResp.Msg.Token,
	}))
	require.NoError(t, err)
	require.Equal(t, "grace", redeemResp.Msg.UserSlug)
	require.Equal(t, credentials.IntentSetPassword, redeemResp.Msg.Intent)

	// Redeem again — should fail (consumed).
	_, err = svc.RedeemEnrollmentToken(ctx, connect.NewRequest(&authpb.RedeemEnrollmentTokenRequest{
		Token: mintResp.Msg.Token,
	}))
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, connect.CodeUnauthenticated, ce.Code())
}
