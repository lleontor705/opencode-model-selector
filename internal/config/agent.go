package config

import (
	"fmt"
	"sort"
	"strings"
)

// systemAgents is the set of agent names reserved for opencode's internal use.
// These are excluded from user-facing agent listings (GetAgents).
// The check is case-sensitive (REQ-CFG-008).
var systemAgents = map[string]bool{
	"compactación": true,
	"title":        true,
	"summary":      true,
}

// IsSystemAgent returns true if the agent name is a system agent
// ("compactación", "title", or "summary"). The match is case-sensitive.
func IsSystemAgent(name string) bool {
	return systemAgents[name]
}

// agentMap returns the "agent" section of the config as a typed map, or nil if
// the section is absent or not an object. Centralizes the type assertion so
// callers do not repeat it.
func (c *Config) agentMap() map[string]interface{} {
	if c.data == nil {
		return nil
	}
	agent, ok := c.data["agent"].(map[string]interface{})
	if !ok {
		return nil
	}
	return agent
}

// GetAgentField returns the value of a field for an agent. Returns (nil, false)
// if the agent or the field does not exist (REQ-CFG-003).
func (c *Config) GetAgentField(agentName, fieldName string) (interface{}, bool) {
	agents := c.agentMap()
	if agents == nil {
		return nil, false
	}
	agent, ok := agents[agentName].(map[string]interface{})
	if !ok {
		return nil, false
	}
	val, ok := agent[fieldName]
	if !ok {
		return nil, false
	}
	return val, true
}

// SetAgentField sets a field value for an agent. If the agent does not exist it
// is created automatically. Returns an error if the agent is disabled
// (disable: true), since disabled agents must not be mutated (REQ-CFG-005).
func (c *Config) SetAgentField(agentName, fieldName string, value interface{}) error {
	if c.IsAgentDisabled(agentName) {
		return fmt.Errorf("cannot modify disabled agent %q", agentName)
	}

	if c.data == nil {
		c.data = make(map[string]interface{})
	}

	agents, ok := c.data["agent"].(map[string]interface{})
	if !ok || agents == nil {
		agents = make(map[string]interface{})
		c.data["agent"] = agents
	}

	agent, ok := agents[agentName].(map[string]interface{})
	if !ok || agent == nil {
		agent = make(map[string]interface{})
		agents[agentName] = agent
	}

	agent[fieldName] = value
	return nil
}

// AgentGroups contains mutually exclusive role buckets plus disabled-agent
// metadata. An agent appears in exactly one of Primary, Subagents, or All, and
// may additionally appear in Disabled.
type AgentGroups struct {
	Primary   []string
	Subagents []string
	All       []string
	Disabled  []string
}

// GetAgents retains the legacy three-result API for existing callers. New
// callers that need agents whose role is "all" should use GetAgentGroups.
//
// The lists now include MARKDOWN agents discovered in the global
// (~/.config/opencode/agents/*.md) and project (<cwd>/.opencode/agents/*.md)
// layers, merged per-field with the inline-JSON agents (decision #5: JSON >
// project md > global md). An agent with mode "all" (the markdown default) is
// not placed in either the primary or subagent slice. A markdown-only agent
// with disable semantics is still governed by the inline-JSON "disable" flag
// only (markdown has no disable field), so pure-markdown agents are never
// disabled.
//
// The returned slices are sorted alphabetically for DETERMINISTIC order.
//
// Markdown FILES stay read-only: Save (config.go) writes the inline-JSON
// config only and never touches .md files. Markdown-backed agents ARE
// editable in the TUI: edits persist as inline-JSON overrides
// (agent.<name>.<field>) via SetAgentField — the OpenCode-native per-field
// overlay (the same path the tool's bulk "Apply to ALL" already uses).
// Pure-JSON GetAgentField/GetGlobalModel/decodeConfig/LoadConfig/GetConfigPath
// behavior is unchanged (REGRESS-001).
func (c *Config) GetAgents() (primary, subagents, disabled []string) {
	groups := c.GetAgentGroups()
	return groups.Primary, groups.Subagents, groups.Disabled
}

// GetAgentGroups returns all non-system agents grouped by normalized mode.
// Missing, empty, invalid, and explicit "all" modes use the distinct All
// bucket and are never duplicated into Primary or Subagents. Every slice is
// sorted alphabetically.
func (c *Config) GetAgentGroups() AgentGroups {
	var groups AgentGroups
	for name, m := range c.mergedAgents() {
		if IsSystemAgent(name) {
			continue
		}
		switch m.Mode() {
		case "primary":
			groups.Primary = append(groups.Primary, name)
		case "subagent":
			groups.Subagents = append(groups.Subagents, name)
		default:
			groups.All = append(groups.All, name)
		}
		if m.Disabled() {
			groups.Disabled = append(groups.Disabled, name)
		}
	}

	sort.Strings(groups.Primary)
	sort.Strings(groups.Subagents)
	sort.Strings(groups.All)
	sort.Strings(groups.Disabled)
	return groups
}

// mergedAgents is the shared core of GetAgents and MergedAgents: it discovers
// the markdown layers (global + project) and merges them with the inline-JSON
// agents into a unified map keyed by agent name.
func (c *Config) mergedAgents() map[string]*MergedAgent {
	inlineJSON := c.agentMap()
	if inlineJSON == nil {
		inlineJSON = map[string]interface{}{}
	}
	globalMD, projectMD := DiscoverMarkdownAgents(DefaultProjectAgentsDir())
	return MergeAgents(globalMD, projectMD, inlineJSON)
}

// MergedAgents returns the merged markdown + inline-JSON agent representation
// keyed by agent name (decision #5: JSON > project md > global md). The TUI
// uses this for display (e.g. the [MD] badge while an agent is still
// MdOnly). Markdown-backed agents ARE editable: edits persist as inline-JSON
// overrides via SetAgentField — the .md file is never written.
func (c *Config) MergedAgents() map[string]*MergedAgent {
	return c.mergedAgents()
}

// GetMergedAgentField returns the resolved (merged) value of a field for an
// agent, honoring the markdown + inline-JSON per-field precedence
// (JSON > project md > global md). It is the DISPLAY-side companion to
// SetAgentField: writes still go through SetAgentField (JSON only) — this
// reader never mutates anything.
//
// For a markdown-only agent this surfaces the md frontmatter value so the TUI
// can show the agent's real model (etc.) instead of "(none)". For a hybrid
// (JSON + md) agent the JSON value wins. Empty values are treated as absent
// (same rule the merger uses, so an explicit JSON "" does not shadow an md
// value).
//
// Returns (nil, false) when the agent does not exist or the field is absent.
func (c *Config) GetMergedAgentField(name, field string) (interface{}, bool) {
	merged := c.MergedAgents()
	m, ok := merged[name]
	if !ok {
		return nil, false
	}
	v, present := m.Fields[field]
	if !present || isEmptyValue(v) {
		return nil, false
	}
	return v, true
}

// GetGlobalModel returns the top-level "model" key value. Returns ("", false)
// if the key is absent or not a string (REQ-CFG-004).
func (c *Config) GetGlobalModel() (string, bool) {
	if c.data == nil {
		return "", false
	}
	model, ok := c.data["model"].(string)
	if !ok {
		return "", false
	}
	return model, true
}

// GetAgentModelOverride returns only the inline-JSON agent model override. It
// does not resolve markdown or the global model fallback.
func (c *Config) GetAgentModelOverride(agentName string) (string, bool) {
	value, ok := c.GetAgentField(agentName, "model")
	if !ok {
		return "", false
	}
	return nonEmptyString(value)
}

// SetAgentModelOverride writes only agent.<name>.model through the existing
// inline-JSON map. Compatibility field setters remain available during caller
// migration but are not exposed through this model-only method.
func (c *Config) SetAgentModelOverride(agentName, model string) error {
	return c.SetAgentField(agentName, "model", model)
}

// ResolveEffectiveModel returns the effective model for an agent and the layer
// that supplied it: inline JSON > project markdown > global markdown > global
// top-level JSON fallback.
func (c *Config) ResolveEffectiveModel(agentName string) (string, ModelProvenance, bool) {
	globalMD, projectMD := DiscoverMarkdownAgents(DefaultProjectAgentsDir())
	globalModel, _ := c.GetGlobalModel()
	inlineJSON := c.agentMap()
	if inlineJSON == nil {
		inlineJSON = map[string]interface{}{}
	}
	return ResolveModel(agentName, globalMD, projectMD, inlineJSON, globalModel)
}

// ModelSelection is the independent model, variant, and options state.
type ModelSelection struct {
	Model   string
	Variant string
	Options map[string]interface{}
}

// ResolveEffectiveVariant resolves variant independently; it has no top-level fallback.
func (c *Config) ResolveEffectiveVariant(agentName string) (string, ModelProvenance, bool) {
	globalMD, projectMD := DiscoverMarkdownAgents(DefaultProjectAgentsDir())
	return ResolveVariant(agentName, globalMD, projectMD, c.agentMap())
}

// ResolveVariant resolves inline JSON > project Markdown > global Markdown.
func ResolveVariant(agentName string, globalMD, projectMD map[string]MDAgent, inlineJSON map[string]interface{}) (string, ModelProvenance, bool) {
	if agent, ok := inlineJSON[agentName].(map[string]interface{}); ok {
		if value, ok := nonEmptyString(agent["variant"]); ok {
			return value, ModelProvenanceInlineJSON, true
		}
	}
	if agent, ok := projectMD[agentName]; ok {
		if value, ok := nonEmptyString(agent.Raw["variant"]); ok {
			return value, ModelProvenanceProjectMarkdown, true
		}
	}
	if agent, ok := globalMD[agentName]; ok {
		if value, ok := nonEmptyString(agent.Raw["variant"]); ok {
			return value, ModelProvenanceGlobalMarkdown, true
		}
	}
	return "", ModelProvenanceNone, false
}

// ResolveEffectiveSelection resolves each selection field independently.
func (c *Config) ResolveEffectiveSelection(agentName string) ModelSelection {
	model, _, _ := c.ResolveEffectiveModel(agentName)
	variant, _, _ := c.ResolveEffectiveVariant(agentName)
	return ModelSelection{Model: model, Variant: variant, Options: c.agentOptions(agentName)}
}

func (c *Config) agentOptions(agentName string) map[string]interface{} {
	result := map[string]interface{}{}
	model, _, _ := c.ResolveEffectiveModel(agentName)
	variant, _, _ := c.ResolveEffectiveVariant(agentName)
	c.mergeProviderOptions(result, model, variant)
	globalMD, projectMD := DiscoverMarkdownAgents(DefaultProjectAgentsDir())
	for _, layer := range []map[string]MDAgent{globalMD, projectMD} {
		if agent, ok := layer[agentName]; ok {
			if value, ok := agent.Raw["options"].(map[string]interface{}); ok {
				mergeOptionMap(result, value)
			}
		}
	}
	if agent, ok := c.agentMap()[agentName].(map[string]interface{}); ok {
		if value, ok := agent["options"].(map[string]interface{}); ok {
			mergeOptionMap(result, value)
		}
	}
	return result
}

func mergeOptionMap(dst, src map[string]interface{}) {
	for key, value := range src {
		dst[key] = value
	}
}

func (c *Config) mergeProviderOptions(dst map[string]interface{}, model, variant string) {
	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 || c.data == nil {
		return
	}
	providers, _ := c.data["provider"].(map[string]interface{})
	provider, _ := providers[parts[0]].(map[string]interface{})
	if value, ok := provider["options"].(map[string]interface{}); ok {
		mergeOptionMap(dst, value)
	}
	models, _ := provider["models"].(map[string]interface{})
	modelConfig, _ := models[parts[1]].(map[string]interface{})
	if value, ok := modelConfig["options"].(map[string]interface{}); ok {
		mergeOptionMap(dst, value)
	}
	variants, _ := modelConfig["variants"].(map[string]interface{})
	variantConfig, _ := variants[variant].(map[string]interface{})
	if value, ok := variantConfig["options"].(map[string]interface{}); ok {
		mergeOptionMap(dst, value)
	} else if variantConfig != nil {
		mergeOptionMap(dst, variantConfig)
	}
}

// SetAgentVariantOverride writes only agent.<name>.variant.
func (c *Config) SetAgentVariantOverride(agentName, variant string) error {
	return c.SetAgentField(agentName, "variant", variant)
}

// SetAgentOptionsOverride writes only agent.<name>.options.
func (c *Config) SetAgentOptionsOverride(agentName string, options map[string]interface{}) error {
	return c.SetAgentField(agentName, "options", options)
}

// SetGlobalModel sets the top-level "model" key (REQ-CFG-004).
func (c *Config) SetGlobalModel(model string) {
	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	c.data["model"] = model
}

// IsAgentDisabled returns true if the agent has disable: true (REQ-CFG-006).
func (c *Config) IsAgentDisabled(agentName string) bool {
	return getBoolFieldOrNil(c, agentName, "disable")
}

// IsAgentHidden returns true if the agent has hidden: true (REQ-CFG-007).
func (c *Config) IsAgentHidden(agentName string) bool {
	return getBoolFieldOrNil(c, agentName, "hidden")
}

// GetAgentMode returns the mode string for an agent ("primary", "subagent", or
// "all"). Returns "all" when the agent has no mode field (REQ-CFG-003).
func (c *Config) GetAgentMode(agentName string) string {
	val, ok := c.GetAgentField(agentName, "mode")
	if !ok {
		return "all"
	}
	return normalizeAgentMode(val)
}

// getModeString extracts the "mode" string from an agent map, defaulting to
// "all" when absent or non-string.
func getModeString(agent map[string]interface{}) string {
	return normalizeAgentMode(agent["mode"])
}

// normalizeAgentMode canonicalizes supported role values and safely falls
// back to "all" for absent, empty, non-string, or unknown values.
func normalizeAgentMode(value interface{}) string {
	mode, ok := value.(string)
	if !ok {
		return "all"
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "primary":
		return "primary"
	case "subagent":
		return "subagent"
	default:
		return "all"
	}
}

// getBoolField extracts a boolean field from an agent map, defaulting to false.
func getBoolField(agent map[string]interface{}, fieldName string) bool {
	b, ok := agent[fieldName].(bool)
	return ok && b
}

// getBoolFieldOrNil routes through GetAgentField so the lookup handles missing
// agents and missing fields uniformly.
func getBoolFieldOrNil(c *Config, agentName, fieldName string) bool {
	val, ok := c.GetAgentField(agentName, fieldName)
	if !ok {
		return false
	}
	b, ok := val.(bool)
	return ok && b
}
