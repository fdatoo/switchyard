package script

import (
	"fmt"
	"strconv"
	"strings"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
)

// Script is the runtime representation of a compiled gohome.scripts entry.
// Immutable after Compile; callers obtain pointers under the engine RLock.
type Script struct {
	Name    string
	Params  []Param
	Handler string
}

// Param is a compiled, type-resolved parameter declaration.
type Param struct {
	Name     string
	Type     configpb.ScriptParam_Type
	Required bool
	// Default holds the pre-coerced default for non-required params.
	// When Required is true and no arg supplied, Engine.Call rejects.
	HasDefault bool
	Default    any
}

// Coerce converts the stringified input to the Param's declared type.
// Returns the typed value (string / int64 / float64 / bool) on success.
func (p Param) Coerce(s string) (any, error) {
	switch p.Type {
	case configpb.ScriptParam_TYPE_STRING:
		return s, nil
	case configpb.ScriptParam_TYPE_INT:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("param %q: expected int, got %q", p.Name, s)
		}
		return n, nil
	case configpb.ScriptParam_TYPE_FLOAT:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("param %q: expected float, got %q", p.Name, s)
		}
		return f, nil
	case configpb.ScriptParam_TYPE_BOOL:
		switch strings.ToLower(s) {
		case "true", "1", "yes":
			return true, nil
		case "false", "0", "no":
			return false, nil
		default:
			return nil, fmt.Errorf("param %q: expected bool, got %q", p.Name, s)
		}
	case configpb.ScriptParam_TYPE_ENTITY_ID:
		if !strings.Contains(s, ".") || strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") {
			return nil, fmt.Errorf("param %q: invalid entity id %q (expected \"<type>.<name>\")", p.Name, s)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("param %q: unknown type %v", p.Name, p.Type)
	}
}
