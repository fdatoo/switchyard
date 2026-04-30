package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
)

type fakeAutomations struct {
	automations []api.Automation
	runID       string
	triggerErr  error
	enableErr   error
}

func (f *fakeAutomations) List(_ context.Context, _ api.PageReq) ([]api.Automation, api.Cursor, error) {
	return f.automations, api.Cursor{}, nil
}

func (f *fakeAutomations) Get(_ context.Context, id string) (api.Automation, error) {
	for _, a := range f.automations {
		if a.ID == id {
			return a, nil
		}
	}
	return api.Automation{}, api.ErrAutomationNotFound
}

func (f *fakeAutomations) SetEnabled(_ context.Context, id string, enabled bool, _ string) (api.Automation, error) {
	if f.enableErr != nil {
		return api.Automation{}, f.enableErr
	}
	for i, a := range f.automations {
		if a.ID == id {
			f.automations[i].Enabled = enabled
			return f.automations[i], nil
		}
	}
	return api.Automation{}, api.ErrAutomationNotFound
}

func (f *fakeAutomations) Trigger(_ context.Context, id, _ string) (string, error) {
	if f.triggerErr != nil {
		return "", f.triggerErr
	}
	for _, a := range f.automations {
		if a.ID == id {
			if !a.Enabled {
				return "", api.ErrAutomationDisabled
			}
			return f.runID, nil
		}
	}
	return "", api.ErrAutomationNotFound
}

func (f *fakeAutomations) Trace(_ context.Context, _, _ string, _ uint64) (<-chan api.TraceEvent, func(), error) {
	ch := make(chan api.TraceEvent)
	return ch, func() { close(ch) }, nil
}

var _ api.AutomationControl = (*fakeAutomations)(nil)

func TestAutomationService_Trigger(t *testing.T) {
	fa := &fakeAutomations{
		automations: []api.Automation{{ID: "morning-routine", Enabled: true}},
		runID:       "run-1",
	}
	s := api.NewAutomationService(fa)
	resp, err := s.Trigger(context.Background(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: "morning-routine"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.RunId != "run-1" {
		t.Errorf("unexpected run_id: %s", resp.Msg.RunId)
	}
}

func TestAutomationService_Trigger_Disabled(t *testing.T) {
	fa := &fakeAutomations{
		automations: []api.Automation{{ID: "night-mode", Enabled: false}},
	}
	s := api.NewAutomationService(fa)
	_, err := s.Trigger(context.Background(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: "night-mode"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("expected CodeFailedPrecondition, got: %v", err)
	}
}

func TestAutomationService_Enable(t *testing.T) {
	fa := &fakeAutomations{
		automations: []api.Automation{{ID: "lights-off", Enabled: false}},
	}
	s := api.NewAutomationService(fa)
	resp, err := s.Enable(context.Background(), connect.NewRequest(&v1.EnableAutomationRequest{Id: "lights-off"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Automation.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestAutomationService_Trigger_NotFound(t *testing.T) {
	fa := &fakeAutomations{}
	s := api.NewAutomationService(fa)
	_, err := s.Trigger(context.Background(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: "no-such"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got: %v", err)
	}
}

func TestAutomationService_List(t *testing.T) {
	fa := &fakeAutomations{
		automations: []api.Automation{
			{ID: "a1", DisplayName: "Automation 1", Mode: "single", Enabled: true},
		},
	}
	s := api.NewAutomationService(fa)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListAutomationsRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Msg.Automations) != 1 || resp.Msg.Automations[0].Id != "a1" {
		t.Errorf("unexpected: %+v", resp.Msg.Automations)
	}
}
