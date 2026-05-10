//go:build integration

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/cli"
	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

var updateCLIGolden = flag.Bool("update", false, "rewrite CLI golden files")

const cliGoldenHelperEnv = "SWITCHYARD_CLI_GOLDEN_HELPER"

type cliGoldenFixture struct {
	dataDir string
	sock    string
	cancel  context.CancelFunc
	done    <-chan error
	logs    *bytes.Buffer
}

type cliGoldenCase struct {
	name    string
	args    []string
	timeout time.Duration
}

func TestCLIGoldenHelper(t *testing.T) {
	if os.Getenv(cliGoldenHelperEnv) != "1" {
		return
	}
	idx := -1
	for i, arg := range os.Args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx < 0 {
		fmt.Fprintln(os.Stderr, "missing -- before CLI args")
		os.Exit(2)
	}
	timeout := 10 * time.Second
	if raw := os.Getenv("SWITCHYARD_CLI_GOLDEN_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad helper timeout %q: %v\n", raw, err)
			os.Exit(2)
		}
		timeout = d
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	root := cli.NewRoot()
	root.SetContext(ctx)
	root.SetArgs(os.Args[idx+1:])
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestCLIGoldenFixtures(t *testing.T) {
	if os.Getenv(cliGoldenHelperEnv) == "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	fx := startCLIGoldenDaemon(t, ctx)
	defer func() {
		fx.cancel()
		select {
		case err := <-fx.done:
			if err != nil {
				t.Fatalf("daemon exited with error: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("daemon did not stop")
		}
	}()

	cases := []cliGoldenCase{
		{name: "version", args: []string{"version"}},
		{name: "root-error", args: []string{"no-such-command"}},
		{name: "system-version", args: []string{"system", "version"}},
		{name: "system-health", args: []string{"system", "health"}},
		{name: "system-error", args: []string{"system", "health", "--endpoint", "unix:///tmp/switchyard-missing.sock"}},
		{name: "events-query", args: []string{"events", "query", "--kind", "config", "--limit", "1"}},
		{name: "events-inspect", args: []string{"events", "inspect", "1"}},
		{name: "events-export", args: []string{"events", "export", "--to", "1"}},
		{name: "events-tail", args: []string{"events", "tail", "--kind", "script_finished"}, timeout: 600 * time.Millisecond},
		{name: "events-error", args: []string{"events", "inspect", "not-a-number"}},
		{name: "state-get", args: []string{"state", "get", "test_light"}},
		{name: "state-dump", args: []string{"state", "dump"}},
		{name: "state-error", args: []string{"state", "get", "missing.entity"}},
		{name: "registry-list-devices", args: []string{"registry", "list", "devices"}},
		{name: "registry-list-entities", args: []string{"registry", "list", "entities"}},
		{name: "registry-list-drivers", args: []string{"registry", "list", "drivers"}},
		{name: "registry-show", args: []string{"registry", "show", "test_light"}},
		{name: "registry-error", args: []string{"registry", "show", "missing.entity"}},
		{name: "snapshot-create", args: []string{"snapshot", "create", "--owner", "state_cache", "--reason", "cli-golden"}},
		{name: "snapshot-list", args: []string{"snapshot", "list"}},
		{name: "snapshot-error", args: []string{"snapshot", "list", "--data-dir", filepath.Join(fx.dataDir, "missing")}},
		{name: "driver-list", args: []string{"driver", "list"}},
		{name: "driver-status", args: []string{"driver", "status", "testdriver-main"}},
		{name: "command-send", args: []string{"command", "send", "test_light", "turn_on", "--arg", "brightness=128"}},
		{name: "command-error", args: []string{"command", "send", "missing.entity", "turn_on"}},
		{name: "driver-restart", args: []string{"driver", "restart", "testdriver-main", "--reason", "cli-golden"}},
		{name: "driver-error", args: []string{"driver", "restart", "missing-driver"}},
		{name: "config-validate", args: []string{"config", "validate"}},
		{name: "config-validate-offline", args: []string{"config", "validate", "--offline"}},
		{name: "config-apply-dry-run", args: []string{"config", "apply", "--dry-run", "--message", "cli golden"}},
		{name: "config-reload", args: []string{"config", "reload"}},
		{name: "config-error", args: []string{"config", "validate", "--offline", "--config-dir", filepath.Join(fx.dataDir, "missing-config")}},
		{name: "eval", args: []string{"eval", filepath.Join(fx.dataDir, "config", "eval.star")}},
		{name: "eval-error", args: []string{"eval", filepath.Join(fx.dataDir, "config", "missing.star")}},
		{name: "test", args: []string{"test", filepath.Join(fx.dataDir, "config", "sample_test.star")}},
		{name: "test-error", args: []string{"test", filepath.Join(fx.dataDir, "config", "missing_test.star")}},
		{name: "automation-list", args: []string{"automation", "list"}},
		{name: "automation-get", args: []string{"automation", "get", "manual_scene"}},
		{name: "automation-disable", args: []string{"automation", "disable", "manual_scene"}},
		{name: "automation-enable", args: []string{"automation", "enable", "manual_scene"}},
		{name: "automation-trigger", args: []string{"automation", "trigger", "manual_scene"}},
		{name: "automation-trace", args: []string{"automation", "trace", "manual_scene"}, timeout: 600 * time.Millisecond},
		{name: "automation-watch", args: []string{"automation", "watch"}, timeout: 600 * time.Millisecond},
		{name: "automation-error", args: []string{"automation", "get", "missing_auto"}},
		{name: "script-list", args: []string{"script", "list"}},
		{name: "script-run", args: []string{"script", "run", "hello", "--arg", "name=cli"}},
		{name: "script-error", args: []string{"script", "run", "missing_script"}},
		{name: "mcp-tools", args: []string{"mcp", "tools", "--json"}},
		{name: "mcp-serve-error", args: []string{"mcp", "serve", "--endpoint", "unix:///tmp/switchyard-missing.sock"}},
		{name: "auth-login", args: []string{"auth", "login"}},
		{name: "auth-logout", args: []string{"auth", "logout"}},
		{name: "auth-whoami", args: []string{"auth", "whoami"}},
		{name: "auth-users-list", args: []string{"auth", "users", "list"}},
		{name: "auth-bootstrap", args: []string{"auth", "bootstrap", "alice", "--ttl", "1h"}},
		{name: "auth-tokens-create", args: []string{"auth", "tokens", "create", "--label", "cli-golden"}},
		{name: "auth-tokens-revoke", args: []string{"auth", "tokens", "revoke", "tok_missing"}},
		{name: "auth-explain-error", args: []string{"auth", "explain", "--user", "alice", "--action", "EntityService.Get", "--target", "entity:test_light"}},
		{name: "auth-policies-list", args: []string{"auth", "policies", "list"}},
		{name: "auth-policies-inspect", args: []string{"auth", "policies", "inspect", "admin_all"}},
		{name: "auth-error", args: []string{"auth", "tokens", "revoke"}},
		{name: "ui-dev", args: []string{"ui", "dev"}},
		{name: "ui-error", args: []string{"ui", "missing"}},
		{name: "widget-list", args: []string{"widget", "list"}},
		{name: "widget-install-error", args: []string{"widget", "install", "example.invalid/widget:missing"}},
		{name: "widget-uninstall-error", args: []string{"widget", "uninstall", "missing-pack"}},
		{name: "widget-error", args: []string{"widget", "install"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := runCLIGoldenCase(t, fx, tc)
			assertCLIGolden(t, tc.name, got)
		})
	}
}

func startCLIGoldenDaemon(t *testing.T, ctx context.Context) cliGoldenFixture {
	t.Helper()
	dataDir := shortCLITempDir(t)
	configDir := filepath.Join(dataDir, "config")
	driversDir := filepath.Join(dataDir, "drivers")
	writeCLIFixtureFiles(t, configDir, driversDir)

	adminPort := freeCLITCPPort(t)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	d := daemon.New(daemon.Config{
		DataDir:    dataDir,
		ConfigDir:  configDir,
		DriversDir: driversDir,
		AdminPort:  adminPort,
		LogFormat:  "json",
	}, logger, observability.NewMetrics())
	daemonCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- d.Run(daemonCtx) }()

	waitCLIHealth(t, fmt.Sprintf("http://127.0.0.1:%d/health", adminPort), done, logs, 30*time.Second)
	sock := filepath.Join(dataDir, "switchyardd.sock")
	waitCLIFile(t, sock, 10*time.Second)
	waitCLIEntity(t, dataDir, sock, "test_light", 15*time.Second)
	return cliGoldenFixture{dataDir: dataDir, sock: sock, cancel: cancel, done: done, logs: logs}
}

func writeCLIFixtureFiles(t *testing.T, configDir, driversDir string) {
	t.Helper()
	driverDir := filepath.Join(driversDir, "testdriver")
	if err := os.MkdirAll(driverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(driverDir, "testdriver-driver")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/testdriver")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build testdriver: %v\n%s", err, out)
	}
	writeFile(t, filepath.Join(driverDir, "manifest.pkl"), `extends "switchyard:driver"

const name = "testdriver"
const version = "0.0.0"
description = "CLI golden fake driver"
produces {
  "light"
}
binary = "testdriver-driver"
`)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "main.pkl"), `amends "switchyard:config"

import "switchyard:carport" as cp
import "switchyard:entities" as ent
import "switchyard:automations" as auto
import "switchyard:scripts" as scr

local class TestDriverInstance extends cp.DriverInstance {
  TESTDRIVER_MODE: String = "repeat_register"
}

driverInstances {
  new TestDriverInstance {
    id = "testdriver-main"
    driverName = "testdriver"
    TESTDRIVER_MODE = "repeat_register"
  }
}

entities {
  new ent.Light {
    id = "light.fixture"
    friendlyName = "Fixture Light"
    supportsBrightness = true
  }
}

scripts {
  new scr.Script {
    name = "hello"
    params {
      new scr.ScriptParam {
        name = "name"
        type = "string"
        required = false
        default = "world"
      }
    }
    handler = """
print("hello " + params["name"])
"""
  }
}

automations {
  new auto.Automation {
    id = "manual_scene"
    triggers {
      new auto.EventTrigger {
        kind = "manual_probe"
      }
    }
    actions {
      new auto.StarlarkAction {
        body = """
print("automation fired")
"""
      }
    }
  }
}
`)
	writeFile(t, filepath.Join(configDir, "eval.star"), `print("eval fixture")
`)
	writeFile(t, filepath.Join(configDir, "sample_test.star"), `def test_passes():
    assert(True)
`)
}

func runCLIGoldenCase(t *testing.T, fx cliGoldenFixture, tc cliGoldenCase) string {
	t.Helper()
	timeout := tc.timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	args := []string{
		"--data-dir", fx.dataDir,
		"--endpoint", "unix://" + fx.sock,
		"--no-color",
		"--format", "json",
	}
	args = append(args, tc.args...)

	cmdArgs := []string{"-test.run=TestCLIGoldenHelper", "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(),
		cliGoldenHelperEnv+"=1",
		"SWITCHYARD_CLI_GOLDEN_TIMEOUT="+timeout.String(),
		"NO_COLOR=1",
		"CLICOLOR=0",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
			stderr.WriteString(err.Error())
			stderr.WriteByte('\n')
		}
	}
	stdoutText := stdout.String()
	if stdoutText != "" && !strings.HasSuffix(stdoutText, "\n") {
		stdoutText += "\n"
	}
	out := fmt.Sprintf("$ switchyard %s\nexit: %d\nstdout:\n%sstderr:\n%s",
		strings.Join(args, " "),
		exitCode,
		stdoutText,
		stderr.String(),
	)
	return normalizeCLIGolden(out, fx.dataDir, fx.sock)
}

func assertCLIGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "cli", name+".golden.txt")
	if *updateCLIGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", path, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch for %s\n--- expected:\n%s\n--- got:\n%s", name, want, got)
	}
}

var (
	uuidRE             = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	ulidRE             = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	rfc3339RE          = regexp.MustCompile(`20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.+-]+Z?`)
	sqlTimeRE          = regexp.MustCompile(`20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}`)
	deadlineRE         = regexp.MustCompile(`deadline_exceeded: context deadline exceeded`)
	tokenIDRE          = regexp.MustCompile(`tok_[A-Za-z0-9_=-]+`)
	switchyardTokenRE  = regexp.MustCompile(`switchyard_[0-9A-HJKMNP-TV-Z]{26}_[A-Z2-7]{32,}`)
	enrollmentTokenRE  = regexp.MustCompile(`[0-9A-Z]{39,}`)
	longTokenLineRE    = regexp.MustCompile(`(?m)^[A-Za-z0-9_-]{24,}={0,2}$`)
	appliedAtUnixMsRE  = regexp.MustCompile(`"appliedAtUnixMs":[[:space:]]*"[^"]+"`)
	eventCursorRE      = regexp.MustCompile(`"cursor":[0-9]+`)
	eventPositionRE    = regexp.MustCompile(`"position":[0-9]+`)
	snapshotCursorRE   = regexp.MustCompile(`snapshot: cursor [0-9]+`)
	snapshotTablePosRE = regexp.MustCompile(`│[0-9]+[[:space:]]*│state_cache`)
	defaultDataDirRE   = regexp.MustCompile(`default "[^"]*\.local/share/switchyard"`)
	driverSpawnedPIDRE = regexp.MustCompile(`"detail":"[0-9]+","driverInstanceId":"testdriver-main","kind":"spawned"`)
	goVersionRE        = regexp.MustCompile(`"go_version":"go[0-9.]+`)
)

func normalizeCLIGolden(s, dataDir, sock string) string {
	s = strings.ReplaceAll(s, dataDir, "<DATA_DIR>")
	s = strings.ReplaceAll(s, sock, "<SOCKET>")
	s = defaultDataDirRE.ReplaceAllString(s, `default "<DEFAULT_DATA_DIR>"`)
	s = switchyardTokenRE.ReplaceAllString(s, "<SWITCHYARD_TOKEN>")
	s = enrollmentTokenRE.ReplaceAllString(s, "<ENROLLMENT_TOKEN>")
	s = uuidRE.ReplaceAllString(s, "<UUID>")
	s = ulidRE.ReplaceAllString(s, "<ULID>")
	s = rfc3339RE.ReplaceAllString(s, "<TIMESTAMP>")
	s = sqlTimeRE.ReplaceAllString(s, "<TIMESTAMP>")
	s = tokenIDRE.ReplaceAllString(s, "<TOKEN_ID>")
	s = longTokenLineRE.ReplaceAllString(s, "<SECRET>")
	s = appliedAtUnixMsRE.ReplaceAllString(s, `"appliedAtUnixMs":"<UNIX_MS>"`)
	s = eventCursorRE.ReplaceAllString(s, `"cursor":<CURSOR>`)
	s = eventPositionRE.ReplaceAllString(s, `"position":<CURSOR>`)
	s = snapshotCursorRE.ReplaceAllString(s, "snapshot: cursor <CURSOR>")
	s = snapshotTablePosRE.ReplaceAllString(s, "│<CURSOR>│state_cache")
	s = driverSpawnedPIDRE.ReplaceAllString(s, `"detail":"<PID>","driverInstanceId":"testdriver-main","kind":"spawned"`)
	s = goVersionRE.ReplaceAllString(s, `"go_version":"<GO_VERSION>`)
	s = deadlineRE.ReplaceAllString(s, "deadline_exceeded: <DEADLINE>")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func waitCLIEntity(t *testing.T, dataDir, sock, entity string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tc := cliGoldenCase{name: "wait-state", args: []string{"state", "get", entity}, timeout: 2 * time.Second}
		out := runCLIGoldenCase(t, cliGoldenFixture{dataDir: dataDir, sock: sock}, tc)
		if strings.Contains(out, "exit: 0") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("entity %s did not appear within %s", entity, timeout)
}

func waitCLIHealth(t *testing.T, url string, done <-chan error, logs *bytes.Buffer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("daemon exited before ready: %v\n%s", err, logs.String())
		default:
		}
		resp, err := http.Get(url) //nolint:noctx
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("daemon health did not become ready at %s\n%s", url, logs.String())
}

func waitCLIFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not appear within %s", path, timeout)
}

func freeCLITCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func shortCLITempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "sycli")
	if err != nil {
		dir = t.TempDir()
		return dir
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		next := filepath.Dir(wd)
		if next == wd {
			t.Fatal("repo root not found")
		}
		wd = next
	}
}
