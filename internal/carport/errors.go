// Package carport hosts the driver-supervisor subsystem: per-instance subprocess
// lifecycle, command dispatch, and event ingest from drivers over the Carport
// gRPC protocol (v1alpha1). Driver instances are registered dynamically via
// RegisterInstance.
//
// See docs/superpowers/specs/2026-04-21-c2-carport-protocol-design.md.
package carport

import "errors"

var (
	// ErrEntityUnknown: routing.Resolve found no entity with the given id.
	ErrEntityUnknown = errors.New("carport: entity unknown")

	// ErrInstanceNotRunning: the entity's owning driver instance is not in state=running.
	ErrInstanceNotRunning = errors.New("carport: driver instance not running")

	// ErrDispatchTimeout: deadline elapsed before CommandResult arrived.
	ErrDispatchTimeout = errors.New("carport: dispatch timeout")

	// ErrStreamClosed: Run stream died mid-dispatch (driver crash, network error).
	ErrStreamClosed = errors.New("carport: driver stream closed")

	// ErrContextCanceled: caller's context was canceled.
	ErrContextCanceled = errors.New("carport: context canceled")

	// ErrHostStopped: carport.Host is shutting down or already stopped.
	ErrHostStopped = errors.New("carport: host stopped")
)
