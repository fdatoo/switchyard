package sessions_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func setupSessionDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	_, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	return db
}

func newStore(t *testing.T) *sessions.Store {
	return sessions.New(setupSessionDB(t), sessions.Config{
		Key:         []byte("test-hmac-key-32-bytes-long-xxxx"),
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  30 * 24 * time.Hour,
		RefreshIdle: 14 * 24 * time.Hour,
		AccessName:  "switchyard_access",
		RefreshName: "switchyard_refresh",
	})
}

func requestFromRecorder(rec *httptest.ResponseRecorder) *http.Request {
	req := httptest.NewRequest("GET", "https://switchyard.test/x", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func joinOther(cookies []*http.Cookie, skip int) string {
	var s string
	for i, c := range cookies {
		if i == skip {
			continue
		}
		s += c.Name + "=" + c.Value + "; "
	}
	return s
}

func TestSessions_IssueProducesCookies(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()

	data, err := st.Issue(ctx, rec, sessions.IssueInput{
		UserSlug:   "alice",
		AuthMethod: "password",
		RemoteIP:   "127.0.0.1",
		UserAgent:  "test-agent",
	})
	require.NoError(t, err)
	require.NotEmpty(t, data.SessionID)
	require.Equal(t, "alice", data.UserSlug)

	cookies := rec.Result().Cookies()
	var accessCookie, refreshCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case "switchyard_access":
			accessCookie = c
		case "switchyard_refresh":
			refreshCookie = c
		}
	}
	require.NotNil(t, accessCookie, "switchyard_access cookie must be set")
	require.NotNil(t, refreshCookie, "switchyard_refresh cookie must be set")

	require.True(t, accessCookie.HttpOnly, "access cookie must be HttpOnly")
	require.True(t, accessCookie.Secure, "access cookie must be Secure")
	require.Equal(t, http.SameSiteStrictMode, accessCookie.SameSite)

	require.True(t, refreshCookie.HttpOnly, "refresh cookie must be HttpOnly")
	require.True(t, refreshCookie.Secure, "refresh cookie must be Secure")
	require.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
}

func TestSessions_VerifyAccess_HappyPath(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()

	_, err := st.Issue(ctx, rec, sessions.IssueInput{
		UserSlug:   "bob",
		AuthMethod: "password",
	})
	require.NoError(t, err)

	req := requestFromRecorder(rec)
	principal, err := st.VerifyAccess(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "bob", principal.UserSlug)
	require.NotEmpty(t, principal.SessionID)
}

func TestSessions_VerifyAccess_TamperedRejected(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rec := httptest.NewRecorder()

	_, err := st.Issue(ctx, rec, sessions.IssueInput{UserSlug: "carol"})
	require.NoError(t, err)

	req := requestFromRecorder(rec)
	cookies := req.Cookies()
	for i, c := range cookies {
		if c.Name == "switchyard_access" {
			req = httptest.NewRequest("GET", "https://switchyard.test/x", nil)
			req.Header.Set("Cookie", c.Name+"=GARBAGE.GARBAGE; "+joinOther(cookies, i))
			break
		}
	}

	_, err = st.VerifyAccess(ctx, req)
	require.ErrorIs(t, err, sessions.ErrSessionInvalid)
}

func TestSessions_Refresh_RotatesAndAcceptsNewAccess(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	issueRec := httptest.NewRecorder()
	_, err := st.Issue(ctx, issueRec, sessions.IssueInput{UserSlug: "dave"})
	require.NoError(t, err)

	refreshRec := httptest.NewRecorder()
	refreshReq := requestFromRecorder(issueRec)
	data, err := st.Refresh(ctx, refreshRec, refreshReq)
	require.NoError(t, err)
	require.Equal(t, "dave", data.UserSlug)

	// new access cookie from refresh should be valid
	verifyReq := requestFromRecorder(refreshRec)
	principal, err := st.VerifyAccess(ctx, verifyReq)
	require.NoError(t, err)
	require.Equal(t, "dave", principal.UserSlug)
}

func TestSessions_Refresh_ReplayDetectionRevokesEntireSession(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	issueRec := httptest.NewRecorder()
	_, err := st.Issue(ctx, issueRec, sessions.IssueInput{UserSlug: "eve"})
	require.NoError(t, err)

	// First refresh — valid, produces new cookies
	refresh1Rec := httptest.NewRecorder()
	refresh1Req := requestFromRecorder(issueRec)
	_, err = st.Refresh(ctx, refresh1Rec, refresh1Req)
	require.NoError(t, err)

	// Second refresh using the OLD refresh cookie — replay attack
	replay1Rec := httptest.NewRecorder()
	_, err = st.Refresh(ctx, replay1Rec, refresh1Req)
	require.ErrorIs(t, err, sessions.ErrSessionReplay)

	// The new access token from the first refresh must also be invalid now
	verifyReq := requestFromRecorder(refresh1Rec)
	_, err = st.VerifyAccess(ctx, verifyReq)
	require.ErrorIs(t, err, sessions.ErrSessionInvalid)
}

func TestSessions_Logout_DeletesRowAndClearsCookies(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	issueRec := httptest.NewRecorder()
	data, err := st.Issue(ctx, issueRec, sessions.IssueInput{UserSlug: "frank"})
	require.NoError(t, err)

	logoutRec := httptest.NewRecorder()
	err = st.Logout(ctx, logoutRec, data.SessionID)
	require.NoError(t, err)

	// Cookies in logout response should have MaxAge=-1
	cookies := logoutRec.Result().Cookies()
	require.NotEmpty(t, cookies, "logout must set clearing cookies")
	for _, c := range cookies {
		require.Equal(t, -1, c.MaxAge, "cookie %s must have MaxAge=-1", c.Name)
	}

	// Verify the session is gone — VerifyAccess should fail
	verifyReq := requestFromRecorder(issueRec)
	_, err = st.VerifyAccess(ctx, verifyReq)
	require.ErrorIs(t, err, sessions.ErrSessionInvalid)
}
