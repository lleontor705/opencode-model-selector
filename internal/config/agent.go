package config

import "fmt"

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

// GetAgents returns all agent names grouped by mode: primary, subagents, and
// disabled. System agents are excluded from every slice. An agent with
// disable:true appears in the disabled slice AND in its mode slice (primary or
// subagent), so callers can filter disabled agents out of either group
// (REQ-CFG-008).
func (c *Config) GetAgents() (primary, subagents, disabled []string) {
	agents := c.agentMap()
	if agents == nil {
		return
	}

	for name, raw := range agents {
		if IsSystemAgent(name) {
			continue
		}
		agent, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		mode := getModeString(agent)
		isDisabled := getBoolField(agent, "disable")

		switch mode {
		case "primary":
			primary = append(primary, name)
		case "subagent":
			subagents = append(subagents, name)
		}

		if isDisabled {
			disabled = append(disabled, name)
		}
	}
	return
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
	s, ok := val.(string)
	if !ok {
		return "all"
	}
	return s
}

// getModeString extracts the "mode" string from an agent map, defaulting to
// "all" when absent or non-string.
func getModeString(agent map[string]interface{}) string {
	if m, ok := agent["mode"].(string); ok {
		return m
	}
	return "all"
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
