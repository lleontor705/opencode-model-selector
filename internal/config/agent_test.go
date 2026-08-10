package config

import (
	"os"
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
	// Isolate $HOME so the host's global markdown agents do not leak into the
	// pure-JSON fixture's GetAgents result (markdown discovery is HOME-based).
	setHomeEnv(t, t.TempDir())

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

// ---------------------------------------------------------------------------
// GetAgents markdown integration + READ-ONLY save (T-B5)
//
// GetAgents now also discovers and merges markdown agents (global + project)
// with inline-JSON agents, returning alphabetically ordered slices. Markdown
// agents are READ-ONLY: Save writes JSON only and never touches .md files.
// Pure-JSON behavior of GetAgentField/GetGlobalModel/GetAgentMode/LoadConfig is
// unchanged (REGRESS-001).
// ---------------------------------------------------------------------------

// writeGlobalMD writes a markdown agent into <home>/.config/opencode/agents and
// returns its path.
func writeGlobalMD(t *testing.T, home, name, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "opencode", "agents")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

// TestGetAgents_IncludesMDAgents verifies that global markdown agents appear in
// the primary/subagent lists alongside inline-JSON agents.
func TestGetAgents_IncludesMDAgents(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "md-sub.md", "---\nmode: subagent\nmodel: md-model\n---\nMd body.\n")
	writeGlobalMD(t, home, "md-prim.md", "---\nmode: primary\n---\nPrim body.\n")

	cfg := &Config{data: map[string]interface{}{
		"agent": map[string]interface{}{
			"build": map[string]interface{}{"mode": "primary"},
		},
	}}

	primary, subagents, _ := cfg.GetAgents()
	assert.Contains(t, primary, "build", "inline-JSON primary agent present")
	assert.Contains(t, primary, "md-prim", "global md primary agent included")
	assert.Contains(t, subagents, "md-sub", "global md subagent included")
}

// TestGetAgents_AlphabeticalOrder verifies primary/subagents/disabled are
// returned in deterministic alphabetical order.
func TestGetAgents_AlphabeticalOrder(t *testing.T) {
	setHomeEnv(t, t.TempDir()) // no md -> pure JSON
	cfg := &Config{data: map[string]interface{}{
		"agent": map[string]interface{}{
			"zebra": map[string]interface{}{"mode": "primary"},
			"alpha": map[string]interface{}{"mode": "primary"},
			"mid":   map[string]interface{}{"mode": "subagent"},
			"beta":  map[string]interface{}{"mode": "subagent"},
		},
	}}

	primary, subagents, disabled := cfg.GetAgents()
	assert.Equal(t, []string{"alpha", "zebra"}, primary, "primary must be alphabetical")
	assert.Equal(t, []string{"beta", "mid"}, subagents, "subagents must be alphabetical")
	assert.Empty(t, disabled)
}

// TestSave_DoesNotWriteMarkdownAgents verifies that Save (JSON-only) never
// creates or modifies markdown agent files. Markdown is READ-ONLY in v1.
func TestSave_DoesNotWriteMarkdownAgents(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	mdContent := []byte("---\nmode: subagent\nmodel: original\n---\nOriginal body.\n")
	mdPath := writeGlobalMD(t, home, "locked.md", string(mdContent))

	cfgPath := filepath.Join(t.TempDir(), "opencode.json")
	cfg := &Config{path: cfgPath, data: map[string]interface{}{
		"agent": map[string]interface{}{
			"build": map[string]interface{}{"mode": "primary"},
		},
	}}
	require.NoError(t, cfg.SetAgentField("build", "model", "openai/gpt-5"))
	require.NoError(t, cfg.Save())

	// The markdown file must be byte-for-byte unchanged.
	got, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Equal(t, string(mdContent), string(got),
		"Save MUST NOT modify markdown agent files")

	// No new markdown files created in the agents dir.
	matches, err := filepath.Glob(filepath.Join(home, ".config", "opencode", "agents", "*.md"))
	require.NoError(t, err)
	assert.Len(t, matches, 1, "Save must not create new markdown files")
}

// TestInlineJSON_PathsUnchanged verifies that with no markdown present, the
// pure-JSON read paths (GetAgentField/GetGlobalModel/GetAgentMode and the
// GetAgents grouping/counts) are identical to pre-change behavior (REGRESS-001).
func TestInlineJSON_PathsUnchanged(t *testing.T) {
	setHomeEnv(t, t.TempDir()) // no md -> pure JSON
	cfg, err := LoadConfig(fixturePath(t, "opencode.json"))
	require.NoError(t, err)

	// GetAgentField reads inline JSON only.
	val, ok := cfg.GetAgentField("code-reviewer", "model")
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", val)

	val, ok = cfg.GetAgentField("build", "model")
	assert.False(t, ok, "build has no model field (unchanged)")
	assert.Nil(t, val)

	// GetGlobalModel unchanged.
	_, gok := cfg.GetGlobalModel()
	assert.False(t, gok, "fixture has no top-level model key")

	// GetAgentMode unchanged.
	assert.Equal(t, "subagent", cfg.GetAgentMode("code-reviewer"))
	assert.Equal(t, "primary", cfg.GetAgentMode("plan"))

	// GetAgents grouping/counts unchanged for pure JSON (system excluded).
	primary, subagents, disabled := cfg.GetAgents()
	assert.Len(t, primary, 2)
	assert.Len(t, subagents, 9)
	assert.Len(t, disabled, 1)
}
