package api_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fdatoo/gohome/internal/api"
)

type fakeWebhookRouter struct {
	secrets  map[string]string // slug → secret
	maxBytes int64
}
type fakeAppender struct {
	app []api.AppendedWebhook
}

func (f *fakeWebhookRouter) SecretFor(slug string) (string, bool) {
	s, ok := f.secrets[slug]
	return s, ok
}
func (f *fakeWebhookRouter) MaxBodyBytes() int64 { return f.maxBytes }
func (f *fakeAppender) AppendWebhook(_ context.Context, w api.AppendedWebhook) error {
	f.app = append(f.app, w)
	return nil
}

func sign(secret, body string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(body))
	return "v1=" + hex.EncodeToString(h.Sum(nil))
}

func TestWebhook_Accepts_ValidSignature(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	body := `{"x":1}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString(body))
	req.Header.Set("X-GoHome-Signature", sign("shh", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if len(app.app) != 1 || app.app[0].Slug != "foo" {
		t.Errorf("appended = %+v", app.app)
	}
}

func TestWebhook_Rejects_BadSignature(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString("body"))
	req.Header.Set("X-GoHome-Signature", "v1=deadbeef")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rr.Code)
	}
	if len(app.app) != 0 {
		t.Error("appended on bad sig")
	}
}

func TestWebhook_Rejects_UnknownSlug(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/nope", bytes.NewBufferString(""))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d", rr.Code)
	}
}

func TestWebhook_Rejects_BodyTooLarge(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 4}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	body := "more than four bytes"
	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString(body))
	req.Header.Set("X-GoHome-Signature", sign("shh", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge && rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rr.Code)
	}
}

func TestWebhook_RejectsNonPost(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", rr.Code)
	}
}
