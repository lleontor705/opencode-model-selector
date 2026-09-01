// Package config — markdown agent types.
//
// MDAgent is the READ-ONLY representation of an OpenCode agent loaded from a
// markdown file (global ~/.config/opencode/agents/*.md or project
// .opencode/agents/*.md). Markdown agents are display-only in v1: edits always
// route to the inline-JSON agent object, never back to the .md file.
//
// The Raw map preserves ALL frontmatter fields — recognized ones with typed
// accessors plus any unknown/future keys — so that nothing is lost on load and
// mistyped recognized values are carried through uncoerced (decision #7).
package config

// RecognizedMDFields is the set of OpenCode agent frontmatter fields that have
// typed accessors on MDAgent. Any other frontmatter key (e.g. steps, color) is
// preserved verbatim in MDAgent.Raw but has no typed accessor.
var RecognizedMDFields = map[string]bool{
	"description": true,
	"mode":        true,
	"model":       true,
	"temperature": true,
	"permission":  true,
	"tools":       true,
}

// MDAgent is a markdown-backed agent. Name is the file stem (caller-assigned);
// Body is the system prompt text after the frontmatter; Raw holds every
// frontmatter field, recognized and unknown.
type MDAgent struct {
	// Name is the agent name derived from the file stem (without extension).
	Name string
	// Body is the markdown body following the YAML frontmatter — the agent's
	// system prompt.
	Body string
	// Raw holds all frontmatter fields verbatim. Recognized fields are also
	// available via typed accessors; unknown fields live only here.
	Raw map[string]interface{}
}

// Description returns the frontmatter "description" string, or "" when absent
// or non-string. Mistyped values are NOT coerced.
func (a MDAgent) Description() string {
	if s, ok := a.Raw["description"].(string); ok {
		return s
	}
	return ""
}

// Mode returns the frontmatter "mode" string, defaulting to "all" when absent
// or non-string. The value is returned verbatim — it is NOT validated against
// the primary|subagent|all vocabulary, so a mistyped value stays as-is.
func (a MDAgent) Mode() string {
	if s, ok := a.Raw["mode"].(string); ok {
		return s
	}
	return "all"
}

// Model returns the frontmatter "model" string and whether it was present as a
// string.
func (a MDAgent) Model() (string, bool) {
	s, ok := a.Raw["model"].(string)
	return s, ok
}

// Temperature returns the frontmatter "temperature" number and whether it was
// present as a numeric type (float64, int, or int64 — YAML may decode whole
// numbers as int).
func (a MDAgent) Temperature() (float64, bool) {
	switch v := a.Raw["temperature"].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// PermissionRaw returns the frontmatter "permission" value (typically a map of
// per-tool rules), or nil when absent. It is returned untyped because the
// shape varies (tool->allow|deny|ask or glob->rule).
func (a MDAgent) PermissionRaw() interface{} {
	return a.Raw["permission"]
}

// ToolsRaw returns the frontmatter "tools" value (e.g. {skill: false}), or nil
// when absent.
func (a MDAgent) ToolsRaw() interface{} {
	return a.Raw["tools"]
}
