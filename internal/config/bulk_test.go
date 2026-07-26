package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// ---------------------------------------------------------------------------
// ApplyModelToAgents — REQ-BULK-001..006
//
// ApplyModelToAgents applies a model ID to a set of agents in a single
// in-memory pass. It validates the model before any mutation, resolves
// targets via GetAgents (system agents excluded), skips disabled agents,
// and is idempotent when an agent already has the target model.
// ---------------------------------------------------------------------------

// bulkFixtureModels returns a deterministic available-models list for tests.
func bulkFixtureModels() []opencode.Model {
	return []opencode.Model{
		{Provider: "openai", ID: "gpt-5", FullName: "openai/gpt-5"},
		{Provider: "anthropic", ID: "claude-sonnet-4", FullName: "anthropic/claude-sonnet-4"},
		{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
	}
}

// newBulkConfig builds a minimal Config with the given agents.
// Each agent map is the raw "agent" section value for one agent.
func newBulkConfig(t *testing.T, agents map[string]map[string]interface{}) *Config {
	t.Helper()
	agentSection := make(map[string]interface{}, len(agents))
	for name, fields := range agents {
		agentSection[name] = fields
	}
	return &Config{
		data: map[string]interface{}{
			"agent": agentSection,
		},
	}
}

// agentModel returns the current model field for an agent, or ("", false).
func agentModel(t *testing.T, cfg *Config, name string) (string, bool) {
	t.Helper()
	val, ok := cfg.GetAgentField(name, "model")
	if !ok {
		return "", false
	}
	s, _ := val.(string)
	return s, true
}

// ---------------------------------------------------------------------------
// REQ-BULK-001: Atomic in-memory bulk application
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_AllTargetsApplyCleanly(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build":   {"mode": "primary"},
		"plan":    {"mode": "primary"},
		"explore": {"mode": "subagent"},
		"review":  {"mode": "subagent"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, skipped)
	assert.ElementsMatch(t, []string{"build", "plan", "explore", "review"}, applied)

	for _, name := range []string{"build", "plan", "explore", "review"} {
		model, ok := agentModel(t, cfg, name)
		require.True(t, ok, "agent %q must have model set", name)
		assert.Equal(t, "openai/gpt-5", model)
	}
}

func TestApplyModelToAgents_SingleTarget(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", []string{"build"}, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, skipped)
	assert.Equal(t, []string{"build"}, applied)

	model, ok := agentModel(t, cfg, "build")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", model)
}

func TestApplyModelToAgents_ModelNotInAvailable(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
		"plan":  {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "bogus/model", nil, bulkFixtureModels())

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Nil(t, skipped)

	// Zero mutations.
	for _, name := range []string{"build", "plan"} {
		_, ok := agentModel(t, cfg, name)
		assert.False(t, ok, "agent %q must remain unmodified", name)
	}
}

// ---------------------------------------------------------------------------
// REQ-BULK-002: System agents excluded automatically
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_SystemAgentsExcludedFromNilNames(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"compactación": {"mode": "primary"},
		"title":        {"mode": "primary"},
		"summary":      {"mode": "primary"},
		"build":        {"mode": "primary"},
		"plan":         {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, skipped)
	assert.ElementsMatch(t, []string{"build", "plan"}, applied)

	// System agents must remain untouched.
	for _, sys := range []string{"compactación", "title", "summary"} {
		_, ok := agentModel(t, cfg, sys)
		assert.False(t, ok, "system agent %q must not be modified", sys)
	}
}

func TestApplyModelToAgents_SystemAgentInCSVSkipped(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
		"title": {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", []string{"build", "title"}, bulkFixtureModels())

	require.NoError(t, err)
	assert.Equal(t, []string{"build"}, applied)
	assert.Contains(t, skipped, "title")

	// System agent unchanged.
	_, ok := agentModel(t, cfg, "title")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// REQ-BULK-003: Disabled agents skipped and reported
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_MixedEnabledDisabled(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build":   {"mode": "primary"},
		"plan":    {"mode": "primary", "disable": true},
		"explore": {"mode": "subagent"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"build", "explore"}, applied)
	assert.ElementsMatch(t, []string{"plan"}, skipped)

	// Disabled agent unchanged.
	_, ok := agentModel(t, cfg, "plan")
	assert.False(t, ok)
}

func TestApplyModelToAgents_AllTargetsDisabled(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary", "disable": true},
		"plan":  {"mode": "primary", "disable": true},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, applied)
	assert.ElementsMatch(t, []string{"build", "plan"}, skipped)

	// No mutation on any agent.
	for _, name := range []string{"build", "plan"} {
		_, ok := agentModel(t, cfg, name)
		assert.False(t, ok)
	}
}

func TestApplyModelToAgents_CSVNamedDisabledSkipped(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"plan": {"mode": "primary", "disable": true},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", []string{"plan"}, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, applied)
	assert.Equal(t, []string{"plan"}, skipped)
}

// ---------------------------------------------------------------------------
// REQ-BULK-004: Model validated before any mutation
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_CaseMismatchModelError(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "OpenAI/GPT-5", nil, bulkFixtureModels())

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Nil(t, skipped)

	_, ok := agentModel(t, cfg, "build")
	assert.False(t, ok)
}

func TestApplyModelToAgents_EmptyAvailableListError(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "any/model", nil, []opencode.Model{})

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Nil(t, skipped)

	_, ok := agentModel(t, cfg, "build")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// REQ-BULK-005: Empty target list returns success
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_EmptyCSVReturnsEmptySuccess(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", []string{}, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, applied)
	assert.Empty(t, skipped)
}

func TestApplyModelToAgents_OnlySystemAgentsReturnsEmptySuccess(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"compactación": {"mode": "primary"},
		"title":        {"mode": "primary"},
		"summary":      {"mode": "primary"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.Empty(t, applied)
	assert.Empty(t, skipped)
}

// ---------------------------------------------------------------------------
// REQ-BULK-006: Idempotent no-op when already on target
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_IdempotentNoOpWhenAlreadyOnTarget(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary", "model": "openai/gpt-5"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", []string{"build"}, bulkFixtureModels())

	require.NoError(t, err)
	// Per design: already-on-target agents appear in `skipped`, NOT `applied`.
	// The spec scenario says "applied == ['build']", but the design internal
	// flow explicitly skips already-set agents and puts them in `skipped`.
	// We follow the design contract.
	assert.Empty(t, applied)
	assert.Equal(t, []string{"build"}, skipped)

	model, ok := agentModel(t, cfg, "build")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", model)
}

func TestApplyModelToAgents_MixedSomeAlreadyOnTarget(t *testing.T) {
	cfg := newBulkConfig(t, map[string]map[string]interface{}{
		"build": {"mode": "primary", "model": "openai/gpt-5"},
		"plan":  {"mode": "primary", "model": "anthropic/claude-sonnet-4"},
	})

	applied, skipped, err := ApplyModelToAgents(cfg, "openai/gpt-5", nil, bulkFixtureModels())

	require.NoError(t, err)
	assert.Equal(t, []string{"plan"}, applied)
	assert.Equal(t, []string{"build"}, skipped)

	buildModel, _ := agentModel(t, cfg, "build")
	assert.Equal(t, "openai/gpt-5", buildModel, "build must remain unchanged")

	planModel, _ := agentModel(t, cfg, "plan")
	assert.Equal(t, "openai/gpt-5", planModel, "plan must be updated")
}

// ---------------------------------------------------------------------------
// Defensive: nil config
// ---------------------------------------------------------------------------

func TestApplyModelToAgents_NilConfig(t *testing.T) {
	applied, skipped, err := ApplyModelToAgents(nil, "openai/gpt-5", nil, bulkFixtureModels())

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Nil(t, skipped)
	assert.Contains(t, err.Error(), "nil")
}
