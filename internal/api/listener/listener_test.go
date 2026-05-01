package listener_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/api/listener"
)

func TestListener_HealthzOnTCP(t *testing.T) {
	dir := t.TempDir()
	cfg := listener.Config{
		UDSPath: filepath.Join(dir, "sock"),
		UDSMode: 0o600,
		TCPBind: "127.0.0.1:0",
	}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	resp, err := http.Get("http://" + l.TCPAddr().String() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, body = %q", resp.StatusCode, b)
	}
}

func TestListener_UDSFileMode(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("stat sock: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
}

func TestListener_HealthzOnUDS(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}}
	resp, err := client.Get("http://unix/healthz")
	if err != nil {
		t.Fatalf("GET over UDS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestListener_ShutdownRemovesSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()
	shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
	defer sc()
	_ = l.Shutdown(shutdownCtx)

	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("sock still exists after shutdown, err = %v", err)
	}
}
