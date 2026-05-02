package resources

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
)

// isTraceURI returns true when the URI looks like an automation trace resource.
func isTraceURI(uri string) bool {
	return strings.Contains(uri, "/runs/") && strings.HasSuffix(uri, "/trace")
}

// parseTraceURI extracts automationID and runID from a trace URI.
// "switchyard://automations/lights-auto/runs/01HZ.../trace" →
//
//	automationID = "lights-auto", runID = "01HZ..."
func parseTraceURI(uri string) (automationID, runID string) {
	// Find the segment between /automations/ and /runs/
	const autoPrefix = "switchyard://automations/"
	trimmed := strings.TrimPrefix(uri, autoPrefix)
	idx := strings.Index(trimmed, "/runs/")
	if idx < 0 {
		return "", ""
	}
	automationID = trimmed[:idx]
	rest := trimmed[idx+len("/runs/"):]
	// rest is "<runID>/trace" — strip the trailing "/trace"
	runID = strings.TrimSuffix(rest, "/trace")
	return automationID, runID
}

// traceReadHandler opens a short-lived Trace stream and returns all events
// collected within a 5-second window.
func traceReadHandler(d Deps) sdk.ResourceHandler {
	return func(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		automationID, runID := parseTraceURI(req.Params.URI)

		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		stream, err := d.Client.Automation.Trace(readCtx, connect.NewRequest(&v1.TraceAutomationRequest{
			Id:    automationID,
			RunId: runID,
		}))
		if err != nil {
			return nil, err
		}
		defer func() { _ = stream.Close() }()

		mo := protojson.MarshalOptions{UseProtoNames: true}
		var events []string
		for stream.Receive() {
			msg := stream.Msg()
			if k, ok := msg.Kind.(*v1.TraceAutomationResponse_Event); ok {
				b, err := mo.Marshal(k.Event)
				if err == nil {
					events = append(events, string(b))
				}
			}
		}

		// Build a JSON array from the already-marshalled JSON objects.
		result := buildJSONArray(events)
		return &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{{
				URI:      req.Params.URI,
				Text:     result,
				MIMEType: "application/json",
			}},
		}, nil
	}
}

// buildJSONArray constructs a JSON array from a slice of JSON-object strings.
func buildJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(s)
	}
	sb.WriteByte(']')
	return sb.String()
}

// watchTrace subscribes to an automation trace stream and sends one
// ResourceUpdated notification per event. On buffer overflow the subscription
// is closed and a final notification is sent.
func watchTrace(ctx context.Context, mgr *Manager, uri, automationID, runID string, d Deps) {
	if d.Metrics != nil && d.Metrics.MCPResourceSubscriptionsActive != nil {
		d.Metrics.MCPResourceSubscriptionsActive.WithLabelValues("trace").Inc()
		defer d.Metrics.MCPResourceSubscriptionsActive.WithLabelValues("trace").Dec()
	}

	stream, err := d.Client.Automation.Trace(ctx, connect.NewRequest(&v1.TraceAutomationRequest{
		Id:    automationID,
		RunId: runID,
	}))
	if err != nil {
		return
	}
	defer func() { _ = stream.Close() }()

	bufSize := int(d.MCPCaps.TraceSubscriptionBuffer)
	if bufSize <= 0 {
		bufSize = 1024
	}
	buf := make(chan *v1.TraceEvent, bufSize)

	for stream.Receive() {
		msg := stream.Msg()
		switch k := msg.Kind.(type) {
		case *v1.TraceAutomationResponse_Event:
			select {
			case buf <- k.Event:
			default:
				// Buffer overflow — close the subscription and send a final notification.
				if d.Metrics != nil && d.Metrics.MCPResourceOverflowCloses != nil {
					d.Metrics.MCPResourceOverflowCloses.WithLabelValues("trace", "trace_overflow").Inc()
				}
				mgr.stop(uri)
				s := mgr.getServer()
				if s != nil {
					_ = s.ResourceUpdated(ctx, &sdk.ResourceUpdatedNotificationParams{URI: uri})
				}
				return
			}
			s := mgr.getServer()
			if s != nil {
				_ = s.ResourceUpdated(ctx, &sdk.ResourceUpdatedNotificationParams{URI: uri})
			}
			if d.Metrics != nil && d.Metrics.MCPResourceUpdatesSent != nil {
				d.Metrics.MCPResourceUpdatesSent.WithLabelValues("trace").Inc()
			}
		case *v1.TraceAutomationResponse_Heartbeat:
			// skip
		}
	}
}

// RegisterTraces adds automation trace resource template handlers to server.
func RegisterTraces(server *sdk.Server, d Deps) {
	server.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "switchyard://automations/{automation_id}/runs/{run_id}/trace",
		Name:        "switchyard-automation-trace",
		Description: "Read trace events for an automation run",
	}, traceReadHandler(d))
}
