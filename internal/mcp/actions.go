package mcp

import "github.com/fdatoo/switchyard/internal/auth"

// ToolActions maps tool names to auth actions.
var ToolActions = map[string]auth.Action{
	"gohome__get_state":         {Service: "MCP", Method: "get_state", Verb: "read"},
	"gohome__list_entities":     {Service: "MCP", Method: "list_entities", Verb: "read"},
	"gohome__call_capability":   {Service: "MCP", Method: "call_capability", Verb: "call"},
	"gohome__query_events":      {Service: "MCP", Method: "query_events", Verb: "read"},
	"gohome__tail_events":       {Service: "MCP", Method: "tail_events", Verb: "read"},
	"gohome__apply_scene":       {Service: "MCP", Method: "apply_scene", Verb: "call"},
	"gohome__run_script":        {Service: "MCP", Method: "run_script", Verb: "call"},
	"gohome__validate_config":   {Service: "MCP", Method: "validate_config", Verb: "read"},
	"gohome__apply_config":      {Service: "MCP", Method: "apply_config", Verb: "admin"},
	"gohome__eval_starlark":     {Service: "MCP", Method: "eval_starlark", Verb: "call"},
	"gohome__read_config_file":  {Service: "MCP", Method: "read_config_file", Verb: "read"},
	"gohome__write_config_file": {Service: "MCP", Method: "write_config_file", Verb: "admin"},
}

// ResourceActions maps resource URI prefixes to auth actions.
var ResourceActions = map[string]auth.Action{
	"gohome://entities/":    {Service: "MCP", Method: "subscribe_entities", Verb: "read"},
	"gohome://automations/": {Service: "MCP", Method: "trace_automation", Verb: "read"},
}
