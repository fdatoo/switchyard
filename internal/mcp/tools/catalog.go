package tools

// ToolDescriptor describes one MCP tool for display.
type ToolDescriptor struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Verb    string `json:"verb"`
	Status  string `json:"status"` // "live" or "unimplemented"
}

// Catalog returns the static tool catalog (12 tools).
func Catalog() []ToolDescriptor {
	return []ToolDescriptor{
		{Name: "switchyard__get_state", Summary: "Get current state of one entity", Verb: "read", Status: "live"},
		{Name: "switchyard__list_entities", Summary: "Browse entities with optional filters", Verb: "read", Status: "live"},
		{Name: "switchyard__call_capability", Summary: "Invoke a capability on one entity", Verb: "call", Status: "live"},
		{Name: "switchyard__query_events", Summary: "Query the event log with filters", Verb: "read", Status: "live"},
		{Name: "switchyard__tail_events", Summary: "Stream recent events with a deadline", Verb: "read", Status: "live"},
		{Name: "switchyard__apply_scene", Summary: "Apply a named scene (UNIMPLEMENTED)", Verb: "call", Status: "unimplemented"},
		{Name: "switchyard__run_script", Summary: "Run a named Starlark script", Verb: "call", Status: "live"},
		{Name: "switchyard__eval_starlark", Summary: "Evaluate a Starlark expression (output capped 64KiB)", Verb: "call", Status: "live"},
		{Name: "switchyard__validate_config", Summary: "Validate a Pkl config bundle without applying", Verb: "read", Status: "live"},
		{Name: "switchyard__apply_config", Summary: "Apply a Pkl config bundle to the daemon", Verb: "admin", Status: "live"},
		{Name: "switchyard__read_config_file", Summary: "Read a file from the config dir", Verb: "read", Status: "live"},
		{Name: "switchyard__write_config_file", Summary: "Write a file to the config dir (with syntax check)", Verb: "admin", Status: "live"},
	}
}
