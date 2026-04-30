package fs

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"go.starlark.net/syntax"
)

// ErrUnsupportedExtension is returned for non-.pkl, non-.star files.
var ErrUnsupportedExtension = errors.New("unsupported extension; expected .pkl or .star")

// SyntaxError carries the offending path plus 1-based line/column.
type SyntaxError struct {
	Path    string
	Line    int
	Column  int
	Message string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Path, e.Line, e.Column, e.Message)
}

// CheckSyntax does a best-effort parse of content based on path's extension.
// .pkl files use a lightweight brace/bracket balance check (no Pkl binary
// required). .star files use go.starlark.net/syntax.
func CheckSyntax(path string, content []byte) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pkl":
		return checkPkl(path, content)
	case ".star":
		return checkStarlark(path, content)
	default:
		return ErrUnsupportedExtension
	}
}

func checkStarlark(path string, content []byte) error {
	_, err := (&syntax.FileOptions{}).Parse(path, content, 0)
	if err == nil {
		return nil
	}
	if se, ok := err.(syntax.Error); ok {
		return &SyntaxError{
			Path:    path,
			Line:    int(se.Pos.Line),
			Column:  int(se.Pos.Col),
			Message: se.Msg,
		}
	}
	return &SyntaxError{Path: path, Line: 1, Message: err.Error()}
}

// checkPkl does a lightweight structural check: balanced braces/brackets and
// no unterminated string literals. This avoids needing the pkl binary at
// MCP-server runtime. False negatives (accepting invalid Pkl) are possible
// but acceptable; false positives (rejecting valid Pkl) are bugs.
func checkPkl(path string, content []byte) error {
	depth := 0
	inStr := false
	escape := false
	for i, b := range content {
		if escape {
			escape = false
			continue
		}
		if inStr {
			switch b {
			case '\\':
				escape = true
			case '"':
				inStr = false
			}
			continue
		}
		switch b {
		case '"':
			inStr = true
		case '{', '(':
			depth++
		case '}', ')':
			depth--
			if depth < 0 {
				line := strings.Count(string(content[:i]), "\n") + 1
				return &SyntaxError{Path: path, Line: line, Column: 1, Message: "unexpected closing delimiter"}
			}
		}
	}
	if inStr {
		return &SyntaxError{Path: path, Line: 1, Message: "unterminated string literal"}
	}
	if depth != 0 {
		return &SyntaxError{Path: path, Line: 1, Message: "unbalanced delimiters"}
	}
	return nil
}
