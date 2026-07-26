package config

import (
	"fmt"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// ApplyModelToAgents applies the given model ID to a set of agents in a single
// in-memory pass (REQ-BULK-001..006).
//
// The model is validated before any mutation occurs: if it is not present in
// available (exact, case-sensitive match on FullName) no agent is touched and
// (nil, nil, error) is returned (REQ-BULK-004).
//
// When names is nil, targets are resolved from GetAgents (primary + subagents);
// system agents are already excluded by GetAgents. When names is non-nil (even
// if empty) it is used verbatim, and any system agent it contains is skipped
// (REQ-BULK-002).
//
// Disabled agents are skipped and reported in skipped (REQ-BULK-003). Agents
// whose current model already equals modelID are skipped (idempotent) and
// reported in skipped rather than applied (REQ-BULK-006). Returns (applied,
// skipped, nil) on success where applied and skipped are non-nil empty slices
// when no agents match.
func ApplyModelToAgents(
	cfg *Config,
	modelID string,
	names []string,
	available []opencode.Model,
) (applied, skipped []string, err error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	if !ValidateModel(modelID, available) {
		return nil, nil, fmt.Errorf("invalid model %q: not in available models", modelID)
	}

	var targets []string
	if names == nil {
		primary, subagents, _ := cfg.GetAgents()
		targets = append(primary, subagents...)
	} else {
		targets = append([]string(nil), names...)
	}

	applied = []string{}
	skipped = []string{}

	for _, name := range targets {
		if IsSystemAgent(name) {
			skipped = append(skipped, name)
			continue
		}
		if cfg.IsAgentDisabled(name) {
			skipped = append(skipped, name)
			continue
		}
		if cur, ok := cfg.GetAgentField(name, "model"); ok {
			if s, _ := cur.(string); s == modelID {
				skipped = append(skipped, name)
				continue
			}
		}
		if err := cfg.SetAgentField(name, "model", modelID); err != nil {
			skipped = append(skipped, name)
			continue
		}
		applied = append(applied, name)
	}

	return applied, skipped, nil
}
