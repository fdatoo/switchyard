package resources

import (
	"context"
	"encoding/json"
	"strings"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
)

// isEntityURI returns true when uri is a gohome entity resource URI.
func isEntityURI(uri string) bool {
	return strings.HasPrefix(uri, "gohome://entities/")
}

// parseEntityID extracts the entity ID from an entity URI.
// "gohome://entities/light.foo" → "light.foo"
// "gohome://entities/"         → "" (list all)
func parseEntityID(uri string) string {
	return strings.TrimPrefix(uri, "gohome://entities/")
}

// entityReadHandler returns an sdk.ResourceHandler that serves both single-
// entity and list-all requests depending on the requested URI.
func entityReadHandler(d Deps) sdk.ResourceHandler {
	return func(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		id := parseEntityID(req.Params.URI)
		if id == "" {
			return readAllEntities(ctx, req.Params.URI, d)
		}
		return readSingleEntity(ctx, req.Params.URI, id, d)
	}
}

func readSingleEntity(ctx context.Context, uri, id string, d Deps) (*sdk.ReadResourceResult, error) {
	resp, err := d.Client.Entity.Get(ctx, connect.NewRequest(&v1.GetEntityRequest{Id: id}))
	if err != nil {
		return nil, err
	}
	b, err := marshalEntity(resp.Msg.Entity)
	if err != nil {
		return nil, err
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{URI: uri, Text: string(b), MIMEType: "application/json"}},
	}, nil
}

func readAllEntities(ctx context.Context, uri string, d Deps) (*sdk.ReadResourceResult, error) {
	resp, err := d.Client.Entity.List(ctx, connect.NewRequest(&v1.ListEntitiesRequest{}))
	if err != nil {
		return nil, err
	}
	entities := make([]json.RawMessage, 0, len(resp.Msg.Entities))
	for _, e := range resp.Msg.Entities {
		b, merr := marshalEntity(e)
		if merr != nil {
			return nil, merr
		}
		entities = append(entities, b)
	}
	b, _ := json.Marshal(entities)
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{URI: uri, Text: string(b), MIMEType: "application/json"}},
	}, nil
}

// marshalEntity serialises a proto Entity to JSON, renaming friendly_name → name.
func marshalEntity(e *v1.Entity) (json.RawMessage, error) {
	mo := protojson.MarshalOptions{UseProtoNames: true}
	raw, err := mo.Marshal(e)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if fn, ok := m["friendly_name"]; ok {
		m["name"] = fn
		delete(m, "friendly_name")
	}
	return json.Marshal(m)
}

// watchEntity runs as a goroutine, subscribing to entity changes and sending
// ResourceUpdated notifications. It uses a size-1 coalescing channel so that a
// slow MCP client only receives one notification per "batch" of changes.
func watchEntity(ctx context.Context, mgr *Manager, uri, entityID string, d Deps) {
	if d.Metrics != nil && d.Metrics.MCPResourceSubscriptionsActive != nil {
		d.Metrics.MCPResourceSubscriptionsActive.WithLabelValues("entity").Inc()
		defer d.Metrics.MCPResourceSubscriptionsActive.WithLabelValues("entity").Dec()
	}

	selector := &v1.EntitySelector{}
	if entityID != "" {
		selector.EntityIds = []string{entityID}
	}
	stream, err := d.Client.Entity.Subscribe(ctx, connect.NewRequest(&v1.SubscribeEntitiesRequest{
		Selector: selector,
	}))
	if err != nil {
		return
	}
	defer func() { _ = stream.Close() }()

	// Coalescing notify channel: buffer of 1 means multiple rapid changes are
	// collapsed into a single ResourceUpdated notification.
	notify := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-notify:
				if !ok {
					return
				}
				s := mgr.getServer()
				if s != nil {
					_ = s.ResourceUpdated(ctx, &sdk.ResourceUpdatedNotificationParams{URI: uri})
				}
				if d.Metrics != nil && d.Metrics.MCPResourceUpdatesSent != nil {
					d.Metrics.MCPResourceUpdatesSent.WithLabelValues("entity").Inc()
				}
			}
		}
	}()

	for stream.Receive() {
		msg := stream.Msg()
		switch msg.Kind.(type) {
		case *v1.SubscribeEntitiesResponse_Change:
			select {
			case notify <- struct{}{}:
				// queued
			default:
				// coalesced — channel full, a notification is already pending
				if d.Metrics != nil && d.Metrics.MCPResourceOverflowCloses != nil {
					d.Metrics.MCPResourceOverflowCloses.WithLabelValues("entity", "coalesced").Inc()
				}
			}
		case *v1.SubscribeEntitiesResponse_Heartbeat:
			// skip
		}
	}
}

// RegisterEntities adds entity resource and resource-template handlers to server.
func RegisterEntities(server *sdk.Server, d Deps) {
	handler := entityReadHandler(d)

	server.AddResource(&sdk.Resource{
		URI:         "gohome://entities/",
		Name:        "gohome-entities",
		Description: "All gohome entities (list)",
		MIMEType:    "application/json",
	}, handler)

	server.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "gohome://entities/{id}",
		Name:        "gohome-entity",
		Description: "Read a single gohome entity by ID",
	}, handler)
}
