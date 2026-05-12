package starlarkls

// predeclaredNames returns the union of Starlark universe builtins and
// Switchyard runtime globals. A reference to any of these names is considered
// resolved at the LSP layer.
func predeclaredNames() map[string]bool {
	out := map[string]bool{}
	for _, n := range starlarkUniverse {
		out[n] = true
	}
	for _, n := range switchyardGlobals {
		out[n] = true
	}
	return out
}

// starlarkUniverse is the canonical Starlark universe: the names available
// without imports in vanilla Starlark.
var starlarkUniverse = []string{
	"True", "False", "None",
	"bool", "bytes", "dict", "float", "int", "list", "set", "str", "tuple",
	"abs", "all", "any", "chr", "dir", "enumerate", "fail", "getattr", "hasattr",
	"hash", "len", "max", "min", "ord", "print", "range", "repr",
	"reversed", "sorted", "type", "zip",
}

// switchyardGlobals is the superset of names buildStdlib may expose.
// Keep in sync with internal/starlark/runtime.go:buildStdlib when adding
// new builtins.
var switchyardGlobals = []string{
	"state", "now", "time", "log",
	"call_service", "random",
	"sleep", "notify", "scene", "event",
}
