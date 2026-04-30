package api

import (
	"net/http"

	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/auth/authn"
)

// MCPAuthMiddleware wraps an http.Handler with bearer-token authentication.
// It calls the Bearer authenticator; on success it stashes the principal on
// the context. On failure or missing header it passes through without setting
// the principal — the MCP handler decides whether to require authentication.
func MCPAuthMiddleware(bearer *authn.Bearer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bearer != nil && r.Header.Get("Authorization") != "" {
			req := auth.Request{
				Headers:    r.Header,
				RemoteAddr: r.RemoteAddr,
			}
			p, err := bearer.Authenticate(r.Context(), req)
			if err == nil {
				r = r.WithContext(auth.WithPrincipal(r.Context(), p))
			}
		}
		next.ServeHTTP(w, r)
	})
}
