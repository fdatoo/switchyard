package api_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
)

const (
	webauthnTestRPID   = "localhost"
	webauthnTestOrigin = "https://localhost"
)

type captureAppender struct {
	mu     sync.Mutex
	events []*eventv1.AuthEvent
}

func (a *captureAppender) AppendAuth(_ context.Context, ev *eventv1.AuthEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, ev)
	return nil
}

func (a *captureAppender) hasLoginFailedReason(reason string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ev := range a.events {
		if failed := ev.GetLoginFailed(); failed != nil && failed.GetReason() == reason {
			return true
		}
	}
	return false
}

func passkeyDeps(t *testing.T) (api.AuthDeps, *sql.DB, *captureAppender) {
	t.Helper()
	deps, db := newAuthTestDeps(t)
	w, err := wa.New(&wa.Config{
		RPID:          webauthnTestRPID,
		RPDisplayName: "Switchyard Test",
		RPOrigins:     []string{webauthnTestOrigin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
	require.NoError(t, err)
	appender := &captureAppender{}
	deps.Passkeys = credentials.NewPasskeys(db, w)
	deps.Challenges = credentials.NewChallengeStore(time.Minute)
	deps.Audit = audit.New(appender)
	return deps, db, appender
}

func TestAuthService_PasskeyRegisterThenLogin(t *testing.T) {
	deps, db, _ := passkeyDeps(t)
	seedUser(t, db, deps.Identity, "alice", "unused")
	svc := api.NewAuthService(deps)
	authenticator := newAPIVirtualAuthenticator(t, webauthnTestRPID)

	registerChallenge := startRegisterChallenge(t, svc, "alice")
	regBody := authenticator.register(t, registerChallenge, webauthnTestOrigin)
	registerCtx := auth.WithPrincipal(context.Background(), auth.Principal{
		ID: "user:alice", Kind: "user", Metadata: map[string]string{"session_id": "register-session"},
	})
	registerResp, err := svc.RegisterPasskey(registerCtx, connect.NewRequest(&authpb.RegisterPasskeyRequest{
		PublicKeyCredential: regBody,
		WebauthnChallengeId: registerChallenge.id,
		Label:               "platform",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, registerResp.Msg.CredentialId)

	loginResp := finishPasskeyLogin(t, svc, authenticator)
	require.NotEmpty(t, loginResp.Msg.SessionToken)
}

func TestAuthService_PasskeyLoginRejectsSignCountRegression(t *testing.T) {
	deps, db, appender := passkeyDeps(t)
	seedUser(t, db, deps.Identity, "alice", "unused")
	svc := api.NewAuthService(deps)
	authenticator := newAPIVirtualAuthenticator(t, webauthnTestRPID)

	registerChallenge := startRegisterChallenge(t, svc, "alice")
	regBody := authenticator.register(t, registerChallenge, webauthnTestOrigin)
	registerCtx := auth.WithPrincipal(context.Background(), auth.Principal{
		ID: "user:alice", Kind: "user", Metadata: map[string]string{"session_id": "register-session"},
	})
	_, err := svc.RegisterPasskey(registerCtx, connect.NewRequest(&authpb.RegisterPasskeyRequest{
		PublicKeyCredential: regBody,
		WebauthnChallengeId: registerChallenge.id,
		Label:               "platform",
	}))
	require.NoError(t, err)

	_ = finishPasskeyLogin(t, svc, authenticator)
	authenticator.signCount = 0

	_, err = finishPasskeyLoginExpectError(t, svc, authenticator)
	require.Error(t, err)
	require.True(t, appender.hasLoginFailedReason("sign_count_regression"))
}

type apiCredentialCreation struct {
	id      string
	options protocol.PublicKeyCredentialCreationOptions
}

func startRegisterChallenge(t *testing.T, svc *api.AuthService, slug string) apiCredentialCreation {
	t.Helper()
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		ID: "user:" + slug, Kind: "user", Metadata: map[string]string{"session_id": "register-session"},
	})
	resp, err := svc.StartWebAuthnChallenge(ctx, connect.NewRequest(&authpb.StartWebAuthnChallengeRequest{
		Intent:      "register",
		DisplayName: "Alice",
	}))
	require.NoError(t, err)
	var options protocol.PublicKeyCredentialCreationOptions
	require.NoError(t, json.Unmarshal(resp.Msg.Challenge, &options))
	require.NotEmpty(t, resp.Msg.WebauthnChallengeId)
	return apiCredentialCreation{id: resp.Msg.WebauthnChallengeId, options: options}
}

func finishPasskeyLogin(t *testing.T, svc *api.AuthService, authenticator *apiVirtualAuthenticator) *connect.Response[authpb.LoginResponse] {
	t.Helper()
	resp, err := finishPasskeyLoginExpectError(t, svc, authenticator)
	require.NoError(t, err)
	return resp
}

func finishPasskeyLoginExpectError(t *testing.T, svc *api.AuthService, authenticator *apiVirtualAuthenticator) (*connect.Response[authpb.LoginResponse], error) {
	t.Helper()
	rec := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/", nil)
	startResp, err := svc.StartWebAuthnChallenge(ctxWithRW(rec, startReq), connect.NewRequest(&authpb.StartWebAuthnChallengeRequest{
		Intent: "login",
	}))
	require.NoError(t, err)
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	var options protocol.PublicKeyCredentialRequestOptions
	require.NoError(t, json.Unmarshal(startResp.Msg.Challenge, &options))
	require.Len(t, options.AllowedCredentials, 0)

	body := authenticator.assert(t, options, []byte("alice"), webauthnTestOrigin)
	loginReq := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, c := range cookies {
		loginReq.AddCookie(c)
	}
	loginRec := httptest.NewRecorder()
	return svc.Login(ctxWithRW(loginRec, loginReq), connect.NewRequest(&authpb.LoginRequest{
		PublicKeyCredential: body,
		WebauthnChallengeId: startResp.Msg.WebauthnChallengeId,
	}))
}

type apiVirtualAuthenticator struct {
	rpID         string
	credentialID []byte
	signCount    uint32
	priv         *ecdsa.PrivateKey
}

func newAPIVirtualAuthenticator(t *testing.T, rpID string) *apiVirtualAuthenticator {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	id := make([]byte, 32)
	_, err = rand.Read(id)
	require.NoError(t, err)
	return &apiVirtualAuthenticator{rpID: rpID, credentialID: id, priv: priv}
}

const (
	apiFlagUP = 0x01
	apiFlagUV = 0x04
	apiFlagAT = 0x40
)

type apiCOSEPubKeyES256 struct {
	Kty int    `cbor:"1,keyasint"`
	Alg int    `cbor:"3,keyasint"`
	Crv int    `cbor:"-1,keyasint"`
	X   []byte `cbor:"-2,keyasint"`
	Y   []byte `cbor:"-3,keyasint"`
}

func (v *apiVirtualAuthenticator) cosePublicKey(t *testing.T) []byte {
	t.Helper()
	enc, err := apiCTAPEncMode().Marshal(apiCOSEPubKeyES256{
		Kty: int(webauthncose.EllipticKey),
		Alg: int(webauthncose.AlgES256),
		Crv: int(webauthncose.P256),
		X:   apiLeftPad(v.priv.X.Bytes(), 32),
		Y:   apiLeftPad(v.priv.Y.Bytes(), 32),
	})
	require.NoError(t, err)
	return enc
}

func apiLeftPad(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}

func apiCTAPEncMode() cbor.EncMode {
	em, _ := cbor.CTAP2EncOptions().EncMode()
	return em
}

func (v *apiVirtualAuthenticator) buildAuthenticatorData(t *testing.T, includeAttData bool) []byte {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(v.rpID))
	flags := byte(apiFlagUP | apiFlagUV)
	if includeAttData {
		flags |= apiFlagAT
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, rpIDHash[:]...)
	buf = append(buf, flags)
	count := make([]byte, 4)
	binary.BigEndian.PutUint32(count, v.signCount)
	buf = append(buf, count...)
	if includeAttData {
		buf = append(buf, make([]byte, 16)...)
		idLen := make([]byte, 2)
		binary.BigEndian.PutUint16(idLen, uint16(len(v.credentialID)))
		buf = append(buf, idLen...)
		buf = append(buf, v.credentialID...)
		buf = append(buf, v.cosePublicKey(t)...)
	}
	return buf
}

func (v *apiVirtualAuthenticator) register(t *testing.T, creation apiCredentialCreation, origin string) []byte {
	t.Helper()
	clientData := map[string]any{
		"type":      "webauthn.create",
		"challenge": creation.options.Challenge.String(),
		"origin":    origin,
	}
	clientDataJSON, err := json.Marshal(clientData)
	require.NoError(t, err)
	authData := v.buildAuthenticatorData(t, true)
	attObj, err := apiCTAPEncMode().Marshal(map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": authData,
	})
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"id":    apiBase64URL(v.credentialID),
		"rawId": apiBase64URL(v.credentialID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    apiBase64URL(clientDataJSON),
			"attestationObject": apiBase64URL(attObj),
		},
		"clientExtensionResults": map[string]any{},
	})
	require.NoError(t, err)
	return body
}

func (v *apiVirtualAuthenticator) assert(t *testing.T, assertion protocol.PublicKeyCredentialRequestOptions, userHandle []byte, origin string) []byte {
	t.Helper()
	v.signCount++
	clientData := map[string]any{
		"type":      "webauthn.get",
		"challenge": assertion.Challenge.String(),
		"origin":    origin,
	}
	clientDataJSON, err := json.Marshal(clientData)
	require.NoError(t, err)
	clientDataHash := sha256.Sum256(clientDataJSON)
	authData := v.buildAuthenticatorData(t, false)
	signed := append([]byte{}, authData...)
	signed = append(signed, clientDataHash[:]...)
	digest := sha256.Sum256(signed)
	r, s, err := ecdsa.Sign(rand.Reader, v.priv, digest[:])
	require.NoError(t, err)
	sig, err := apiEncodeECDSASig(r, s)
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"id":    apiBase64URL(v.credentialID),
		"rawId": apiBase64URL(v.credentialID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    apiBase64URL(clientDataJSON),
			"authenticatorData": apiBase64URL(authData),
			"signature":         apiBase64URL(sig),
			"userHandle":        apiBase64URL(userHandle),
		},
		"clientExtensionResults": map[string]any{},
	})
	require.NoError(t, err)
	return body
}

func apiEncodeECDSASig(r, s *big.Int) ([]byte, error) {
	type ecdsaSig struct {
		R, S *big.Int
	}
	return asn1.Marshal(ecdsaSig{R: r, S: s})
}

func apiBase64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
