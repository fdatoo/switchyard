package widgetpack

import (
	"context"
	"errors"
	"fmt"
)

// ErrInstallFailed is returned when pack installation fails.
var ErrInstallFailed = errors.New("widgetpack: install failed")

// InstallRequest carries the parameters for a pack installation.
type InstallRequest struct {
	Ref     string // OCI reference, e.g. "registry.example.com/bar-widgets:1.0.0"
	Name    string
	Version string
}

// Installer handles OCI pull + verify + store.
// OCI pull and cosign verification are deferred to a future implementation;
// this stub registers the pack metadata immediately.
type Installer struct {
	store *Store
}

// NewInstaller creates a new pack installer.
func NewInstaller(store *Store) *Installer {
	return &Installer{store: store}
}

// Install registers a widget pack. Currently a stub — real OCI/cosign implementation deferred.
func (i *Installer) Install(ctx context.Context, req InstallRequest) (*InstalledPack, error) {
	if req.Name == "" || req.Version == "" {
		return nil, fmt.Errorf("%w: name and version required", ErrInstallFailed)
	}
	pack := InstalledPack{
		Name:            req.Name,
		Version:         req.Version,
		SHA256:          "pending",
		SignatureStatus: "unsigned",
	}
	if err := i.store.Add(ctx, pack); err != nil {
		return nil, fmt.Errorf("install: store: %w", err)
	}
	return &pack, nil
}
