// Package management implements DriverManagementService — the settings
// sub-shell's view into running and available driver instances.
//
// See proto/switchyard/driver/v1/management.proto.
package management

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	driverv1 "github.com/fdatoo/switchyard/gen/switchyard/driver/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/driver/v1/driverv1connect"
)

// Registry is the interface this service uses to query and control driver
// instances. Production code wires the carport.Host + registry.Registry
// implementations; tests supply a fakeRegistry.
type Registry interface {
	// ListRunning returns all currently running driver summaries.
	ListRunning(ctx context.Context) ([]*driverv1.DriverSummary, error)
	// ListAvailable returns registry drivers that are not running.
	ListAvailable(ctx context.Context) ([]*driverv1.RegistryDriver, error)
	// Get returns a single running driver summary by ID.
	// Returns (nil, ErrNotFound) when the ID is unknown.
	Get(ctx context.Context, id string) (*driverv1.DriverSummary, error)
	// Restart triggers a graceful restart of the named instance.
	Restart(ctx context.Context, id, reason string) error
	// Stop triggers a graceful stop of the named instance.
	Stop(ctx context.Context, id, reason string) error
	// Logs returns the last n log lines for the named instance.
	Logs(ctx context.Context, id string, lastN uint32) ([]string, error)
}

// ErrNotFound is returned by Registry.Get when a driver ID is unknown.
var ErrNotFound = fmt.Errorf("driver not found")

// Service implements driverv1connect.DriverManagementServiceHandler.
type Service struct {
	driverv1connect.UnimplementedDriverManagementServiceHandler
	reg Registry
}

// NewService constructs a Service backed by the given Registry.
func NewService(reg Registry) *Service {
	return &Service{reg: reg}
}

// List returns all running driver summaries and all available registry drivers.
func (s *Service) List(
	ctx context.Context,
	_ *connect.Request[driverv1.ListDriversRequest],
) (*connect.Response[driverv1.ListDriversResponse], error) {
	running, err := s.reg.ListRunning(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list running: %w", err))
	}

	available, err := s.reg.ListAvailable(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list available: %w", err))
	}

	return connect.NewResponse(&driverv1.ListDriversResponse{
		Running:   running,
		Available: available,
	}), nil
}

// Get returns a single running driver summary by ID.
func (s *Service) Get(
	ctx context.Context,
	req *connect.Request[driverv1.GetDriverRequest],
) (*connect.Response[driverv1.GetDriverResponse], error) {
	driver, err := s.reg.Get(ctx, req.Msg.GetId())
	if err != nil {
		if err == ErrNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("driver %q not found", req.Msg.GetId()))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get driver: %w", err))
	}

	return connect.NewResponse(&driverv1.GetDriverResponse{Driver: driver}), nil
}

// Restart requests a graceful restart of the named driver instance.
func (s *Service) Restart(
	ctx context.Context,
	req *connect.Request[driverv1.RestartDriverRequest],
) (*connect.Response[driverv1.RestartDriverResponse], error) {
	if err := s.reg.Restart(ctx, req.Msg.GetId(), req.Msg.GetReason()); err != nil {
		if err == ErrNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("driver %q not found", req.Msg.GetId()))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("restart driver: %w", err))
	}

	return connect.NewResponse(&driverv1.RestartDriverResponse{Restarted: true}), nil
}

// Stop requests a graceful stop of the named driver instance.
func (s *Service) Stop(
	ctx context.Context,
	req *connect.Request[driverv1.StopDriverRequest],
) (*connect.Response[driverv1.StopDriverResponse], error) {
	if err := s.reg.Stop(ctx, req.Msg.GetId(), req.Msg.GetReason()); err != nil {
		if err == ErrNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("driver %q not found", req.Msg.GetId()))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("stop driver: %w", err))
	}

	return connect.NewResponse(&driverv1.StopDriverResponse{Stopped: true}), nil
}

// Logs returns the last N log lines for the named driver instance.
func (s *Service) Logs(
	ctx context.Context,
	req *connect.Request[driverv1.DriverLogsRequest],
) (*connect.Response[driverv1.DriverLogsResponse], error) {
	lines, err := s.reg.Logs(ctx, req.Msg.GetId(), req.Msg.GetLastN())
	if err != nil {
		if err == ErrNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("driver %q not found", req.Msg.GetId()))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get logs: %w", err))
	}

	return connect.NewResponse(&driverv1.DriverLogsResponse{Lines: lines}), nil
}
