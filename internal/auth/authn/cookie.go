package authn

import (
	"context"
	"errors"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
)

// SessionCookie authenticates requests that carry a valid session access cookie.
type SessionCookie struct {
	sessions *sessions.Store
}

// NewSessionCookie returns a SessionCookie authenticator.
func NewSessionCookie(s *sessions.Store) *SessionCookie {
	return &SessionCookie{sessions: s}
}

// Authenticate implements auth.Authenticator.
func (c *SessionCookie) Authenticate(ctx context.Context, req auth.Request) (auth.Principal, error) {
	httpReq := req.HTTP
	if httpReq == nil {
		return auth.Principal{}, auth.ErrNotApplicable
	}
	if _, err := httpReq.Cookie(c.sessions.CookieName()); err != nil {
		return auth.Principal{}, auth.ErrNotApplicable
	}
	p, err := c.sessions.VerifyAccess(ctx, httpReq)
	switch {
	case errors.Is(err, sessions.ErrSessionInvalid),
		errors.Is(err, sessions.ErrSessionExpired),
		errors.Is(err, sessions.ErrSessionReplay):
		return auth.Principal{}, auth.ErrUnauthenticated
	case err != nil:
		return auth.Principal{}, err
	}
	return auth.Principal{
		ID:          "user:" + p.UserSlug,
		Kind:        "user",
		DisplayName: p.UserSlug,
		Metadata: map[string]string{
			"session_id":  p.SessionID,
			"auth_method": p.AuthMethod,
		},
	}, nil
}
