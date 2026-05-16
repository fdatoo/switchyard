// Package commandcatalog provides the server-side verb catalog registry and
// CommandCatalogService implementation for the Switchyard command palette (UI v2 plan 05).
package commandcatalog

import (
	"context"
	"fmt"
	"sync"

	"connectrpc.com/connect"

	catalogv1 "github.com/fdatoo/switchyard/gen/switchyard/commandcatalog/v1"
)

// ArgType mirrors catalogv1.ArgType for domain-code use without importing the proto package.
type ArgType int

const (
	// ArgTypeString accepts a single string value.
	ArgTypeString ArgType = ArgType(catalogv1.ArgType_ARG_TYPE_STRING)

	// ArgTypeInt accepts a base-10 integer value.
	ArgTypeInt ArgType = ArgType(catalogv1.ArgType_ARG_TYPE_INT)

	// ArgTypeBool accepts a boolean value.
	ArgTypeBool ArgType = ArgType(catalogv1.ArgType_ARG_TYPE_BOOL)

	// ArgTypeDuration accepts a Go-style duration string.
	ArgTypeDuration ArgType = ArgType(catalogv1.ArgType_ARG_TYPE_DURATION)

	// ArgTypeStringList accepts a repeated string value.
	ArgTypeStringList ArgType = ArgType(catalogv1.ArgType_ARG_TYPE_STRING_LIST)
)

// ArgSchema describes one argument of a verb.
type ArgSchema struct {
	Name     string
	Type     ArgType
	Required bool
	CLIFlag  string
	Hint     string
}

// Verb is a single command in the catalog.
type Verb struct {
	Name        string
	Description string
	Args        []ArgSchema
	CLIForm     string
	HandlerRef  string
}

// Registrar is the write side of the Registry, accepted by RegisterCommands shims.
type Registrar interface {
	Register(v Verb)
}

// Registry holds all registered verbs.
type Registry struct {
	mu    sync.RWMutex
	verbs []Verb
	names map[string]struct{}
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{names: make(map[string]struct{})}
}

// Register adds a verb to the registry. Panics on duplicate name — registrations
// happen at startup so fail-fast is the correct behaviour.
func (r *Registry) Register(v Verb) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.names[v.Name]; exists {
		panic(fmt.Sprintf("commandcatalog: duplicate verb %q", v.Name))
	}
	r.names[v.Name] = struct{}{}
	r.verbs = append(r.verbs, v)
}

// All returns a snapshot of every registered verb.
func (r *Registry) All() []Verb {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Verb, len(r.verbs))
	copy(out, r.verbs)
	return out
}

// CommandCatalogService implements the ConnectRPC CommandCatalogServiceHandler.
type CommandCatalogService struct {
	registry *Registry
}

// NewCommandCatalogService creates a service backed by the given registry.
func NewCommandCatalogService(r *Registry) *CommandCatalogService {
	return &CommandCatalogService{registry: r}
}

// List returns all registered verbs with full field population.
func (s *CommandCatalogService) List(
	_ context.Context,
	_ *connect.Request[catalogv1.ListRequest],
) (*connect.Response[catalogv1.ListResponse], error) {
	verbs := s.registry.All()
	pbVerbs := make([]*catalogv1.Verb, 0, len(verbs))
	for _, v := range verbs {
		pbArgs := make([]*catalogv1.ArgSchema, 0, len(v.Args))
		for _, a := range v.Args {
			pbArgs = append(pbArgs, &catalogv1.ArgSchema{
				Name:     a.Name,
				Type:     catalogv1.ArgType(a.Type),
				Required: a.Required,
				CliFlag:  a.CLIFlag,
				Hint:     a.Hint,
			})
		}
		pbVerbs = append(pbVerbs, &catalogv1.Verb{
			Name:        v.Name,
			Description: v.Description,
			Args:        pbArgs,
			CliForm:     v.CLIForm,
			HandlerRef:  v.HandlerRef,
		})
	}
	return connect.NewResponse(&catalogv1.ListResponse{Verbs: pbVerbs}), nil
}
