// Package script provides the runtime for invocable named Starlark functions.
// Scripts are compiled once from a ConfigSnapshot into an immutable registry;
// Engine.Call validates arguments, emits ScriptInvoked/ScriptFinished events,
// and executes the handler via the shared starlark.Runtime.
package script

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by Engine.
var (
	ErrScriptNotFound = errors.New("script: unknown name")
	ErrScriptArgs     = errors.New("script: invalid arguments")
)

// ItemError is one compilation error tied to a specific script and path.
type ItemError struct {
	Name   string // script name
	Path   string // "scripts[greet].params[0].default"
	Reason string
}

func (e *ItemError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Name, e.Path, e.Reason)
}

// CompileError aggregates every ItemError found during Compile so authors
// see every problem from a single `gohome config validate`.
type CompileError struct {
	Items []*ItemError
}

func (e *CompileError) Error() string {
	if len(e.Items) == 1 {
		return e.Items[0].Error()
	}
	return fmt.Sprintf("%d script compile errors (first: %s)", len(e.Items), e.Items[0].Error())
}
