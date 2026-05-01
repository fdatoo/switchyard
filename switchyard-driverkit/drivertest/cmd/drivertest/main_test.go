//go:build integration

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCLI_HappyPath compiles the fakedevice binary and runs drivertest against it.
func TestCLI_HappyPath(t *testing.T) {
	tmp := t.TempDir()

	fakedeviceBin := filepath.Join(tmp, "fakedevice")
	if out, err := exec.Command("go", "build", "-o", fakedeviceBin,
		"github.com/fdatoo/switchyard-driverkit/examples/fakedevice").CombinedOutput(); err != nil {
		t.Fatalf("build fakedevice: %v\n%s", err, out)
	}

	driverTestBin := filepath.Join(tmp, "drivertest")
	if out, err := exec.Command("go", "build", "-o", driverTestBin,
		"github.com/fdatoo/switchyard-driverkit/drivertest/cmd/drivertest").CombinedOutput(); err != nil {
		t.Fatalf("build drivertest: %v\n%s", err, out)
	}

	cmd := exec.Command(driverTestBin, "run", fakedeviceBin,
		"--scenario", "happy-path",
		"--entity-id", "light.fake_light",
		"--json")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("drivertest happy-path: %v", err)
	}
}

func TestCLI_Reconnect(t *testing.T) {
	tmp := t.TempDir()

	fakedeviceBin := filepath.Join(tmp, "fakedevice")
	if out, err := exec.Command("go", "build", "-o", fakedeviceBin,
		"github.com/fdatoo/switchyard-driverkit/examples/fakedevice").CombinedOutput(); err != nil {
		t.Fatalf("build fakedevice: %v\n%s", err, out)
	}

	driverTestBin := filepath.Join(tmp, "drivertest")
	if out, err := exec.Command("go", "build", "-o", driverTestBin,
		"github.com/fdatoo/switchyard-driverkit/drivertest/cmd/drivertest").CombinedOutput(); err != nil {
		t.Fatalf("build drivertest: %v\n%s", err, out)
	}

	cmd := exec.Command(driverTestBin, "run", fakedeviceBin, "--scenario", "reconnect", "--json")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("drivertest reconnect: %v", err)
	}
}
