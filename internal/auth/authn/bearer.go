package authn

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
)

// Bearer authenticates requests that carry an "Authorization: Bearer <token>" header.
type Bearer struct {
	tokens *credentials.Tokens
}

// NewBearer returns a Bearer authenticator backed by the given Tokens store.
func NewBearer(t *credentials.Tokens) *Bearer { return &Bearer{tokens: t} }

// Authenticate implements auth.Authenticator.
func (b *Bearer) Authenticate(ctx context.Context, req auth.Request) (auth.Principal, error) {
	h := req.Headers.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return auth.Principal{}, auth.ErrNotApplicable
	}
	plaintext := strings.TrimPrefix(h, "Bearer ")
	look, err := b.tokens.Verify(ctx, plaintext)
	switch {
	case errors.Is(err, credentials.ErrTokenInvalid),
		errors.Is(err, credentials.ErrTokenRevoked),
		errors.Is(err, credentials.ErrTokenExpired):
		return auth.Principal{}, auth.ErrUnauthenticated
	case err != nil:
		return auth.Principal{}, err
	}
	meta := map[string]string{
		"token_id":    look.TokenID,
		"auth_method": "token",
	}
	if len(look.Scope) > 0 {
		meta["token_scope"] = base64.StdEncoding.EncodeToString(look.Scope)
	}
	return auth.Principal{
		ID:          "user:" + look.UserSlug,
		Kind:        "user",
		DisplayName: look.UserSlug,
		Metadata:    meta,
	}, nil
}
