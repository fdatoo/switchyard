package starlark_test

import (
	"testing"

	ghs "github.com/fdatoo/gohome/internal/starlark"
)

func TestParseOnly_ValidExpression(t *testing.T) {
	if err := ghs.ParseOnly("1 + 2", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseOnly_ValidScript(t *testing.T) {
	src := "x = 1\nif x > 0:\n    x = x + 1"
	if err := ghs.ParseOnly(src, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseOnly_SyntaxError(t *testing.T) {
	if err := ghs.ParseOnly("def foo(:", true); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestParseOnly_StatementAsExpression(t *testing.T) {
	// "x = 1" is a statement, not an expression; ParseExpr should reject it.
	if err := ghs.ParseOnly("x = 1", true); err == nil {
		t.Fatal("expected parse error for statement as expression")
	}
}

func TestParseOnly_ScriptAcceptsStatements(t *testing.T) {
	if err := ghs.ParseOnly("x = 1", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
