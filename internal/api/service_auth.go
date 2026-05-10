package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"

	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
	"github.com/fdatoo/switchyard/internal/auth/throttle"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/policy"
)

const webAuthnChallengeCookie = "switchyard_webauthn"

type webAuthnChallengePayload struct {
	Kind        string          `json:"kind"`
	UserSlug    string          `json:"user_slug,omitempty"`
	SessionData *wa.SessionData `json:"session_data"`
}

// AuthDeps holds the dependencies required by the real AuthService.
type AuthDeps struct {
	Identity   *identity.Store
	Password   *credentials.Password
	Passkeys   *credentials.Passkeys
	Tokens     *credentials.Tokens
	Sessions   *sessions.Store
	Enrollment *credentials.Enrollment
	Challenges *credentials.ChallengeStore
	Throttle   *throttle.Throttle
	Audit      *audit.Recorder
	Policy     *policy.Runtime
	Metrics    *observability.Metrics
}

// AuthService implements AuthServiceHandler with real auth logic.
type AuthService struct {
	d AuthDeps
}

// NewAuthService constructs an AuthService with the supplied dependencies.
func NewAuthService(d AuthDeps) *AuthService { return &AuthService{d: d} }

// Login authenticates a user with username + password and issues a session cookie.
func (s *AuthService) Login(ctx context.Context, req *connect.Request[authpb.LoginRequest]) (*connect.Response[authpb.LoginResponse], error) {
	if len(req.Msg.PublicKeyCredential) > 0 || req.Msg.WebauthnChallengeId != "" {
		return s.loginPasskey(ctx, req)
	}

	ip := remoteAddrFromCtx(ctx)
	ua := userAgentFromCtx(ctx)
	username := req.Msg.Username
	password := req.Msg.Password

	start := time.Now()
	defer func() {
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginDurationSeconds.WithLabelValues("password").Observe(time.Since(start).Seconds())
		}
	}()

	if err := s.d.Throttle.Check(ctx, ip, "password"); err != nil {
		if s.d.Metrics != nil {
			s.d.Metrics.AuthThrottleBlocksTotal.WithLabelValues("password").Inc()
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("password", "throttled").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "password", AttemptedUserSlug: username, Reason: "throttled",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("throttled"))
	}

	u, err := s.d.Identity.Get(ctx, username)
	if errors.Is(err, identity.ErrNotFound) || (err == nil && (!u.Active || !u.PasswordAllowed)) {
		_ = s.d.Throttle.Record(ctx, ip, "password", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("password", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "password", AttemptedUserSlug: username, Reason: "user_not_available",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ok, _, err := s.d.Password.Verify(ctx, username, password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		_ = s.d.Throttle.Record(ctx, ip, "password", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("password", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "password", AttemptedUserSlug: username, Reason: "bad_credentials",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	_ = s.d.Throttle.Record(ctx, ip, "password", true)

	w := responseWriterFromCtx(ctx)
	if w == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no response writer"))
	}
	sd, err := s.d.Sessions.Issue(ctx, w, sessions.IssueInput{
		UserSlug: u.Slug, AuthMethod: "password", RemoteIP: ip, UserAgent: ua,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.LoginSucceeded(ctx, audit.Identity{
		PrincipalID: "user:" + u.Slug, SourceIP: ip, RequestID: requestIDFromCtx(ctx),
	}, audit.LoginSucceeded{
		AuthMethod: "password", UserSlug: u.Slug, SessionID: sd.SessionID,
	})
	if s.d.Metrics != nil {
		s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("password", "ok").Inc()
	}
	return connect.NewResponse(&authpb.LoginResponse{SessionToken: sd.SessionID}), nil
}

func (s *AuthService) loginPasskey(ctx context.Context, req *connect.Request[authpb.LoginRequest]) (*connect.Response[authpb.LoginResponse], error) {
	if s.d.Passkeys == nil || s.d.Challenges == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys not configured"))
	}
	ip := remoteAddrFromCtx(ctx)
	ua := userAgentFromCtx(ctx)

	start := time.Now()
	defer func() {
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginDurationSeconds.WithLabelValues("passkey").Observe(time.Since(start).Seconds())
		}
	}()

	if err := s.d.Throttle.Check(ctx, ip, "passkey"); err != nil {
		if s.d.Metrics != nil {
			s.d.Metrics.AuthThrottleBlocksTotal.WithLabelValues("passkey").Inc()
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "throttled").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "passkey", Reason: "throttled",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("throttled"))
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(req.Msg.PublicKeyCredential)
	if err != nil {
		_ = s.d.Throttle.Record(ctx, ip, "passkey", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "passkey", Reason: "passkey_assertion_invalid",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid passkey assertion"))
	}

	payload, err := s.consumeWebAuthnChallenge(ctx, req.Msg.WebauthnChallengeId, "login")
	if err != nil {
		_ = s.d.Throttle.Record(ctx, ip, "passkey", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "passkey", Reason: "passkey_assertion_invalid",
		})
		if errors.Is(err, credentials.ErrChallengeNotFound) {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		return nil, err
	}

	slug, err := s.d.Passkeys.FinishLogin(ctx, payload.SessionData, parsed)
	if errors.Is(err, credentials.ErrSignCountRegression) {
		_ = s.d.Throttle.Record(ctx, ip, "passkey", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "passkey", AttemptedUserSlug: string(parsed.Response.UserHandle), Reason: "sign_count_regression",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if err != nil {
		_ = s.d.Throttle.Record(ctx, ip, "passkey", false)
		if s.d.Metrics != nil {
			s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "failed").Inc()
		}
		_ = s.d.Audit.LoginFailed(ctx, audit.Identity{SourceIP: ip}, audit.LoginFailed{
			AuthMethod: "passkey", AttemptedUserSlug: string(parsed.Response.UserHandle), Reason: "passkey_assertion_invalid",
		})
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid passkey assertion"))
	}
	_ = s.d.Throttle.Record(ctx, ip, "passkey", true)

	w := responseWriterFromCtx(ctx)
	if w == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no response writer"))
	}
	sd, err := s.d.Sessions.Issue(ctx, w, sessions.IssueInput{
		UserSlug: slug, AuthMethod: "passkey", RemoteIP: ip, UserAgent: ua,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	credentialID := base64.RawURLEncoding.EncodeToString(parsed.RawID)
	_ = s.d.Audit.LoginSucceeded(ctx, audit.Identity{
		PrincipalID: "user:" + slug, SourceIP: ip, RequestID: requestIDFromCtx(ctx),
	}, audit.LoginSucceeded{
		AuthMethod: "passkey", UserSlug: slug, SessionID: sd.SessionID, CredentialID: credentialID,
	})
	if s.d.Metrics != nil {
		s.d.Metrics.AuthLoginAttemptsTotal.WithLabelValues("passkey", "ok").Inc()
	}
	return connect.NewResponse(&authpb.LoginResponse{SessionToken: sd.SessionID}), nil
}

// Logout destroys the current session and clears cookies.
func (s *AuthService) Logout(ctx context.Context, _ *connect.Request[authpb.LogoutRequest]) (*connect.Response[authpb.LogoutResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return connect.NewResponse(&authpb.LogoutResponse{}), nil
	}
	sid := p.Metadata["session_id"]
	if sid == "" {
		return connect.NewResponse(&authpb.LogoutResponse{}), nil
	}
	w := responseWriterFromCtx(ctx)
	if w == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no response writer"))
	}
	if err := s.d.Sessions.Logout(ctx, w, sid); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.Logout(ctx, identityFromCtx(ctx), audit.Logout{
		UserSlug: strings.TrimPrefix(p.ID, "user:"), SessionID: sid,
	})
	return connect.NewResponse(&authpb.LogoutResponse{}), nil
}

// Refresh rotates session cookies using the refresh cookie in the request.
func (s *AuthService) Refresh(ctx context.Context, _ *connect.Request[authpb.RefreshRequest]) (*connect.Response[authpb.RefreshResponse], error) {
	w := responseWriterFromCtx(ctx)
	r := httpRequestFromCtx(ctx)
	if w == nil || r == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no http context"))
	}
	sd, err := s.d.Sessions.Refresh(ctx, w, r)
	switch {
	case errors.Is(err, sessions.ErrSessionInvalid), errors.Is(err, sessions.ErrSessionExpired), errors.Is(err, sessions.ErrSessionReplay):
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.SessionRefreshed(ctx, identityFromCtx(ctx), audit.SessionRefreshed{
		UserSlug: sd.UserSlug, SessionID: sd.SessionID,
	})
	return connect.NewResponse(&authpb.RefreshResponse{UserSlug: sd.UserSlug, SessionId: sd.SessionID}), nil
}

// CurrentUser returns the authenticated principal's user record.
func (s *AuthService) CurrentUser(ctx context.Context, _ *connect.Request[authpb.CurrentUserRequest]) (*connect.Response[authpb.CurrentUserResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	slug := strings.TrimPrefix(p.ID, "user:")
	u, err := s.d.Identity.Get(ctx, slug)
	if errors.Is(err, identity.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authpb.CurrentUserResponse{
		User: &authpb.User{
			Slug: u.Slug, DisplayName: u.DisplayName, Active: u.Active, Roles: u.Roles,
		},
	}), nil
}

// CreateToken issues a new bearer token for the authenticated user.
func (s *AuthService) CreateToken(ctx context.Context, req *connect.Request[authpb.CreateTokenRequest]) (*connect.Response[authpb.CreateTokenResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	slug := strings.TrimPrefix(p.ID, "user:")
	plaintext, tokenID, err := s.d.Tokens.Issue(ctx, credentials.IssueTokenInput{
		UserSlug: slug,
		Label:    req.Msg.DisplayName,
		IssuedBy: p.ID,
		Scope:    []byte{},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.TokenMinted(ctx, identityFromCtx(ctx), audit.TokenMinted{
		UserSlug: slug, TokenID: tokenID, Label: req.Msg.DisplayName, IssuedBy: p.ID,
	})
	return connect.NewResponse(&authpb.CreateTokenResponse{Token: plaintext, TokenId: tokenID}), nil
}

// RevokeToken revokes a token by ID.
func (s *AuthService) RevokeToken(ctx context.Context, req *connect.Request[authpb.RevokeTokenRequest]) (*connect.Response[authpb.RevokeTokenResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	if err := s.d.Tokens.Revoke(ctx, req.Msg.TokenId, p.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.TokenRevoked(ctx, identityFromCtx(ctx), audit.TokenRevoked{
		TokenID: req.Msg.TokenId, RevokedBy: p.ID,
	})
	return connect.NewResponse(&authpb.RevokeTokenResponse{}), nil
}

// ListUsers returns all users in the identity store.
func (s *AuthService) ListUsers(ctx context.Context, _ *connect.Request[authpb.ListUsersRequest]) (*connect.Response[authpb.ListUsersResponse], error) {
	users, err := s.d.Identity.ListUsers(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pbUsers := make([]*authpb.User, 0, len(users))
	for _, u := range users {
		pbUsers = append(pbUsers, &authpb.User{
			Slug: u.Slug, DisplayName: u.DisplayName, Active: u.Active, Roles: u.Roles,
		})
	}
	return connect.NewResponse(&authpb.ListUsersResponse{Users: pbUsers}), nil
}

// RegisterPasskey finishes a WebAuthn registration ceremony started by StartWebAuthnChallenge.
func (s *AuthService) RegisterPasskey(ctx context.Context, req *connect.Request[authpb.RegisterPasskeyRequest]) (*connect.Response[authpb.RegisterPasskeyResponse], error) {
	if s.d.Passkeys == nil || s.d.Challenges == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys not configured"))
	}
	slug, err := userSlugForPasskeyRegistration(ctx, req.Msg.UserSlug)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(req.Msg.PublicKeyCredential)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	payload, err := s.consumeWebAuthnChallenge(ctx, req.Msg.WebauthnChallengeId, "register")
	if errors.Is(err, credentials.ErrChallengeNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if err != nil {
		return nil, err
	}
	if payload.UserSlug != "" && payload.UserSlug != slug {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("challenge belongs to a different user"))
	}
	label := req.Msg.Label
	if label == "" {
		label = "passkey"
	}
	pk, err := s.d.Passkeys.FinishRegistration(ctx, slug, label, payload.SessionData, parsed)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	credentialID := base64.RawURLEncoding.EncodeToString(pk.CredentialID)
	_ = s.d.Audit.PasskeyRegistered(ctx, identityFromCtx(ctx), audit.PasskeyRegistered{
		UserSlug: slug, CredentialID: credentialID, Label: label,
	})
	return connect.NewResponse(&authpb.RegisterPasskeyResponse{CredentialId: credentialID}), nil
}

// StartWebAuthnChallenge starts a passkey registration or discoverable-login ceremony.
func (s *AuthService) StartWebAuthnChallenge(ctx context.Context, req *connect.Request[authpb.StartWebAuthnChallengeRequest]) (*connect.Response[authpb.StartWebAuthnChallengeResponse], error) {
	if s.d.Passkeys == nil || s.d.Challenges == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys not configured"))
	}
	switch req.Msg.Intent {
	case "register", "register_passkey":
		slug, err := userSlugForPasskeyRegistration(ctx, req.Msg.UserSlug)
		if err != nil {
			return nil, err
		}
		creation, sd, err := s.d.Passkeys.BeginRegistration(ctx, slug, req.Msg.DisplayName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		challengeID, err := s.storeWebAuthnChallenge(ctx, webAuthnChallengePayload{
			Kind: "register", UserSlug: slug, SessionData: sd,
		})
		if err != nil {
			return nil, err
		}
		options, err := json.Marshal(creation.Response)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&authpb.StartWebAuthnChallengeResponse{
			Challenge: options, WebauthnChallengeId: challengeID,
		}), nil
	case "login":
		assertion, sd, err := s.d.Passkeys.BeginLogin(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		assertion.Response.AllowedCredentials = []protocol.CredentialDescriptor{}
		challengeID, err := s.storeWebAuthnChallenge(ctx, webAuthnChallengePayload{
			Kind: "login", SessionData: sd,
		})
		if err != nil {
			return nil, err
		}
		options, err := json.Marshal(assertion.Response)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&authpb.StartWebAuthnChallengeResponse{
			Challenge: options, WebauthnChallengeId: challengeID,
		}), nil
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid webauthn intent"))
	}
}

func userSlugForPasskeyRegistration(ctx context.Context, requested string) (string, error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if ok && strings.HasPrefix(p.ID, "user:") {
		slug := strings.TrimPrefix(p.ID, "user:")
		if requested != "" && requested != slug {
			return "", connect.NewError(connect.CodePermissionDenied, errors.New("cannot register passkey for another user"))
		}
		return slug, nil
	}
	if requested != "" {
		return requested, nil
	}
	return "", connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
}

func (s *AuthService) storeWebAuthnChallenge(ctx context.Context, payload webAuthnChallengePayload) (string, error) {
	key, err := webAuthnChallengeSessionKey(ctx, true)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	id, err := s.d.Challenges.Store(ctx, key, data)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	return id, nil
}

func (s *AuthService) consumeWebAuthnChallenge(ctx context.Context, id, kind string) (webAuthnChallengePayload, error) {
	if id == "" {
		return webAuthnChallengePayload{}, connect.NewError(connect.CodeInvalidArgument, errors.New("missing webauthn challenge id"))
	}
	key, err := webAuthnChallengeSessionKey(ctx, false)
	if err != nil {
		return webAuthnChallengePayload{}, err
	}
	data, err := s.d.Challenges.Consume(ctx, key, id)
	if err != nil {
		return webAuthnChallengePayload{}, err
	}
	var payload webAuthnChallengePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return webAuthnChallengePayload{}, connect.NewError(connect.CodeInternal, err)
	}
	if payload.Kind != kind || payload.SessionData == nil {
		return webAuthnChallengePayload{}, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid webauthn challenge"))
	}
	return payload, nil
}

func webAuthnChallengeSessionKey(ctx context.Context, create bool) (string, error) {
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		if sid := p.Metadata["session_id"]; sid != "" {
			return "session:" + sid, nil
		}
	}
	r := httpRequestFromCtx(ctx)
	if r != nil {
		if c, err := r.Cookie(webAuthnChallengeCookie); err == nil && c.Value != "" {
			return "cookie:" + c.Value, nil
		}
	}
	if !create {
		return "", credentials.ErrChallengeNotFound
	}
	w := responseWriterFromCtx(ctx)
	if w == nil {
		return "", connect.NewError(connect.CodeInternal, errors.New("no response writer"))
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{
		Name: webAuthnChallengeCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		MaxAge: int((5 * time.Minute).Seconds()),
	})
	return "cookie:" + value, nil
}

// MintEnrollmentToken issues a one-time enrollment token for a user.
func (s *AuthService) MintEnrollmentToken(ctx context.Context, req *connect.Request[authpb.MintEnrollmentTokenRequest]) (*connect.Response[authpb.MintEnrollmentTokenResponse], error) {
	ttl := time.Duration(req.Msg.TtlSeconds) * time.Second
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	expiresAt := time.Now().Add(ttl)
	plaintext, err := s.d.Enrollment.Mint(ctx, req.Msg.UserSlug, req.Msg.Intent, ttl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.EnrollmentTokenMinted(ctx, identityFromCtx(ctx), audit.EnrollmentTokenMinted{
		UserSlug: req.Msg.UserSlug, Intent: req.Msg.Intent, ExpiresAt: expiresAt.Unix(),
	})
	return connect.NewResponse(&authpb.MintEnrollmentTokenResponse{
		Token: plaintext, ExpiresAt: expiresAt.Unix(),
	}), nil
}

// RedeemEnrollmentToken validates and consumes a one-time enrollment token.
func (s *AuthService) RedeemEnrollmentToken(ctx context.Context, req *connect.Request[authpb.RedeemEnrollmentTokenRequest]) (*connect.Response[authpb.RedeemEnrollmentTokenResponse], error) {
	result, err := s.d.Enrollment.Redeem(ctx, req.Msg.Token)
	switch {
	case errors.Is(err, credentials.ErrEnrollmentInvalid), errors.Is(err, credentials.ErrEnrollmentExpired), errors.Is(err, credentials.ErrEnrollmentConsumed):
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.EnrollmentTokenRedeemed(ctx, identityFromCtx(ctx), audit.EnrollmentTokenRedeemed{
		UserSlug: result.UserSlug, Intent: result.Intent,
	})
	return connect.NewResponse(&authpb.RedeemEnrollmentTokenResponse{
		UserSlug: result.UserSlug, Intent: result.Intent,
	}), nil
}

// ChangePassword verifies the old password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, req *connect.Request[authpb.ChangePasswordRequest]) (*connect.Response[authpb.ChangePasswordResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	slug := strings.TrimPrefix(p.ID, "user:")
	ok2, _, err := s.d.Password.Verify(ctx, slug, req.Msg.OldPlaintext)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok2 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	if err := s.d.Password.Set(ctx, slug, req.Msg.NewPlaintext, p.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = s.d.Audit.PasswordChanged(ctx, identityFromCtx(ctx), audit.PasswordChanged{
		UserSlug: slug, SetBy: p.ID,
	})
	return connect.NewResponse(&authpb.ChangePasswordResponse{}), nil
}

// ExplainAuthorization evaluates and explains a policy decision for a given user and action.
func (s *AuthService) ExplainAuthorization(ctx context.Context, req *connect.Request[authpb.ExplainAuthorizationRequest]) (*connect.Response[authpb.ExplainAuthorizationResponse], error) {
	u, err := s.d.Identity.Get(ctx, req.Msg.UserSlug)
	if errors.Is(err, identity.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	principal := auth.Principal{
		ID:   "user:" + u.Slug,
		Kind: "user",
	}
	action := auth.Action{
		Service: req.Msg.ActionService,
		Method:  req.Msg.ActionMethod,
		Verb:    req.Msg.ActionVerb,
	}
	target := auth.Target{
		Kind:  req.Msg.TargetKind,
		ID:    req.Msg.TargetId,
		Area:  req.Msg.TargetArea,
		Class: req.Msg.TargetClass,
	}
	if s.d.Policy == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("policy runtime not configured"))
	}
	trace := policy.Explain(ctx, s.d.Policy, principal, action, target)
	return connect.NewResponse(&authpb.ExplainAuthorizationResponse{
		Decision: trace.Decision, Reason: trace.Reason,
		RuleName: trace.RuleName, Steps: trace.Steps,
	}), nil
}
