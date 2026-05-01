package mcp

import "github.com/fdatoo/switchyard/internal/auth"

// ToolActions maps tool names to auth actions.
var ToolActions = map[string]auth.Action{
	"switchyard__get_state":         {Service: "MCP", Method: "get_state", Verb: "read"},
	"switchyard__list_entities":     {Service: "MCP", Method: "list_entities", Verb: "read"},
	"switchyard__call_capability":   {Service: "MCP", Method: "call_capability", Verb: "call"},
	"switchyard__query_events":      {Service: "MCP", Method: "query_events", Verb: "read"},
	"switchyard__tail_events":       {Service: "MCP", Method: "tail_events", Verb: "read"},
	"switchyard__apply_scene":       {Service: "MCP", Method: "apply_scene", Verb: "call"},
	"switchyard__run_script":        {Service: "MCP", Method: "run_script", Verb: "call"},
	"switchyard__validate_config":   {Service: "MCP", Method: "validate_config", Verb: "read"},
	"switchyard__apply_config":      {Service: "MCP", Method: "apply_config", Verb: "admin"},
	"switchyard__eval_starlark":     {Service: "MCP", Method: "eval_starlark", Verb: "call"},
	"switchyard__read_config_file":  {Service: "MCP", Method: "read_config_file", Verb: "read"},
	"switchyard__write_config_file": {Service: "MCP", Method: "write_config_file", Verb: "admin"},
}

// ResourceActions maps resource URI prefixes to auth actions.
var ResourceActions = map[string]auth.Action{
	"switchyard://entities/":    {Service: "MCP", Method: "subscribe_entities", Verb: "read"},
	"switchyard://automations/": {Service: "MCP", Method: "trace_automation", Verb: "read"},
}
