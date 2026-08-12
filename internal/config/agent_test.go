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

// ---------------------------------------------------------------------------
// GetMergedAgentField — display-only reader honoring md + inline-JSON
// precedence (T-B7)
//
// GetMergedAgentField returns the resolved (merged) value for an agent field,
// honoring per-field precedence (JSON > project md > global md). It is the
// display-side companion to SetAgentField: writes still go through
// SetAgentField (JSON only). For a markdown-only agent this returns the md
// value, so the TUI can show the agent's actual model instead of "(none)".
// ---------------------------------------------------------------------------

// TestGetMergedAgentField_ReturnsMdValueWhenNoJSON verifies that for an agent
// that exists only in markdown (MdOnly=true), GetMergedAgentField returns the
// markdown frontmatter value. The pure-JSON GetAgentField returns (nil,false)
// for the same agent — the merge layer is what surfaces the md value.
func TestGetMergedAgentField_ReturnsMdValueWhenNoJSON(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "review.md",
		"---\nmode: subagent\nmodel: anthropic/claude-3-opus\n---\nReview body.\n")

	cfg := &Config{data: map[string]interface{}{}} // no JSON agents

	// Pure-JSON reader: no inline-JSON entry, so absent.
	v, ok := cfg.GetAgentField("review", "model")
	assert.False(t, ok, "GetAgentField reads JSON only — must be absent")
	assert.Nil(t, v)

	// Merged reader: surfaces the md value.
	got, ok := cfg.GetMergedAgentField("review", "model")
	require.True(t, ok, "GetMergedAgentField MUST return the md model value")
	assert.Equal(t, "anthropic/claude-3-opus", got)
}

// TestGetMergedAgentField_JSONOverridesMd verifies that when both JSON and md
// define a field, the JSON value wins (per-field precedence), so the user's
// inline-JSON override is reflected in the merged display.
func TestGetMergedAgentField_JSONOverridesMd(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "review.md",
		"---\nmode: subagent\nmodel: md-original\n---\nBody.\n")

	cfg := &Config{data: map[string]interface{}{
		"agent": map[string]interface{}{
			"review": map[string]interface{}{
				"model": "json-override",
			},
		},
	}}

	got, ok := cfg.GetMergedAgentField("review", "model")
	require.True(t, ok, "merged reader MUST report the field when JSON overrides md")
	assert.Equal(t, "json-override", got,
		"JSON value MUST win over md per per-field precedence")
}

// TestGetMergedAgentField_MdOnlyReflectsJSONAfterSet verifies the merge is
// live with respect to SetAgentField: after a model-only SetAgentField on a
// previously md-only agent, GetMergedAgentField reflects the override AND the
// merge layer marks the agent as no longer MdOnly. This is the contract that
// lets the TUI drop the [MD] badge once a JSON override exists.
func TestGetMergedAgentField_MdOnlyReflectsJSONAfterSet(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "review.md",
		"---\nmode: subagent\nmodel: md-original\n---\nBody.\n")

	cfg := &Config{data: map[string]interface{}{}}

	// Initially md-only.
	merged := cfg.MergedAgents()
	require.True(t, merged["review"].MdOnly, "precondition: review starts md-only")

	// SetAgentField writes a model-only JSON override — the per-field merge
	// then treats the agent as JSON-backed for that field while the rest of
	// the agent (mode, body) still comes from md.
	require.NoError(t, cfg.SetAgentField("review", "model", "json-model"))

	got, ok := cfg.GetMergedAgentField("review", "model")
	require.True(t, ok)
	assert.Equal(t, "json-model", got,
		"merged model MUST reflect the model-only JSON override")

	merged = cfg.MergedAgents()
	assert.False(t, merged["review"].MdOnly,
		"once a JSON override exists, the agent MUST NOT be flagged MdOnly")
	assert.True(t, merged["review"].HasInlineJSON,
		"agent MUST be flagged HasInlineJSON once any JSON override exists")
	// Other fields still come from md (per-field merge, not whole-agent replace).
	assert.Equal(t, "subagent", merged["review"].Mode(),
		"non-overridden fields MUST still come from md (per-field merge)")
}

// TestGetMergedAgentField_EmptyIsTreatedAsAbsent verifies that an empty-string
// value is treated as absent by the merged reader (consistent with pickField's
// emptiness rule). This protects the display path from showing "" as a real
// value when a user clears a field via JSON.
func TestGetMergedAgentField_EmptyIsTreatedAsAbsent(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "empty.md",
		"---\nmode: subagent\nmodel: \"\"\n---\nBody.\n")

	cfg := &Config{data: map[string]interface{}{}}

	got, ok := cfg.GetMergedAgentField("empty", "model")
	assert.False(t, ok, "empty-string model MUST be treated as absent")
	assert.Nil(t, got)
}

// TestGetMergedAgentField_UnknownAgentReturnsFalse verifies the reader is
// nil-safe for agent names that exist in neither layer.
func TestGetMergedAgentField_UnknownAgentReturnsFalse(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	cfg := &Config{data: map[string]interface{}{}}

	got, ok := cfg.GetMergedAgentField("nope", "model")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// Model-only persistence and effective-resolution facade (T05)
// ---------------------------------------------------------------------------

func TestModelFacade_GlobalModelSetAndRead(t *testing.T) {
	cfg := &Config{data: map[string]interface{}{"theme": "dark"}}

	cfg.SetGlobalModel("openai/gpt-5")
	model, ok := cfg.GetGlobalModel()

	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", model)
	assert.Equal(t, "dark", cfg.data["theme"], "unrelated top-level JSON must survive")
}

func TestModelFacade_AgentModelOverrideSetAndRead(t *testing.T) {
	cfg := &Config{data: map[string]interface{}{
		"agent": map[string]interface{}{
			"review": map[string]interface{}{"mode": "subagent", "temperature": 0.2},
		},
	}}

	require.NoError(t, cfg.SetAgentModelOverride("review", "anthropic/claude-sonnet-4"))
	model, ok := cfg.GetAgentModelOverride("review")

	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-sonnet-4", model)
	agent := cfg.data["agent"].(map[string]interface{})["review"].(map[string]interface{})
	assert.Equal(t, "subagent", agent["mode"])
	assert.Equal(t, 0.2, agent["temperature"])
}

func TestModelFacade_EffectiveModelPrecedenceAndProvenance(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	writeGlobalMD(t, home, "review.md", "---\nmodel: global-md\n---\nGlobal.\n")

	projectRoot := t.TempDir()
	projectAgents := filepath.Join(projectRoot, ".opencode", "agents")
	require.NoError(t, os.MkdirAll(projectAgents, 0o755))
	projectPath := filepath.Join(projectAgents, "review.md")
	require.NoError(t, os.WriteFile(projectPath, []byte("---\nmodel: project-md\n---\nProject.\n"), 0o644))
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectRoot))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWD)) })

	tests := []struct {
		name       string
		data       map[string]interface{}
		wantModel  string
		wantSource ModelProvenance
	}{
		{
			name: "inline JSON wins",
			data: map[string]interface{}{
				"model": "global-json",
				"agent": map[string]interface{}{"review": map[string]interface{}{"model": "inline-json"}},
			},
			wantModel: "inline-json", wantSource: ModelProvenanceInlineJSON,
		},
		{
			name:      "project markdown wins global markdown",
			data:      map[string]interface{}{"model": "global-json"},
			wantModel: "project-md", wantSource: ModelProvenanceProjectMarkdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{data: tt.data}
			model, source, ok := cfg.ResolveEffectiveModel("review")
			require.True(t, ok)
			assert.Equal(t, tt.wantModel, model)
			assert.Equal(t, tt.wantSource, source)
		})
	}

	require.NoError(t, os.Remove(projectPath))
	cfg := &Config{data: map[string]interface{}{"model": "global-json"}}
	model, source, ok := cfg.ResolveEffectiveModel("review")
	require.True(t, ok)
	assert.Equal(t, "global-md", model)
	assert.Equal(t, ModelProvenanceGlobalMarkdown, source)

	require.NoError(t, os.Remove(filepath.Join(home, ".config", "opencode", "agents", "review.md")))
	model, source, ok = cfg.ResolveEffectiveModel("review")
	require.True(t, ok)
	assert.Equal(t, "global-json", model)
	assert.Equal(t, ModelProvenanceGlobalTopLevel, source)

	delete(cfg.data, "model")
	model, source, ok = cfg.ResolveEffectiveModel("review")
	assert.False(t, ok)
	assert.Empty(t, model)
	assert.Equal(t, ModelProvenanceNone, source)
}

func TestModelFacade_SavePreservesUnrelatedJSONAndMarkdownBytes(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	mdBytes := []byte("---\nmodel: md-model\ntemperature: 0.1\n---\nDo not rewrite.\n")
	mdPath := writeGlobalMD(t, home, "review.md", string(mdBytes))
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	cfg := &Config{path: configPath, data: map[string]interface{}{
		"model": "old-global",
		"mcp":   map[string]interface{}{"server": map[string]interface{}{"url": "https://example.invalid"}},
		"agent": map[string]interface{}{
			"review": map[string]interface{}{"mode": "subagent", "temperature": 0.4},
		},
	}}

	cfg.SetGlobalModel("new-global")
	require.NoError(t, cfg.SetAgentModelOverride("review", "new-agent"))
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, cfg.data["mcp"], reloaded.data["mcp"])
	assert.Equal(t, "subagent", reloaded.data["agent"].(map[string]interface{})["review"].(map[string]interface{})["mode"])
	assert.Equal(t, 0.4, reloaded.data["agent"].(map[string]interface{})["review"].(map[string]interface{})["temperature"])
	gotMD, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Equal(t, mdBytes, gotMD, "model facade must never rewrite markdown")
}
