package storage_test

import (
	"testing"

	"github.com/fdatoo/gohome/internal/storage"
)

func TestLockfile_SecondAcquireFailsWhileHeld(t *testing.T) {
	dir := t.TempDir()
	l1, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Release()

	if _, err := storage.AcquireLockfile(dir); err == nil {
		t.Fatal("expected second acquire to fail")
	}
}

func TestLockfile_ReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	l1, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l1.Release(); err != nil {
		t.Fatal(err)
	}
	l2, err := storage.AcquireLockfile(dir)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	_ = l2.Release()
}
