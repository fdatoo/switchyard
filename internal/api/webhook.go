package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/fdatoo/switchyard/internal/observability"
)

// WebhookRouter knows which slugs are registered and their HMAC secret.
type WebhookRouter interface {
	SecretFor(slug string) (string, bool)
	MaxBodyBytes() int64
}

// WebhookAppender persists the inbound webhook as a WebhookReceived event.
type WebhookAppender interface {
	AppendWebhook(ctx context.Context, w AppendedWebhook) error
}

// AppendedWebhook is the parsed and verified inbound webhook payload.
type AppendedWebhook struct {
	Slug     string
	Body     []byte
	Headers  map[string]string
	SourceIP string
}

// WebhookMetrics is optional; nil is fine in tests.
type WebhookMetrics interface {
	Inc(slug, result string)
}

// NewWebhookHandler returns an http.Handler mounted at /webhooks/.
func NewWebhookHandler(router WebhookRouter, app WebhookAppender, m WebhookMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			incWebhook(m, "", "method_not_allowed")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		slug := strings.TrimPrefix(r.URL.Path, "/webhooks/")
		if slug == "" || strings.Contains(slug, "/") {
			incWebhook(m, slug, "bad_slug")
			http.Error(w, "bad slug", http.StatusBadRequest)
			return
		}
		secret, ok := router.SecretFor(slug)
		if !ok {
			incWebhook(m, slug, "unknown_slug")
			http.Error(w, "unknown slug", http.StatusNotFound)
			return
		}

		max := router.MaxBodyBytes()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				incWebhook(m, slug, "too_large")
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			incWebhook(m, slug, "bad_body")
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		sig := r.Header.Get("X-Switchyard-Signature")
		if !verifySignature(secret, body, sig) {
			incWebhook(m, slug, "bad_signature")
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		id, _ := observability.RequestIDFromContext(ctx)
		_ = id

		headers := map[string]string{
			"content-type": r.Header.Get("Content-Type"),
			"user-agent":   r.Header.Get("User-Agent"),
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			headers["x-forwarded-for"] = xff
		}

		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if err := app.AppendWebhook(ctx, AppendedWebhook{
			Slug:     slug,
			Body:     body,
			Headers:  headers,
			SourceIP: ip,
		}); err != nil {
			incWebhook(m, slug, "append_failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		incWebhook(m, slug, "accepted")
		w.WriteHeader(http.StatusAccepted)
	})
}

func verifySignature(secret string, body []byte, header string) bool {
	const prefix = "v1="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want := header[len(prefix):]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(want))
}

func incWebhook(m WebhookMetrics, slug, result string) {
	if m != nil {
		m.Inc(slug, result)
	}
}
