// Package config — markdown + inline-JSON agent merge.
//
// MergeAgents combines the three agent layers into a unified map[string]*MergedAgent
// keyed by agent name. Per-field precedence (highest non-empty wins):
//
//	global markdown  <  project markdown  <  inline-JSON agent.<name>
//
// The merged system prompt uses the markdown body (project over global) and is
// overridden by an inline-JSON "prompt" (string) when present. Unknown markdown
// frontmatter fields are carried into MergedAgent.Raw (project over global) so
// nothing is lost. The merger keeps EVERY name (including system agents);
// filtering system agents is GetAgents' responsibility.
//
// Markdown agents are READ-ONLY in v1: this function never writes, and Save
// (config.go) writes only the inline-JSON config.
package config

import "sort"

// ModelProvenance identifies the config layer that supplied an effective
// agent model. It intentionally lives in config so catalog composition can map
// it without config importing a higher-level catalog package.
type ModelProvenance string

const (
	ModelProvenanceNone            ModelProvenance = "none"
	ModelProvenanceInlineJSON      ModelProvenance = "inline_json"
	ModelProvenanceProjectMarkdown ModelProvenance = "project_markdown"
	ModelProvenanceGlobalMarkdown  ModelProvenance = "global_markdown"
	ModelProvenanceGlobalTopLevel  ModelProvenance = "global_top_level"
)

// ResolveModel resolves an agent model across config layers. Agent-specific
// values use inline JSON > project markdown > global markdown precedence. The
// top-level JSON model is the final global fallback.
func ResolveModel(agentName string, globalMD, projectMD map[string]MDAgent, inlineJSON map[string]interface{}, globalModel string) (string, ModelProvenance, bool) {
	if agent, ok := inlineJSON[agentName].(map[string]interface{}); ok {
		if model, ok := nonEmptyString(agent["model"]); ok {
			return model, ModelProvenanceInlineJSON, true
		}
	}
	if agent, ok := projectMD[agentName]; ok {
		if model, ok := nonEmptyString(agent.Raw["model"]); ok {
			return model, ModelProvenanceProjectMarkdown, true
		}
	}
	if agent, ok := globalMD[agentName]; ok {
		if model, ok := nonEmptyString(agent.Raw["model"]); ok {
			return model, ModelProvenanceGlobalMarkdown, true
		}
	}
	if globalModel != "" {
		return globalModel, ModelProvenanceGlobalTopLevel, true
	}
	return "", ModelProvenanceNone, false
}

func nonEmptyString(value interface{}) (string, bool) {
	model, ok := value.(string)
	return model, ok && model != ""
}

// MergedAgent is the unified, read-oriented representation of an agent after
// combining the markdown and inline-JSON layers. It is what GetAgents consumes
// to build its name lists and what the TUI displays.
type MergedAgent struct {
	// Name is the agent name (inline-JSON key or markdown file stem).
	Name string
	// Fields holds the merged values for every key present across the layers
	// (recognized frontmatter fields, JSON-only fields such as disable/top_p,
	// and anything else). Per-field precedence: JSON > project md > global md.
	Fields map[string]interface{}
	// Prompt is the resolved system prompt: the merged markdown body (project
	// over global), overridden by an inline-JSON string "prompt" when present.
	Prompt string
	// Raw carries the unknown markdown frontmatter fields (keys not in the
	// recognized set), merged project-over-global, so they survive into the
	// result.
	Raw map[string]interface{}
	// HasInlineJSON is true when an inline-JSON agent.<name> object backs this
	// agent. MdOnly is the negation: a pure-markdown agent (non-editable).
	HasInlineJSON bool
	// MdOnly is true when this agent has NO inline-JSON backing — it exists
	// only in markdown. Such agents are display-only / non-editable in v1.
	MdOnly bool
}

// Mode returns the resolved agent mode ("primary"|"subagent"|"all"), defaulting
// to "all" when absent or non-string.
func (m *MergedAgent) Mode() string {
	return normalizeAgentMode(m.Fields["mode"])
}

// Disabled reports whether the agent has disable:true (a JSON-only field).
func (m *MergedAgent) Disabled() bool {
	b, _ := m.Fields["disable"].(bool)
	return b
}

// MergeAgents combines global markdown, project markdown, and inline-JSON agent
// layers into a unified map keyed by agent name. nil layers are treated as
// empty. See the package doc for the precedence rules.
func MergeAgents(globalMD, projectMD map[string]MDAgent, inlineJSON map[string]interface{}) map[string]*MergedAgent {
	result := map[string]*MergedAgent{}

	// Collect the union of names across all layers.
	names := map[string]bool{}
	for n := range globalMD {
		names[n] = true
	}
	for n := range projectMD {
		names[n] = true
	}
	for n := range inlineJSON {
		names[n] = true
	}

	for name := range names {
		g, gOK := globalMD[name]
		p, pOK := projectMD[name]
		jsonRaw, hasJSON := inlineJSON[name].(map[string]interface{})

		merged := &MergedAgent{
			Name:          name,
			Fields:        map[string]interface{}{},
			Raw:           map[string]interface{}{},
			HasInlineJSON: hasJSON,
			MdOnly:        !hasJSON,
		}

		// Sources in ASCENDING precedence order (last wins in pickField).
		mdSources := []map[string]interface{}{}
		if gOK && g.Raw != nil {
			mdSources = append(mdSources, g.Raw)
		}
		if pOK && p.Raw != nil {
			mdSources = append(mdSources, p.Raw)
		}
		allSources := mdSources
		if hasJSON {
			allSources = append(allSources, jsonRaw)
		}

		// Merge every key present in any source: JSON > project md > global md.
		mergedKeys := unionKeys(allSources...)
		for _, key := range mergedKeys {
			if v, ok := pickField(allSources, key); ok {
				merged.Fields[key] = v
			}
		}

		// Carry unknown markdown frontmatter fields (project over global).
		mdKeys := unionKeys(mdSources...)
		for _, key := range mdKeys {
			if RecognizedMDFields[key] {
				continue
			}
			if v, ok := pickField(mdSources, key); ok {
				merged.Raw[key] = v
			}
		}

		// Resolve the system prompt: merged md body (project over global), then
		// a JSON string "prompt" overrides.
		merged.Prompt = firstNonEmpty(p.Body, g.Body)
		if hasJSON {
			if s, ok := jsonRaw["prompt"].(string); ok && s != "" {
				merged.Prompt = s
			}
		}

		result[name] = merged
	}
	return result
}

// pickField returns the highest-precedence non-empty value for key across
// sources, where sources are in ASCENDING precedence order (so a later source
// overrides an earlier one). A nil or empty-string value does not override.
func pickField(sources []map[string]interface{}, key string) (interface{}, bool) {
	var found interface{}
	ok := false
	for _, src := range sources {
		if src == nil {
			continue
		}
		v, present := src[key]
		if !present || isEmptyValue(v) {
			continue
		}
		found = v
		ok = true
	}
	return found, ok
}

// unionKeys returns the deduplicated, sorted set of keys across the given maps,
// sorted for deterministic iteration order.
func unionKeys(maps ...map[string]interface{}) []string {
	seen := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			seen[k] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// firstNonEmpty returns the first argument that is a non-empty string, else "".
func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}

// isEmptyValue reports whether v should be treated as absent for merge
// precedence: nil always; "" for strings.
func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok && s == "" {
		return true
	}
	return false
}
