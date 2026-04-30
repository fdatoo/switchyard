package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

type AutomationService struct{ be AutomationControl }

func NewAutomationService(be AutomationControl) *AutomationService {
	return &AutomationService{be: be}
}

var _ gohomev1alpha1connect.AutomationServiceHandler = (*AutomationService)(nil)

func (s *AutomationService) List(ctx context.Context, req *connect.Request[v1.ListAutomationsRequest]) (*connect.Response[v1.ListAutomationsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	automations, next, err := s.be.List(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListAutomationsResponse{Page: &v1.PageResponse{}}
	if tok, _ := EncodeCursor(next); tok != "" {
		out.Page.NextPageToken = tok
	}
	for _, a := range automations {
		out.Automations = append(out.Automations, automationToProto(a))
	}
	return connect.NewResponse(out), nil
}

func (s *AutomationService) Get(ctx context.Context, req *connect.Request[v1.GetAutomationRequest]) (*connect.Response[v1.GetAutomationResponse], error) {
	a, err := s.be.Get(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "automation_not_found")
	}
	return connect.NewResponse(&v1.GetAutomationResponse{Automation: automationToProto(a)}), nil
}

func (s *AutomationService) Enable(ctx context.Context, req *connect.Request[v1.EnableAutomationRequest]) (*connect.Response[v1.EnableAutomationResponse], error) {
	a, err := s.be.SetEnabled(ctx, req.Msg.Id, true, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "enable_failed")
	}
	return connect.NewResponse(&v1.EnableAutomationResponse{Automation: automationToProto(a)}), nil
}

func (s *AutomationService) Disable(ctx context.Context, req *connect.Request[v1.DisableAutomationRequest]) (*connect.Response[v1.DisableAutomationResponse], error) {
	a, err := s.be.SetEnabled(ctx, req.Msg.Id, false, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "disable_failed")
	}
	return connect.NewResponse(&v1.DisableAutomationResponse{Automation: automationToProto(a)}), nil
}

func (s *AutomationService) Trigger(ctx context.Context, req *connect.Request[v1.TriggerAutomationRequest]) (*connect.Response[v1.TriggerAutomationResponse], error) {
	runID, err := s.be.Trigger(ctx, req.Msg.Id, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "trigger_failed")
	}
	return connect.NewResponse(&v1.TriggerAutomationResponse{RunId: runID}), nil
}

func (s *AutomationService) Trace(ctx context.Context, req *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
	cfg := currentStreamConfig()
	src, cancel, err := s.be.Trace(ctx, req.Msg.Id, req.Msg.RunId, req.Msg.FromCursor)
	if err != nil {
		return ToConnect(ctx, err, "trace_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	var latest uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case te, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = te.Cursor
			if err := stream.Send(&v1.TraceAutomationResponse{
				Kind: &v1.TraceAutomationResponse_Event{Event: &v1.TraceEvent{
					Cursor:       te.Cursor,
					At:           ProtoTime(te.At),
					AutomationId: te.AutomationID,
					RunId:        te.RunID,
					Kind:         te.Kind,
					Detail:       te.Detail,
					Metadata:     te.Metadata,
				}},
			}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.TraceAutomationResponse{
				Kind: &v1.TraceAutomationResponse_Heartbeat{Heartbeat: &v1.Heartbeat{
					LatestCursor: latest, ServerTime: ProtoTime(t),
				}},
			}); err != nil {
				return err
			}
		}
	}
}

func automationToProto(a Automation) *v1.Automation {
	return &v1.Automation{
		Id:          a.ID,
		DisplayName: a.DisplayName,
		Mode:        a.Mode,
		Enabled:     a.Enabled,
		InFlight:    a.InFlight,
	}
}
