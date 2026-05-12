//go:build integration

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

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

// TestConfigSubscribe_FilesystemTrigger boots a real daemon with a minimal
// config, opens a ConfigService.Subscribe stream, writes a new automation
// file to disk, triggers a reload via the Reload RPC, and asserts that a
// ConfigChanged event arrives within 15 seconds.
//
// Pipeline under test:
//
//	Reload RPC → Reloader.Trigger → debounce 250 ms → Manager.Apply
//	→ OnApplied → ConfigPubsub.Publish → ConfigService.Subscribe → client
//
// Design notes:
//
//   - We use the Reload RPC (not the filesystem watcher) to trigger the
//     re-evaluation.  The watcher polls every 500 ms and only watches files
//     registered at daemon startup; the Reload RPC is a cleaner synchronous
//     trigger for an integration test.
//
//   - The Subscribe stream is opened over the Unix-domain socket (same path
//     as TestDaemon_APIVersion) to inherit LocalPeerCred authentication.
//     Streaming over HTTP/1.1 blocks in CallServerStream until the first
//     server message, so we open the stream in a goroutine and trigger the
//     Reload in parallel so the first ConfigChanged event unblocks the
//     stream immediately.
//
//   - The TCP port embedded in main.pkl must not be port 8080 (the Pkl
//     default) because 8080 is often already in use on developer machines.
func TestConfigSubscribe_FilesystemTrigger(t *testing.T) {
	dir := shortTempDir(t)

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "automations"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Allocate a free TCP port for the Connect listener embedded in main.pkl.
	// The Pkl default is "127.0.0.1:8080" which can collide with other
	// processes.
	connectPort := freeTCPPort(t)

	mainPkl := fmt.Sprintf(`amends "switchyard:config"

listener {
  tcp {
    bind = "127.0.0.1:%d"
  }
}
`, connectPort)
	if err := os.WriteFile(filepath.Join(configDir, "main.pkl"), []byte(mainPkl), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()
	adminPort := freeTCPPort(t)

	d := daemon.New(daemon.Config{
		DataDir:   dir,
		ConfigDir: configDir,
		LogLevel:  slog.LevelInfo,
		LogFormat: "json",
		AdminPort: adminPort,
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Poll /health until the daemon reports ready (phase 5).
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(60 * time.Second)
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
		t.Fatal("daemon did not report ready within 60s")
	}
	t.Log("daemon ready")

	// Connect via Unix-domain socket for LocalPeerCred authentication.
	// The UDS path is <DataDir>/switchyardd.sock (daemon default).
	sock := filepath.Join(dir, "switchyardd.sock")
	udsClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	cfgClient := switchyardv1alpha1connect.NewConfigServiceClient(udsClient, "http://unix")

	// Write the automation file before opening the stream so that when the
	// Reload fires and Apply runs, the new automation is already on disk.
	const automationPkl = `amends "switchyard:automation"
import "switchyard:automations" as auto

id = "e2e"
enabled = true
triggers {
  new auto.EventTrigger { kind = "sun.sunset" }
}
actions {}
`
	automationPath := filepath.Join(configDir, "automations", "e2e.pkl")
	if err := os.WriteFile(automationPath, []byte(automationPkl), 0o644); err != nil {
		t.Fatalf("write automation file: %v", err)
	}
	t.Log("automation file written")

	// Open the Subscribe stream in a goroutine because CallServerStream over
	// HTTP/1.1 blocks until the server sends the first message (it cannot
	// return just from the 200 OK headers in chunked streaming mode).  We
	// need the Reload RPC to fire in parallel to produce the first message.
	//
	// The overall test deadline is 15 s; the stream context is set to the
	// same deadline so stream.Receive() times out cleanly.
	testDeadline := 15 * time.Second
	streamCtx, streamCancel := context.WithTimeout(ctx, testDeadline)
	defer streamCancel()

	type streamResult struct {
		gotChanged bool
		err        error
	}
	streamCh := make(chan streamResult, 1)

	go func() {
		stream, err := cfgClient.Subscribe(streamCtx, connect.NewRequest(&v1.SubscribeConfigRequest{}))
		if err != nil {
			streamCh <- streamResult{err: fmt.Errorf("Subscribe: %w", err)}
			return
		}
		defer func() { _ = stream.Close() }()

		for stream.Receive() {
			msg := stream.Msg()
			if msg.GetChanged() != nil {
				t.Logf("ConfigChanged received: at_unix_ms=%d", msg.GetChanged().GetAtUnixMs())
				streamCh <- streamResult{gotChanged: true}
				return
			}
			t.Log("heartbeat received; still waiting for ConfigChanged")
		}
		if streamErr := stream.Err(); streamErr != nil {
			streamCh <- streamResult{err: fmt.Errorf("stream error: %w", streamErr)}
			return
		}
		streamCh <- streamResult{err: fmt.Errorf("stream closed without ConfigChanged")}
	}()

	// Give the stream goroutine a moment to establish the connection, then
	// trigger the config reload.  The small sleep is not load-bearing: the
	// Reloader debounces for 250 ms, so even if Reload fires slightly before
	// the stream is open, the event lands in the pubsub buffer (cap 16) and
	// is delivered when Subscribe's select loop runs.
	time.Sleep(200 * time.Millisecond)

	// Trigger config reload via the Reload RPC (routes through Reloader →
	// Manager.Apply → OnApplied → ConfigPubsub.Publish).
	_, err := cfgClient.Reload(ctx, connect.NewRequest(&v1.ReloadConfigRequest{}))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	t.Log("Reload RPC sent")

	// Wait for the stream goroutine to report success or failure.
	select {
	case res := <-streamCh:
		if res.err != nil {
			t.Fatalf("stream goroutine: %v", res.err)
		}
		if !res.gotChanged {
			t.Fatal("stream goroutine returned without ConfigChanged")
		}
	case <-time.After(testDeadline):
		t.Fatal("timed out waiting for ConfigChanged event")
	}
}
