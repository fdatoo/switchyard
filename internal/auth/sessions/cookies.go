package sessions

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrSessionInvalid means a session cookie is malformed or fails signature checks.
	ErrSessionInvalid = errors.New("sessions: invalid")
	// ErrSessionExpired means a valid session is past its expiration time.
	ErrSessionExpired = errors.New("sessions: expired")
	// ErrSessionReplay means a refresh token was reused after rotation.
	ErrSessionReplay = errors.New("sessions: refresh replay detected")
)

// AccessClaim is what the access cookie carries (HMAC-signed).
type AccessClaim struct {
	SessionID  string
	UserSlug   string
	AuthMethod string
	Exp        int64 // unix seconds
}

func encodeAccessCookie(c AccessClaim, key []byte) string {
	payload := fmt.Sprintf("%s|%s|%s|%d", c.SessionID, c.UserSlug, c.AuthMethod, c.Exp)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func decodeAccessCookie(value string, key []byte) (AccessClaim, error) {
	dot := strings.IndexByte(value, '.')
	if dot < 0 {
		return AccessClaim{}, ErrSessionInvalid
	}
	payloadB64, sig := value[:dot], value[dot+1:]
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return AccessClaim{}, ErrSessionInvalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payloadBytes)
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return AccessClaim{}, ErrSessionInvalid
	}
	parts := strings.SplitN(string(payloadBytes), "|", 4)
	if len(parts) != 4 {
		return AccessClaim{}, ErrSessionInvalid
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return AccessClaim{}, ErrSessionInvalid
	}
	return AccessClaim{SessionID: parts[0], UserSlug: parts[1], AuthMethod: parts[2], Exp: exp}, nil
}

// RefreshCookie carries the refresh secret in plaintext.
//
// The server-stored hash is authoritative; this type only represents the
// client cookie value before verification.
type RefreshCookie struct {
	SessionID     string
	RefreshSecret string
}

func encodeRefreshCookie(c RefreshCookie) string {
	return c.SessionID + "." + c.RefreshSecret
}

func decodeRefreshCookie(value string) (RefreshCookie, error) {
	dot := strings.IndexByte(value, '.')
	if dot < 0 {
		return RefreshCookie{}, ErrSessionInvalid
	}
	return RefreshCookie{SessionID: value[:dot], RefreshSecret: value[dot+1:]}, nil
}

func deadline(now time.Time, ttl time.Duration) int64 { return now.Add(ttl).Unix() }
