package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const DefaultApplyMessage = "config(repo): apply configuration"

var (
	ErrInvalidSemanticMessage = errors.New("invalid semantic config message")
	semanticMessagePattern    = regexp.MustCompile(`^config\(([a-z][a-z0-9-]*)\): ([a-z0-9][a-z0-9 ,` + "`" + `._:/()+-]*[^.])$`)
)

var allowedConfigMessageScopes = map[string]struct{}{
	"area":       {},
	"automation": {},
	"driver":     {},
	"entity":     {},
	"page":       {},
	"policy":     {},
	"scene":      {},
	"script":     {},
	"auth":       {},
	"mcp":        {},
	"repo":       {},
}

type SemanticMessageError struct {
	Message string
	Reason  string
}

func (e *SemanticMessageError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%v: %s", ErrInvalidSemanticMessage, e.Reason)
	}
	return fmt.Sprintf("%v %q: %s", ErrInvalidSemanticMessage, e.Message, e.Reason)
}

func (e *SemanticMessageError) Unwrap() error {
	return ErrInvalidSemanticMessage
}

func NormalizeApplyMessage(message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = DefaultApplyMessage
	}
	if strings.ContainsAny(message, "\r\n") {
		return "", &SemanticMessageError{Message: message, Reason: "must be one line"}
	}
	matches := semanticMessagePattern.FindStringSubmatch(message)
	if matches == nil {
		return "", &SemanticMessageError{
			Message: message,
			Reason:  `expected "config(<scope>): <lowercase imperative subject>"`,
		}
	}
	scope := matches[1]
	if _, ok := allowedConfigMessageScopes[scope]; !ok {
		return "", &SemanticMessageError{
			Message: message,
			Reason:  fmt.Sprintf("unknown scope %q", scope),
		}
	}
	if len(message) > 120 {
		return "", &SemanticMessageError{Message: message, Reason: "must be 120 characters or fewer"}
	}
	return message, nil
}
