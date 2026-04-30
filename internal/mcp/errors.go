package mcp

import (
	"errors"

	"connectrpc.com/connect"

	errorpb "github.com/fdatoo/gohome/gen/gohome/error/v1alpha1"
)

// MCPErrorEnvelope is the JSON shape returned in MCP tool errors.
type MCPErrorEnvelope struct {
	Reason        string            `json:"reason"`
	Message       string            `json:"message"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	RequestID     string            `json:"request_id"`
	CorrelationID string            `json:"correlation_id"`
}

// ToMCPErrorEnvelope converts any error into an MCP tool error envelope.
func ToMCPErrorEnvelope(err error) MCPErrorEnvelope {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return MCPErrorEnvelope{Reason: "internal", Message: err.Error()}
	}
	env := MCPErrorEnvelope{
		Reason:   reasonFromCode(ce.Code()),
		Message:  ce.Message(),
		Metadata: map[string]string{},
	}
	for _, d := range ce.Details() {
		m, derr := d.Value()
		if derr != nil {
			continue
		}
		if detail, ok := m.(*errorpb.ErrorDetail); ok {
			if detail.Reason != "" {
				env.Reason = detail.Reason
			}
			for k, v := range detail.Metadata {
				env.Metadata[k] = v
			}
			env.RequestID = detail.RequestId
			env.CorrelationID = detail.CorrelationId
			break
		}
	}
	return env
}

func reasonFromCode(c connect.Code) string {
	switch c {
	case connect.CodeInvalidArgument:
		return "invalid_argument"
	case connect.CodeNotFound:
		return "not_found"
	case connect.CodeFailedPrecondition:
		return "failed_precondition"
	case connect.CodePermissionDenied:
		return "forbidden"
	case connect.CodeUnauthenticated:
		return "unauthenticated"
	case connect.CodeResourceExhausted:
		return "resource_exhausted"
	case connect.CodeDeadlineExceeded:
		return "deadline_exceeded"
	case connect.CodeUnimplemented:
		return "unimplemented"
	case connect.CodeUnavailable:
		return "unavailable"
	default:
		return "internal"
	}
}
