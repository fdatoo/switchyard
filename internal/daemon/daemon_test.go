package daemon_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/daemon"
	"github.com/fdatoo/gohome/internal/observability"
)

func TestDaemon_StartsAndShutsDownCleanly(t *testing.T) {
	dir := shortTempDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()

	adminPort := freeTCPPort(t)
	d := daemon.New(daemon.Config{
		DataDir:    dir,
		LogLevel:   slog.LevelInfo,
		LogFormat:  "json",
		AdminPort:  adminPort,
		SocketPath: fmt.Sprintf("gohomed-%d.sock", os.Getpid()),
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Poll /health until the daemon reports ready (Phase 5). The race detector
	// can slow migrations enough that cancelling earlier races with goose.
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(20 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		cancel()
		<-done
		t.Fatal("daemon did not report ready within 20s")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestDaemon_APIVersion(t *testing.T) {
	dir := shortTempDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()

	adminPort := freeTCPPort(t)
	d := daemon.New(daemon.Config{
		DataDir:    dir,
		LogLevel:   slog.LevelInfo,
		LogFormat:  "json",
		AdminPort:  adminPort,
		SocketPath: fmt.Sprintf("gohomed-%d.sock", os.Getpid()),
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Poll /health until ready.
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(20 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL) //nolint:noctx
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		cancel()
		<-done
		t.Fatal("daemon did not report ready within 20s")
	}

	// Dial the Connect API over the Unix-domain socket.
	sock := filepath.Join(dir, "gohomed.sock")
	httpClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	client := gohomev1alpha1connect.NewSystemServiceClient(httpClient, "http://unix")
	resp, err := client.Version(context.Background(), connect.NewRequest(&v1.VersionRequest{}))
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.Msg == nil {
		t.Fatal("expected non-nil VersionResponse")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// shortTempDir creates a temporary directory under /tmp with a short path.
// macOS limits Unix socket paths to 104 bytes; t.TempDir() paths via $TMPDIR
// are too long on macOS runners. /tmp is a shorter base on both Linux and macOS.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ghd")
	if err != nil {
		// Fall back to the standard temp dir if /tmp is not writable.
		dir = t.TempDir()
		return dir
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
