package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Lockfile holds an exclusive PID file for the daemon.
type Lockfile struct {
	path string
}

// AcquireLockfile writes <dataDir>/switchyardd.lock with the current PID.
// Returns an error if a live process already owns the file.
func AcquireLockfile(dataDir string) (*Lockfile, error) {
	path := filepath.Join(dataDir, "switchyardd.lock")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dataDir, err)
	}

	if existingPID, ok := readPID(path); ok {
		if processAlive(existingPID) {
			return nil, fmt.Errorf("switchyardd already running (pid %d)", existingPID)
		}
		// Stale — fall through and overwrite.
	}

	body := fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return nil, fmt.Errorf("write lockfile: %w", err)
	}
	return &Lockfile{path: path}, nil
}

// Release removes the PID lockfile if this process still owns its path.
func (l *Lockfile) Release() error {
	if l == nil {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func readPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	lines := strings.SplitN(string(data), "\n", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, signal 0 tests process existence.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
