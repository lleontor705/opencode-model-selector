package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// IsSystemAgent (package-level helper)
// ---------------------------------------------------------------------------

func TestIsSystemAgent_TrueForCompactacion(t *testing.T) {
	assert.True(t, IsSystemAgent("compactación"),
		"compactación is a system agent (case-sensitive)")
}

func TestIsSystemAgent_TrueForTitle(t *testing.T) {
	assert.True(t, IsSystemAgent("title"))
}

func TestIsSystemAgent_TrueForSummary(t *testing.T) {
	assert.True(t, IsSystemAgent("summary"))
}

func TestIsSystemAgent_FalseForBuild(t *testing.T) {
	assert.False(t, IsSystemAgent("build"))
}

func TestIsSystemAgent_CaseSensitive(t *testing.T) {
	// Case-sensitive check: Title != title.
	assert.False(t, IsSystemAgent("Title"))
	assert.False(t, IsSystemAgent("SUMMARY"))
}

// ---------------------------------------------------------------------------
// GetAgents (REQ-CFG-008)
// ---------------------------------------------------------------------------

func TestGetAgents_ReturnsThreeGroupsExcludingSystem(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	primary, subagents, disabled := cfg.GetAgents()

	// System agents excluded from ALL slices.
	for _, sys := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, primary, sys, "system agent %q must not be in primary", sys)
		assert.NotContains(t, subagents, sys, "system agent %q must not be in subagents", sys)
		assert.NotContains(t, disabled, sys, "system agent %q must not be in disabled", sys)
	}

	// Primary non-system: build, plan.
	assert.Contains(t, primary, "build")
	assert.Contains(t, primary, "plan")
	assert.Len(t, primary, 2, "primary must contain exactly build and plan")

	// Subagents non-system (9 of them).
	expectedSubs := []string{
		"general", "explore", "code-reviewer", "debug", "docs",
		"security-auditor", "orchestrator", "team-lead", "parallel-dispatch",
	}
	for _, name := range expectedSubs {
		assert.Contains(t, subagents, name, "subagents must contain %q", name)
	}
	assert.Len(t, subagents, 9, "subagents must contain exactly 9 entries")

	// Disabled: build only — but build ALSO appears in primary.
	assert.Contains(t, disabled, "build")
	assert.Len(t, disabled, 1, "disabled must contain exactly build")
}

// ---------------------------------------------------------------------------
// GetAgentField (REQ-CFG-003)
// ---------------------------------------------------------------------------

func TestGetAgentField_ExistingField(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	val, ok := cfg.GetAgentField("code-reviewer", "model")
	require.True(t, ok, "code-reviewer.model must exist")
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", val)
}

func TestGetAgentField_MissingField(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	val, ok := cfg.GetAgentField("build", "model")
	assert.False(t, ok, "build has no model field")
	assert.Nil(t, val)
}

func TestGetAgentField_NonExistentAgent(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	val, ok := cfg.GetAgentField("no-such-agent", "model")
	assert.False(t, ok)
	assert.Nil(t, val)
}

// ---------------------------------------------------------------------------
// SetAgentField (REQ-CFG-005)
// ---------------------------------------------------------------------------

func TestSetAgentField_ExistingAgentPersists(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	cfg.path = filepath.Join(t.TempDir(), "opencode.json")

	require.NoError(t, cfg.SetAgentField("plan", "model", "glm-5.2"))
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(cfg.path)
	require.NoError(t, err)

	val, ok := reloaded.GetAgentField("plan", "model")
	require.True(t, ok, "plan.model must exist after reload")
	assert.Equal(t, "glm-5.2", val)
}

func TestSetAgentField_CreatesNonExistentAgent(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	require.NoError(t, cfg.SetAgentField("new-agent", "model", "glm-5.2"))

	val, ok := cfg.GetAgentField("new-agent", "model")
	require.True(t, ok, "newly created agent must expose the field")
	assert.Equal(t, "glm-5.2", val)
}

func TestSetAgentField_DisabledAgentReturnsError(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	err = cfg.SetAgentField("build", "model", "glm-5.2")
	require.Error(t, err, "must refuse to modify a disabled agent")
}

// ---------------------------------------------------------------------------
// GetGlobalModel / SetGlobalModel (REQ-CFG-004)
// ---------------------------------------------------------------------------

func TestGetGlobalModel_NotSet(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	val, ok := cfg.GetGlobalModel()
	assert.False(t, ok, "fixture has no top-level model key")
	assert.Equal(t, "", val)
}

func TestSetGlobalModel_PersistsThroughRoundTrip(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	cfg.path = filepath.Join(t.TempDir(), "opencode.json")

	cfg.SetGlobalModel("glm-5.2")
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(cfg.path)
	require.NoError(t, err)

	val, ok := reloaded.GetGlobalModel()
	require.True(t, ok, "top-level model must exist after reload")
	assert.Equal(t, "glm-5.2", val)
}

// ---------------------------------------------------------------------------
// IsAgentDisabled (REQ-CFG-006)
// ---------------------------------------------------------------------------

func TestIsAgentDisabled_TrueForBuild(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.True(t, cfg.IsAgentDisabled("build"), "build has disable:true")
}

func TestIsAgentDisabled_FalseForPlan(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.False(t, cfg.IsAgentDisabled("plan"), "plan has no disable field")
}

func TestIsAgentDisabled_FalseForUnknownAgent(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.False(t, cfg.IsAgentDisabled("no-such-agent"))
}

// ---------------------------------------------------------------------------
// IsAgentHidden (REQ-CFG-007)
// ---------------------------------------------------------------------------

func TestIsAgentHidden_TrueForParallelDispatch(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.True(t, cfg.IsAgentHidden("parallel-dispatch"), "parallel-dispatch has hidden:true")
}

func TestIsAgentHidden_FalseForPlan(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.False(t, cfg.IsAgentHidden("plan"))
}

// ---------------------------------------------------------------------------
// GetAgentMode (REQ-CFG-003)
// ---------------------------------------------------------------------------

func TestGetAgentMode_Subagent(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.Equal(t, "subagent", cfg.GetAgentMode("code-reviewer"))
}

func TestGetAgentMode_Primary(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	assert.Equal(t, "primary", cfg.GetAgentMode("plan"))
}

func TestGetAgentMode_DefaultAll(t *testing.T) {
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	// Agent without mode field returns "all" default.
	require.NoError(t, cfg.SetAgentField("no-mode-agent", "description", "test"))
	assert.Equal(t, "all", cfg.GetAgentMode("no-mode-agent"))
}
