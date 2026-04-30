package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	errorv1 "github.com/fdatoo/gohome/gen/gohome/error/v1alpha1"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/storage"
)

func openReadOnlyDB(ctx context.Context, dataDir string) (*sql.DB, error) {
	dataDir = expandHome(dataDir)
	path := filepath.Join(dataDir, "gohome.db")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no database at %s — is gohomed running?", path)
	}
	return storage.OpenReadOnly(ctx, storage.Config{Path: path})
}

func expandHome(p string) string {
	if len(p) == 0 || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}

func nullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func nullMetrics() *observability.Metrics {
	return observability.NewMetrics()
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

const defaultRPCTimeout = 30 * time.Second

// ResolveEndpoint picks the API endpoint to dial.
// Precedence: explicit flag value > GOHOME_ENDPOINT env > unix://<dataDir>/gohomed.sock.
func ResolveEndpoint(flagValue, dataDir string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("GOHOME_ENDPOINT"); env != "" {
		return env
	}
	return "unix://" + dataDir + "/gohomed.sock"
}

// Dial returns an http.Client and a base URL for a Connect-Go client constructor.
// For unix:// endpoints the transport dials the socket; base URL is "http://unix".
// For http(s):// or tcp:// the base URL is the input (tcp:// → http://).
func Dial(_ context.Context, endpoint string) (*http.Client, string, error) {
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		sock := strings.TrimPrefix(endpoint, "unix://")
		return &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", sock)
			},
		}}, "http://unix", nil
	case strings.HasPrefix(endpoint, "tcp://"):
		return http.DefaultClient, "http://" + strings.TrimPrefix(endpoint, "tcp://"), nil
	default:
		return http.DefaultClient, endpoint, nil
	}
}

func renderConnectErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return err
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if ed, ok := v.(*errorv1.ErrorDetail); ok && ed.Reason != "" {
			return fmt.Errorf("%s: %s (request_id=%s)", ce.Code(), ed.Reason, ed.RequestId)
		}
	}
	return fmt.Errorf("%s: %s", ce.Code(), ce.Message())
}
