package script_test

import (
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/script"
)

func TestParam_Coerce_String(t *testing.T) {
	p := script.Param{Name: "who", Type: configpb.ScriptParam_TYPE_STRING, Required: true}
	v, err := p.Coerce("alice")
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "alice" {
		t.Fatalf("got %v", v)
	}
}

func TestParam_Coerce_IntOK(t *testing.T) {
	p := script.Param{Name: "n", Type: configpb.ScriptParam_TYPE_INT, Required: true}
	v, err := p.Coerce("42")
	if err != nil {
		t.Fatal(err)
	}
	if v.(int64) != 42 {
		t.Fatalf("got %v", v)
	}
}

func TestParam_Coerce_IntBad(t *testing.T) {
	p := script.Param{Name: "n", Type: configpb.ScriptParam_TYPE_INT, Required: true}
	if _, err := p.Coerce("abc"); err == nil {
		t.Fatal("want error")
	}
}

func TestParam_Coerce_FloatOK(t *testing.T) {
	p := script.Param{Name: "r", Type: configpb.ScriptParam_TYPE_FLOAT, Required: true}
	v, err := p.Coerce("1.5")
	if err != nil {
		t.Fatal(err)
	}
	if v.(float64) != 1.5 {
		t.Fatalf("got %v", v)
	}
}

func TestParam_Coerce_Bool(t *testing.T) {
	p := script.Param{Name: "b", Type: configpb.ScriptParam_TYPE_BOOL, Required: true}
	for _, tt := range []struct {
		in   string
		want bool
	}{{"true", true}, {"false", false}, {"1", true}, {"0", false}} {
		v, err := p.Coerce(tt.in)
		if err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		if v.(bool) != tt.want {
			t.Fatalf("%s: got %v", tt.in, v)
		}
	}
}

func TestParam_Coerce_EntityIDOK(t *testing.T) {
	p := script.Param{Name: "e", Type: configpb.ScriptParam_TYPE_ENTITY_ID, Required: true}
	v, err := p.Coerce("light.kitchen")
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "light.kitchen" {
		t.Fatalf("got %v", v)
	}
}

func TestParam_Coerce_EntityIDBad(t *testing.T) {
	p := script.Param{Name: "e", Type: configpb.ScriptParam_TYPE_ENTITY_ID, Required: true}
	if _, err := p.Coerce("notanid"); err == nil {
		t.Fatal("want error for missing dot")
	}
}
