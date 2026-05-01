package credentials_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth/credentials"
)

// virtualAuthenticator is a minimal ES256 authenticator used to drive the
// register-then-authenticate flow against the real go-webauthn library.
type virtualAuthenticator struct {
	rpID         string
	credentialID []byte
	signCount    uint32
	priv         *ecdsa.PrivateKey
}

func newVirtualAuthenticator(t *testing.T, rpID string) *virtualAuthenticator {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	id := make([]byte, 32)
	_, err = rand.Read(id)
	require.NoError(t, err)
	return &virtualAuthenticator{rpID: rpID, credentialID: id, priv: priv}
}

const (
	flagUP = 0x01
	flagUV = 0x04
	flagAT = 0x40
)

// cosePubKeyES256 is the canonical CTAP2 COSE encoding for an ES256 public key
// in the order (1, 3, -1, -2, -3) required by the spec.
type cosePubKeyES256 struct {
	Kty int    `cbor:"1,keyasint"`
	Alg int    `cbor:"3,keyasint"`
	Crv int    `cbor:"-1,keyasint"`
	X   []byte `cbor:"-2,keyasint"`
	Y   []byte `cbor:"-3,keyasint"`
}

func (v *virtualAuthenticator) cosePublicKey(t *testing.T) []byte {
	t.Helper()
	x := v.priv.X.Bytes()
	y := v.priv.Y.Bytes()
	x = leftPad(x, 32)
	y = leftPad(y, 32)
	enc, err := ctapEncMode().Marshal(cosePubKeyES256{
		Kty: int(webauthncose.EllipticKey),
		Alg: int(webauthncose.AlgES256),
		Crv: int(webauthncose.P256),
		X:   x,
		Y:   y,
	})
	require.NoError(t, err)
	return enc
}

func leftPad(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}

func ctapEncMode() cbor.EncMode {
	em, _ := cbor.CTAP2EncOptions().EncMode()
	return em
}

func (v *virtualAuthenticator) buildAuthenticatorData(t *testing.T, includeAttData bool) []byte {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(v.rpID))
	flags := byte(flagUP | flagUV)
	if includeAttData {
		flags |= flagAT
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, rpIDHash[:]...)
	buf = append(buf, flags)
	count := make([]byte, 4)
	binary.BigEndian.PutUint32(count, v.signCount)
	buf = append(buf, count...)

	if includeAttData {
		// AAGUID: 16 zero bytes (no attestation).
		buf = append(buf, make([]byte, 16)...)
		// Credential ID length (2 bytes) + credential ID.
		idLen := make([]byte, 2)
		binary.BigEndian.PutUint16(idLen, uint16(len(v.credentialID)))
		buf = append(buf, idLen...)
		buf = append(buf, v.credentialID...)
		// COSE public key.
		buf = append(buf, v.cosePublicKey(t)...)
	}
	return buf
}

// register builds a CredentialCreationResponse JSON body matching the given
// CredentialCreation challenge, using "none" attestation.
func (v *virtualAuthenticator) register(t *testing.T, creation *protocol.CredentialCreation, origin string) []byte {
	t.Helper()
	clientData := map[string]any{
		"type":      "webauthn.create",
		"challenge": creation.Response.Challenge.String(),
		"origin":    origin,
	}
	clientDataJSON, err := json.Marshal(clientData)
	require.NoError(t, err)

	authData := v.buildAuthenticatorData(t, true)

	attObj := map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": authData,
	}
	attObjBytes, err := ctapEncMode().Marshal(attObj)
	require.NoError(t, err)

	resp := map[string]any{
		"id":    base64URL(v.credentialID),
		"rawId": base64URL(v.credentialID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    base64URL(clientDataJSON),
			"attestationObject": base64URL(attObjBytes),
		},
		"clientExtensionResults": map[string]any{},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)
	return body
}

// assert builds a CredentialAssertionResponse JSON body for the given assertion.
func (v *virtualAuthenticator) assert(t *testing.T, assertion *protocol.CredentialAssertion, userHandle []byte, origin string) []byte {
	t.Helper()
	v.signCount++

	clientData := map[string]any{
		"type":      "webauthn.get",
		"challenge": assertion.Response.Challenge.String(),
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
	sig, err := encodeECDSASig(r, s)
	require.NoError(t, err)

	resp := map[string]any{
		"id":    base64URL(v.credentialID),
		"rawId": base64URL(v.credentialID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    base64URL(clientDataJSON),
			"authenticatorData": base64URL(authData),
			"signature":         base64URL(sig),
			"userHandle":        base64URL(userHandle),
		},
		"clientExtensionResults": map[string]any{},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)
	return body
}

func encodeECDSASig(r, s *big.Int) ([]byte, error) {
	type ecdsaSig struct {
		R, S *big.Int
	}
	return asn1.Marshal(ecdsaSig{R: r, S: s})
}

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestWebAuthn_RegisterThenAuthenticate(t *testing.T) {
	const (
		rpID   = "localhost"
		origin = "https://localhost"
	)
	w, err := wa.New(&wa.Config{
		RPID:          rpID,
		RPDisplayName: "Switchyard Test",
		RPOrigins:     []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
	require.NoError(t, err)

	db := setupAuthDB(t)
	pk := credentials.NewPasskeys(db, w)
	ctx := context.Background()

	const slug = "fdatoo"
	auth := newVirtualAuthenticator(t, rpID)

	creation, sd, err := pk.BeginRegistration(ctx, slug, "FBI Agent")
	require.NoError(t, err)

	regBody := auth.register(t, creation, origin)
	parsedReg, err := protocol.ParseCredentialCreationResponseBytes(regBody)
	require.NoError(t, err)

	stored, err := pk.FinishRegistration(ctx, slug, "yubikey", sd, parsedReg)
	require.NoError(t, err)
	require.Equal(t, slug, stored.UserSlug)
	require.Equal(t, auth.credentialID, stored.CredentialID)

	assertion, lsd, err := pk.BeginLogin(ctx)
	require.NoError(t, err)

	loginBody := auth.assert(t, assertion, []byte(slug), origin)
	parsedLogin, err := protocol.ParseCredentialRequestResponseBytes(loginBody)
	require.NoError(t, err)

	gotSlug, err := pk.FinishLogin(ctx, lsd, parsedLogin)
	require.NoError(t, err)
	require.Equal(t, slug, gotSlug)

	list, err := pk.List(ctx, slug)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].LastUsedAt)
	require.Equal(t, uint32(1), list[0].SignCount)

	require.NoError(t, pk.Remove(ctx, stored.CredentialID))
	list, err = pk.List(ctx, slug)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestWebAuthn_FinishLogin_RejectsSignCountRegression(t *testing.T) {
	const (
		rpID   = "localhost"
		origin = "https://localhost"
	)
	w, err := wa.New(&wa.Config{
		RPID:          rpID,
		RPDisplayName: "Switchyard Test",
		RPOrigins:     []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
	require.NoError(t, err)

	db := setupAuthDB(t)
	pk := credentials.NewPasskeys(db, w)
	ctx := context.Background()

	auth := newVirtualAuthenticator(t, rpID)

	creation, sd, err := pk.BeginRegistration(ctx, "fdatoo", "FBI Agent")
	require.NoError(t, err)
	regBody := auth.register(t, creation, origin)
	parsedReg, err := protocol.ParseCredentialCreationResponseBytes(regBody)
	require.NoError(t, err)
	_, err = pk.FinishRegistration(ctx, "fdatoo", "yubikey", sd, parsedReg)
	require.NoError(t, err)

	// First login advances stored sign count to 1.
	assertion, lsd, err := pk.BeginLogin(ctx)
	require.NoError(t, err)
	loginBody := auth.assert(t, assertion, []byte("fdatoo"), origin)
	parsedLogin, err := protocol.ParseCredentialRequestResponseBytes(loginBody)
	require.NoError(t, err)
	_, err = pk.FinishLogin(ctx, lsd, parsedLogin)
	require.NoError(t, err)

	// Reset to 0 so the next assert produces count=1 == stored count: triggers CloneWarning.
	auth.signCount = 0

	assertion2, lsd2, err := pk.BeginLogin(ctx)
	require.NoError(t, err)
	loginBody2 := auth.assert(t, assertion2, []byte("fdatoo"), origin)
	parsedLogin2, err := protocol.ParseCredentialRequestResponseBytes(loginBody2)
	require.NoError(t, err)
	_, err = pk.FinishLogin(ctx, lsd2, parsedLogin2)
	require.ErrorIs(t, err, credentials.ErrSignCountRegression)
}

func TestChallengeStore_StoreAndConsume(t *testing.T) {
	store := credentials.NewChallengeStore(time.Minute)
	ctx := context.Background()

	id, err := store.Store(ctx, "session-1", []byte("payload-1"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if id == "" {
		t.Fatal("Store returned empty id")
	}

	got, err := store.Consume(ctx, "session-1", id)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if string(got) != "payload-1" {
		t.Errorf("Consume payload = %q, want %q", got, "payload-1")
	}

	// Second consume must fail (replay protection).
	if _, err := store.Consume(ctx, "session-1", id); err == nil {
		t.Error("expected error on replay consume")
	}
}

func TestChallengeStore_Expires(t *testing.T) {
	store := credentials.NewChallengeStore(10 * time.Millisecond)
	ctx := context.Background()
	id, _ := store.Store(ctx, "s", []byte("p"))
	time.Sleep(20 * time.Millisecond)
	if _, err := store.Consume(ctx, "s", id); err == nil {
		t.Error("expected error consuming expired challenge")
	}
}

func TestChallengeStore_RejectsCrossSessionConsume(t *testing.T) {
	store := credentials.NewChallengeStore(time.Minute)
	ctx := context.Background()
	id, _ := store.Store(ctx, "session-1", []byte("p"))
	if _, err := store.Consume(ctx, "session-2", id); err == nil {
		t.Error("expected error consuming with wrong session")
	}
}
