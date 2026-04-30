package starlark

import "go.starlark.net/syntax"

// ParseOnly parses src as a Starlark expression (expr=true) or script (expr=false).
// Returns a syntax error if parsing fails; nil on success. Does not execute.
func ParseOnly(src string, expr bool) error {
	opts := &syntax.FileOptions{}
	if expr {
		_, err := opts.ParseExpr("<input>", src, 0)
		return err
	}
	_, err := opts.Parse("<input>", src, 0)
	return err
}
