package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/policy"
)

// EntityService implements entity query, command, and subscription RPCs.
type EntityService struct {
	r      EntityReader
	caller CapabilityCaller

	// streamSource is wired by the daemon. Nil means Subscribe returns
	// UNIMPLEMENTED for tests or partial embedders that do not provide streams.
	streamSource EntityStreamSource

	policyRuntime *policy.Runtime // nil until wired; filter passes through if nil
}

// NewEntityService returns an entity service backed by registry reads and capability dispatch.
func NewEntityService(r EntityReader, caller CapabilityCaller) *EntityService {
	return &EntityService{r: r, caller: caller}
}

// SetStreamSource wires the live subscription source after construction.
func (s *EntityService) SetStreamSource(src EntityStreamSource) { s.streamSource = src }

// SetPolicyRuntime wires the policy runtime after construction.
func (s *EntityService) SetPolicyRuntime(rt *policy.Runtime) { s.policyRuntime = rt }

var _ switchyardv1alpha1connect.EntityServiceHandler = (*EntityService)(nil)

// List returns entities matching the request selector.
func (s *EntityService) List(ctx context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	sel := selectorFromProto(req.Msg.Selector)
	ents, next, err := s.r.ListEntities(ctx, sel, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListEntitiesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, e := range ents {
		out.Entities = append(out.Entities, entityToProto(e))
	}
	return connect.NewResponse(out), nil
}

// Get returns one entity by id.
func (s *EntityService) Get(ctx context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
	e, err := s.r.GetEntity(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "entity_not_found")
	}
	return connect.NewResponse(&v1.GetEntityResponse{Entity: entityToProto(e)}), nil
}

// CallCapability dispatches a command to the entity's owning driver.
func (s *EntityService) CallCapability(ctx context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
	if req.Msg.EntityId == "" || req.Msg.Capability == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "missing_required_field")
	}
	var params map[string]any
	if req.Msg.Parameters != nil {
		params = req.Msg.Parameters.AsMap()
	}
	res, err := s.caller.Call(ctx, req.Msg.EntityId, req.Msg.Capability, params)
	if err != nil {
		return nil, ToConnect(ctx, err, "call_failed")
	}
	return connect.NewResponse(&v1.CallCapabilityResponse{
		CorrelationId: res.CorrelationID,
		Success:       res.Success,
		ErrorMessage:  res.ErrorMessage,
	}), nil
}

// Subscribe streams policy-filtered entity changes and idle heartbeats.
func (s *EntityService) Subscribe(ctx context.Context, req *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
	if s.streamSource == nil {
		return ToConnect(ctx, ErrNotImplemented, "subscribe_unimplemented")
	}
	p, _ := auth.PrincipalFromContext(ctx)
	filter := NewEntityFilter(s.policyRuntime, req.Msg.PolicyMode)

	cfg := currentStreamConfig()
	sel := selectorFromProto(req.Msg.Selector)

	// Preflight: evaluate policy against the initial entity set so STRICT mode
	// can reject the connection before the stream opens.
	if candidates, _, lerr := s.r.ListEntities(ctx, sel, PageReq{Size: 1000}); lerr == nil {
		targets := make([]policy.Target, 0, len(candidates))
		for _, e := range candidates {
			targets = append(targets, policy.Target{Kind: "entity", ID: e.ID, Area: policy.AreaSlug(e.AreaID), Class: e.Type})
		}
		if _, perr := filter.Preflight(ctx, p, targets); perr != nil {
			return perr
		}
	}

	var fromCursor uint64
	if req.Msg.FromCursor != 0 {
		fromCursor = req.Msg.FromCursor
	}
	src, cancel, err := s.streamSource.Subscribe(ctx, sel, fromCursor)
	if err != nil {
		return ToConnect(ctx, err, "subscribe_failed")
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
		case ec, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = ec.Cursor
			t := policy.Target{Kind: "entity", ID: ec.EntityID, Area: policy.AreaSlug(ec.Entity.AreaID), Class: ec.Entity.Type}
			if !filter.AllowsEntity(ctx, p, t) {
				continue
			}
			if err := stream.Send(&v1.SubscribeEntitiesResponse{
				Kind: &v1.SubscribeEntitiesResponse_Change{Change: &v1.EntityChange{
					EntityId: ec.EntityID,
					Cursor:   ec.Cursor,
					At:       ProtoTime(time.UnixMilli(ec.AtUnixMs)),
					Entity:   entityToProto(ec.Entity),
				}},
			}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case tick := <-ticker.C():
			if err := stream.Send(&v1.SubscribeEntitiesResponse{
				Kind: &v1.SubscribeEntitiesResponse_Heartbeat{Heartbeat: &v1.Heartbeat{
					LatestCursor: latest, ServerTime: ProtoTime(tick),
				}},
			}); err != nil {
				return err
			}
		}
	}
}

func entityToProto(e Entity) *v1.Entity {
	return &v1.Entity{
		Id:           e.ID,
		Type:         e.Type,
		DeviceId:     e.DeviceID,
		AreaId:       e.AreaID,
		ZoneId:       e.ZoneID,
		FriendlyName: e.FriendlyName,
		State:        e.State,
		Capabilities: e.Capabilities,
	}
}

func selectorFromProto(p *v1.EntitySelector) EntitySelector {
	if p == nil {
		return EntitySelector{}
	}
	return EntitySelector{
		EntityIDs: p.EntityIds,
		DeviceIDs: p.DeviceIds,
		Areas:     p.Areas,
		Zones:     p.Zones,
		Classes:   p.Classes,
	}
}
