// Command drivertest exercises a compiled Carport driver binary end-to-end.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	carportv1alpha1 "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "drivertest: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	binary     string
	instanceID string
	entityID   string
	cfgJSON    string
	scenario   string
	timeout    time.Duration
	jsonOut    bool
}

func run(args []string) error {
	cfg := config{
		instanceID: "test-instance",
		entityID:   "test.entity",
		cfgJSON:    "{}",
		scenario:   "happy-path",
		timeout:    30 * time.Second,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "run":
			if i+1 >= len(args) {
				return fmt.Errorf("run requires a binary path")
			}
			i++
			cfg.binary = args[i]
		case "--instance-id":
			i++
			cfg.instanceID = args[i]
		case "--entity-id":
			i++
			cfg.entityID = args[i]
		case "--config":
			i++
			cfg.cfgJSON = args[i]
		case "--scenario":
			i++
			cfg.scenario = args[i]
		case "--timeout":
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			cfg.timeout = d
		case "--json":
			cfg.jsonOut = true
		}
	}

	if cfg.binary == "" {
		return fmt.Errorf("usage: drivertest run <binary> [--scenario happy-path|reconnect] [--entity-id <id>] [--json]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	switch cfg.scenario {
	case "happy-path":
		return runHappyPath(ctx, cfg)
	case "reconnect":
		return runReconnect(ctx, cfg)
	default:
		return fmt.Errorf("unknown scenario %q; valid: happy-path, reconnect", cfg.scenario)
	}
}

type result struct {
	Scenario string `json:"scenario"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

func printResult(cfg config, r result) {
	if cfg.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(r)
	} else {
		if r.OK {
			fmt.Printf("PASS scenario=%s\n", r.Scenario)
		} else {
			fmt.Printf("FAIL scenario=%s error=%s\n", r.Scenario, r.Error)
		}
	}
}

func runHappyPath(ctx context.Context, cfg config) error {
	dir, err := os.MkdirTemp("", "ghdt-cli")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	sock := filepath.Join(dir, "s")
	secret := "cli-secret"

	cmd := exec.CommandContext(ctx, cfg.binary)
	cmd.Env = append(os.Environ(),
		"GOHOME_CARPORT_SOCKET="+sock,
		"GOHOME_CARPORT_SECRET="+secret,
		"GOHOME_CARPORT_INSTANCE_ID="+cfg.instanceID,
		"GOHOME_CARPORT_INSTANCE_CONFIG="+cfg.cfgJSON,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start driver: %w", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	client, cc, err := dialSocket(ctx, sock)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}
	defer func() { _ = cc.Close() }()

	// Verify Handshake returned a manifest.
	hsResp, err := doHandshake(ctx, client, secret, cfg.instanceID)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: "handshake: " + err.Error()})
		return err
	}
	if hsResp.GetManifest().GetName() == "" {
		err := fmt.Errorf("manifest name is empty")
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}

	// Send one command per declared capability (to "test.entity") and assert ok.
	stream, err := client.Run(ctx)
	if err != nil {
		printResult(cfg, result{Scenario: "happy-path", Error: "open Run stream: " + err.Error()})
		return err
	}

	for i, cap := range hsResp.GetManifest().GetSupportedCapabilities() {
		cmdID := fmt.Sprintf("cli-cmd-%d", i)
		if err := stream.Send(&carportv1alpha1.HostToDriver{
			Kind: &carportv1alpha1.HostToDriver_Command{
				Command: &carportv1alpha1.Command{
					CommandId:  cmdID,
					EntityId:   cfg.entityID,
					Capability: cap,
				},
			},
		}); err != nil {
			printResult(cfg, result{Scenario: "happy-path", Error: "send command: " + err.Error()})
			return err
		}

		// Drain until we get the CommandResult for this command.
		if err := drainUntilResult(stream, cmdID, 5*time.Second); err != nil {
			printResult(cfg, result{Scenario: "happy-path", Error: fmt.Sprintf("cap %q: %v", cap, err)})
			return err
		}
	}

	// Graceful shutdown.
	sCtx, sCancel := context.WithTimeout(ctx, 5*time.Second)
	defer sCancel()
	resp, err := client.Shutdown(sCtx, &carportv1alpha1.ShutdownRequest{GraceMs: 3000})
	if err != nil || !resp.GetAcknowledged() {
		err = fmt.Errorf("shutdown: %v acknowledged=%v", err, resp.GetAcknowledged())
		printResult(cfg, result{Scenario: "happy-path", Error: err.Error()})
		return err
	}

	printResult(cfg, result{Scenario: "happy-path", OK: true, Detail: fmt.Sprintf("entities=%d caps=%d", len(hsResp.GetInitialEntities()), len(hsResp.GetManifest().GetSupportedCapabilities()))})
	return nil
}

func runReconnect(ctx context.Context, cfg config) error {
	dir, err := os.MkdirTemp("", "ghdt-cli")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	sock := filepath.Join(dir, "s")
	secret := "cli-secret"

	cmd := exec.CommandContext(ctx, cfg.binary)
	cmd.Env = append(os.Environ(),
		"GOHOME_CARPORT_SOCKET="+sock,
		"GOHOME_CARPORT_SECRET="+secret,
		"GOHOME_CARPORT_INSTANCE_ID="+cfg.instanceID,
		"GOHOME_CARPORT_INSTANCE_CONFIG="+cfg.cfgJSON,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start driver: %w", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	// First handshake.
	_, cc1, err := dialSocket(ctx, sock)
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: "first handshake: " + err.Error()})
		return err
	}
	resp1, err := doHandshake(ctx, carportv1alpha1.NewDriverClient(cc1), secret, cfg.instanceID)
	if err != nil {
		_ = cc1.Close()
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}
	entityCount := len(resp1.GetInitialEntities())
	_ = cc1.Close()

	// Wait for driver to reconnect.
	time.Sleep(300 * time.Millisecond)

	// Second handshake.
	_, cc2, err := dialSocket(ctx, sock)
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: "reconnect handshake: " + err.Error()})
		return err
	}
	resp2, err := doHandshake(ctx, carportv1alpha1.NewDriverClient(cc2), secret, cfg.instanceID)
	_ = cc2.Close()
	if err != nil {
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}

	if len(resp2.GetInitialEntities()) != entityCount {
		err := fmt.Errorf("entity count after reconnect: got %d, want %d", len(resp2.GetInitialEntities()), entityCount)
		printResult(cfg, result{Scenario: "reconnect", Error: err.Error()})
		return err
	}

	printResult(cfg, result{Scenario: "reconnect", OK: true, Detail: fmt.Sprintf("entities=%d", entityCount)})
	return nil
}

func dialSocket(ctx context.Context, sock string) (carportv1alpha1.DriverClient, *grpc.ClientConn, error) {
	// Poll for socket.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	cc, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	return carportv1alpha1.NewDriverClient(cc), cc, nil
}

func doHandshake(ctx context.Context, client carportv1alpha1.DriverClient, secret, instanceID string) (*carportv1alpha1.HandshakeResponse, error) {
	hCtx, hCancel := context.WithTimeout(ctx, 5*time.Second)
	defer hCancel()
	return client.Handshake(hCtx, &carportv1alpha1.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		HandshakeSecret: secret,
		InstanceId:      instanceID,
		InstanceConfig:  []byte("{}"),
	})
}

func drainUntilResult(stream carportv1alpha1.Driver_RunClient, commandID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if r := msg.GetResult(); r != nil && r.GetCommandId() == commandID {
			if !r.GetOk() {
				return fmt.Errorf("command failed: %s", r.GetErrorMessage())
			}
			return nil
		}
		// StateChanged or other messages — continue draining.
	}
	return fmt.Errorf("timeout waiting for result of command %q", commandID)
}
