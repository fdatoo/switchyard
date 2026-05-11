package management_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	driverv1 "github.com/fdatoo/switchyard/gen/switchyard/driver/v1"
	"github.com/fdatoo/switchyard/internal/driver/management"
)

// fakeRegistry is an in-memory Registry implementation for tests.
type fakeRegistry struct {
	running   []*driverv1.DriverSummary
	available []*driverv1.RegistryDriver
	logs      map[string][]string

	restarted []string
	stopped   []string
}

func (f *fakeRegistry) ListRunning(_ context.Context) ([]*driverv1.DriverSummary, error) {
	return f.running, nil
}

func (f *fakeRegistry) ListAvailable(_ context.Context) ([]*driverv1.RegistryDriver, error) {
	return f.available, nil
}

func (f *fakeRegistry) Get(_ context.Context, id string) (*driverv1.DriverSummary, error) {
	for _, d := range f.running {
		if d.Id == id {
			return d, nil
		}
	}
	return nil, management.ErrNotFound
}

func (f *fakeRegistry) Restart(_ context.Context, id, _ string) error {
	for _, d := range f.running {
		if d.Id == id {
			f.restarted = append(f.restarted, id)
			return nil
		}
	}
	return management.ErrNotFound
}

func (f *fakeRegistry) Stop(_ context.Context, id, _ string) error {
	for _, d := range f.running {
		if d.Id == id {
			f.stopped = append(f.stopped, id)
			return nil
		}
	}
	return management.ErrNotFound
}

func (f *fakeRegistry) Logs(_ context.Context, id string, lastN uint32) ([]string, error) {
	lines, ok := f.logs[id]
	if !ok {
		return nil, management.ErrNotFound
	}
	if uint32(len(lines)) > lastN && lastN > 0 {
		lines = lines[uint32(len(lines))-lastN:]
	}
	return lines, nil
}

// newTestRegistry returns a fakeRegistry with two running drivers and one available.
func newTestRegistry() *fakeRegistry {
	return &fakeRegistry{
		running: []*driverv1.DriverSummary{
			{
				Id:           "hue-bridge",
				Pack:         "@switchyard/hue",
				Version:      "1.2.3",
				Status:       "healthy",
				UptimeSeconds: 86400,
				EntityCount:  42,
			},
			{
				Id:           "z2m",
				Pack:         "@switchyard/z2m",
				Version:      "2.0.1",
				Status:       "reconnecting",
				UptimeSeconds: 3600,
				EntityCount:  17,
			},
		},
		available: []*driverv1.RegistryDriver{
			{
				Id:      "homekit-bridge",
				Pack:    "@switchyard/homekit",
				Version: "0.9.0",
				Status:  "available",
			},
		},
		logs: map[string][]string{
			"z2m": {
				"[2026-05-11T10:00:00Z] info: starting zigbee2mqtt",
				"[2026-05-11T10:00:01Z] info: coordinator detected",
				"[2026-05-11T10:00:02Z] info: permit join disabled",
				"[2026-05-11T10:00:03Z] info: device 0x00158d00 joined",
				"[2026-05-11T10:00:04Z] info: device 0x00158d01 joined",
				"[2026-05-11T10:00:05Z] warn: reconnecting to coordinator",
				"[2026-05-11T10:00:06Z] info: reconnected",
				"[2026-05-11T10:00:07Z] info: ready",
				"[2026-05-11T10:00:08Z] info: processing state",
			},
		},
	}
}

func newService(reg management.Registry) *management.Service {
	return management.NewService(reg)
}

func TestList_ReturnsBothGroups(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	resp, err := svc.List(context.Background(), connect.NewRequest(&driverv1.ListDriversRequest{}))
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}

	if got := len(resp.Msg.Running); got != 2 {
		t.Errorf("running: want 2, got %d", got)
	}
	if got := len(resp.Msg.Available); got != 1 {
		t.Errorf("available: want 1, got %d", got)
	}
	if resp.Msg.Running[0].Id != "hue-bridge" {
		t.Errorf("first running driver: want hue-bridge, got %s", resp.Msg.Running[0].Id)
	}
	if resp.Msg.Available[0].Id != "homekit-bridge" {
		t.Errorf("first available driver: want homekit-bridge, got %s", resp.Msg.Available[0].Id)
	}
}

func TestGet_ReturnsCorrectDriver(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	resp, err := svc.Get(context.Background(), connect.NewRequest(&driverv1.GetDriverRequest{Id: "z2m"}))
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if resp.Msg.Driver.Pack != "@switchyard/z2m" {
		t.Errorf("pack: want @switchyard/z2m, got %s", resp.Msg.Driver.Pack)
	}
}

func TestGet_NotFound(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	_, err := svc.Get(context.Background(), connect.NewRequest(&driverv1.GetDriverRequest{Id: "unknown"}))
	if err == nil {
		t.Fatal("Get: expected error for unknown driver, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("error code: want CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestLogs_ReturnsLastNLines(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	resp, err := svc.Logs(context.Background(), connect.NewRequest(&driverv1.DriverLogsRequest{Id: "z2m", LastN: 3}))
	if err != nil {
		t.Fatalf("Logs: unexpected error: %v", err)
	}
	if got := len(resp.Msg.Lines); got != 3 {
		t.Errorf("lines: want 3, got %d", got)
	}
}

func TestLogs_NotFound(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	_, err := svc.Logs(context.Background(), connect.NewRequest(&driverv1.DriverLogsRequest{Id: "hue-bridge", LastN: 5}))
	if err == nil {
		t.Fatal("Logs: expected error for driver with no logs, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("error code: want CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestRestart_CallsThrough(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	resp, err := svc.Restart(context.Background(), connect.NewRequest(&driverv1.RestartDriverRequest{
		Id:     "hue-bridge",
		Reason: "manual restart test",
	}))
	if err != nil {
		t.Fatalf("Restart: unexpected error: %v", err)
	}
	if !resp.Msg.Restarted {
		t.Error("Restart: expected Restarted=true")
	}
	if len(reg.restarted) != 1 || reg.restarted[0] != "hue-bridge" {
		t.Errorf("Restart: registry not called with hue-bridge, got %v", reg.restarted)
	}
}

func TestStop_CallsThrough(t *testing.T) {
	reg := newTestRegistry()
	svc := newService(reg)

	resp, err := svc.Stop(context.Background(), connect.NewRequest(&driverv1.StopDriverRequest{
		Id:     "z2m",
		Reason: "manual stop test",
	}))
	if err != nil {
		t.Fatalf("Stop: unexpected error: %v", err)
	}
	if !resp.Msg.Stopped {
		t.Error("Stop: expected Stopped=true")
	}
	if len(reg.stopped) != 1 || reg.stopped[0] != "z2m" {
		t.Errorf("Stop: registry not called with z2m, got %v", reg.stopped)
	}
}
