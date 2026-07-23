// Package test contains integration tests that cross package boundaries and
// verify real-world usage patterns of the opencode-model-selector tool.
//
// These tests were written as the final task (G3-T4) of the SDD pipeline and
// exercise the full stack together:
//   - Config round-trip (load → modify → save → reload): REQ-CFG-011
//   - Model pipeline (parse → group → validate):         REQ-OC-002/003/004
//   - Validation flow:                                    REQ-CFG-012
//   - Backup creation + retention cleanup:                REQ-CFG-009/010
//   - TUI state transitions via public Update()/View():   REQ-TUI-008
//   - Atomic save failure handling:                       REQ-CFG-011
//   - CLI list-agents dispatch via subprocess:            REQ-CMD-003
//
// All implementation packages are already complete; these tests SHOULD pass
// as-is. A failure here reveals a genuine integration bug in the
// implementation, not a test defect.
//
// Testing strategy notes:
//   - TUI tests observe state exclusively through the PUBLIC View() string,
//     since the tui package's appState/cursor/dirty fields are unexported.
//     Each screen renders distinctive markers (titles, section headers) that
//     uniquely identify the active screen.
//   - CLI tests build the real binary as a subprocess to exercise the
//     complete entry point (main → run → dispatch) end-to-end.
package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencode-model-selector/internal/config"
	"opencode-model-selector/internal/opencode"
	"opencode-model-selector/internal/tui"
)

// ---------------------------------------------------------------------------
// Test helpers — fixtures
// ---------------------------------------------------------------------------

// fixtureDir returns the absolute path to the test/fixtures directory.
// Integration tests run with CWD = test/, so "fixtures" resolves correctly.
func fixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("fixtures")
	require.NoError(t, err, "failed to resolve fixtures directory")
	return abs
}

// loadFixtureConfig loads the sanitized opencode.json fixture (14 agents,
// 6 providers worth of MCP servers, a permission block). Each call gets a
// fresh *Config so tests can mutate it independently.
func loadFixtureConfig(t *testing.T) *config.Config {
	t.Helper()
	path := filepath.Join(fixtureDir(t), "opencode.json")
	cfg, err := config.LoadConfig(path)
	require.NoError(t, err, "fixture opencode.json must load without error")
	require.NotNil(t, cfg)
	return cfg
}

// readFixture returns the raw contents of a fixture file as a string.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir(t), name))
	require.NoError(t, err, "fixture %q must exist and be readable", name)
	return string(data)
}

// fixtureGroupedModels parses and groups the models_output.txt fixture into
// the map[string][]opencode.Model shape the TUI constructor expects.
func fixtureGroupedModels(t *testing.T) map[string][]opencode.Model {
	t.Helper()
	models := opencode.ParseModelsOutput(readFixture(t, "models_output.txt"))
	return opencode.GroupByProvider(models)
}

// writeTestConfig writes a minimal valid opencode-style JSON config to path.
// Used by backup tests that need a standalone config file on disk.
func writeTestConfig(t *testing.T, path string) {
	t.Helper()
	content := `{
  "$schema": "https://opencode.ai/config.json",
  "agent": {
    "plan": {
      "mode": "primary"
    }
  },
  "permission": {
    "edit": "ask"
  }
}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// countBackups counts files in dir matching the opencode.json.backup.* glob.
func countBackups(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "opencode.json.backup.") {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Test helpers — TUI screen detection via View()
// ---------------------------------------------------------------------------

// Each TUI screen renders a distinctive title/header that uniquely identifies
// it. These constants match the literal strings produced by the screen's
// view function in internal/tui. Screen detection via View() is the only way
// to observe state from outside the tui package (state/cursor/dirty are
// unexported), and it doubles as an end-to-end rendering check.
const (
	screenMarkerAgentList      = "opencode-model-selector" // viewAgentList title
	screenMarkerAgentDetail    = "Agent: "                  // viewAgentDetail header
	screenMarkerModelSelection = "Select Model"             // viewModelSelection title
	screenMarkerFieldInput     = "Edit: "                   // viewFieldInput header
	screenMarkerSaveConfirm    = "Save Changes?"            // viewSaveConfirm title
)

// onAgentList reports whether the model is currently rendering the Agent List
// screen, detected via the title marker in View().
func onAgentList(m tea.Model) bool {
	return strings.Contains(m.View(), screenMarkerAgentList) &&
		strings.Contains(m.View(), "Primary Agents")
}

// onAgentDetail reports whether the model is on the Agent Detail screen.
func onAgentDetail(m tea.Model) bool {
	return strings.Contains(m.View(), screenMarkerAgentDetail) &&
		strings.Contains(m.View(), "Editable Fields")
}

// onModelSelection reports whether the model is on the Model Selection screen.
func onModelSelection(m tea.Model) bool {
	return strings.Contains(m.View(), screenMarkerModelSelection)
}

// onFieldInput reports whether the model is on the Field Input screen.
func onFieldInput(m tea.Model) bool {
	return strings.Contains(m.View(), screenMarkerFieldInput)
}

// onSaveConfirm reports whether the model is on the Save Confirm screen.
func onSaveConfirm(m tea.Model) bool {
	return strings.Contains(m.View(), screenMarkerSaveConfirm)
}

// isDirty reports whether the dirty indicator ('*') appears in the rendered
// title line. The dirty marker is prepended to the title when m.dirty == true.
func isDirty(m tea.Model) bool {
	view := m.View()
	// The first non-empty line is the title line for every screen.
	for _, line := range strings.Split(view, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.Contains(line, "*")
		}
	}
	return false
}

// pressKey is a convenience wrapper that sends a KeyMsg and returns the
// resulting model cast back to tea.Model.
func pressKey(m tea.Model, key tea.KeyMsg) tea.Model {
	updated, _ := m.Update(key)
	return updated
}

// keyRune constructs a KeyMsg for a single rune (e.g. 'j', 'k', 's').
func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// ---------------------------------------------------------------------------
// 1. Config Round-Trip — REQ-CFG-011
// ---------------------------------------------------------------------------

// TestIntegration_ConfigRoundTrip_PreservesAllFieldsAndChanges verifies that
// a config loaded from the fixture, modified with three distinct changes,
// saved atomically, and reloaded preserves BOTH the new changes AND every
// original field that the tool does not explicitly model.
//
// GIVEN config loaded from fixture, modified (3 changes), saved, reloaded,
// THEN all 3 changes present AND all original fields preserved.
//
// Spec: REQ-CFG-011 — Scenario: round-trip preserves unknown fields.
func TestIntegration_ConfigRoundTrip_PreservesAllFieldsAndChanges(t *testing.T) {
	original := loadFixtureConfig(t)

	// Apply 3 distinct changes across agent fields and the global model.
	require.NoError(t, original.SetAgentField("code-reviewer", "model", "opencode-go/glm-5.2"))
	require.NoError(t, original.SetAgentField("plan", "temperature", 0.4))
	original.SetGlobalModel("zai-coding-plan/glm-5.2")

	// Capture the top-level key set AFTER modifications (SetGlobalModel adds
	// the "model" key). The round-trip must preserve this exact set.
	modifiedKeys := make([]string, 0, len(original.Data()))
	for k := range original.Data() {
		modifiedKeys = append(modifiedKeys, k)
	}
	sort.Strings(modifiedKeys)

	// Round-trip through a temp file: marshal the modified data, write it,
	// and reload. This exercises the same JSON encoding path that Config.Save()
	// uses internally (json.MarshalIndent with 2-space indent).
	tmpPath := filepath.Join(t.TempDir(), "opencode.json")
	data, err := json.MarshalIndent(original.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	reloaded, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	// --- Change 1: code-reviewer model updated ---
	val, ok := reloaded.GetAgentField("code-reviewer", "model")
	require.True(t, ok, "code-reviewer model field must exist after reload")
	assert.Equal(t, "opencode-go/glm-5.2", val,
		"change 1 (code-reviewer model) must survive round-trip")

	// --- Change 2: plan temperature updated ---
	tempVal, ok := reloaded.GetAgentField("plan", "temperature")
	require.True(t, ok, "plan temperature field must exist after reload")
	assert.Equal(t, float64(0.4), tempVal,
		"change 2 (plan temperature) must survive round-trip")

	// --- Change 3: global model updated ---
	got, ok := reloaded.GetGlobalModel()
	require.True(t, ok, "global model must be set after reload")
	assert.Equal(t, "zai-coding-plan/glm-5.2", got,
		"change 3 (global model) must survive round-trip")

	// --- All top-level keys preserved (including the new "model" key) ---
	reloadedKeys := make([]string, 0, len(reloaded.Data()))
	for k := range reloaded.Data() {
		reloadedKeys = append(reloadedKeys, k)
	}
	sort.Strings(reloadedKeys)
	assert.Equal(t, modifiedKeys, reloadedKeys,
		"all top-level keys (including the new 'model' key) must be preserved after round-trip")
}

// TestIntegration_ConfigRoundTrip_PreservesAPIKeysByteForByte verifies that
// MCP server API keys survive the load → save → reload cycle exactly.
//
// GIVEN config with MCP API keys, WHEN round-tripped,
// THEN API keys preserved byte-for-byte.
//
// Spec: REQ-CFG-011 — Scenario: sensitive fields preserved.
func TestIntegration_ConfigRoundTrip_PreservesAPIKeysByteForByte(t *testing.T) {
	original := loadFixtureConfig(t)

	tmpPath := filepath.Join(t.TempDir(), "opencode.json")
	data, err := json.MarshalIndent(original.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	reloaded, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	mcp, ok := reloaded.Data()["mcp"].(map[string]interface{})
	require.True(t, ok, "mcp section must survive round-trip")

	zai, ok := mcp["zai-coding-plan"].(map[string]interface{})
	require.True(t, ok, "zai-coding-plan server must survive round-trip")
	zaiEnv, ok := zai["env"].(map[string]interface{})
	require.True(t, ok, "zai-coding-plan env must survive round-trip")
	assert.Equal(t, "fake-key-aaaa-bbbb", zaiEnv["Z_AI_API_KEY"],
		"Z_AI_API_KEY must survive round-trip byte-for-byte")

	ctx7, ok := mcp["context7"].(map[string]interface{})
	require.True(t, ok, "context7 server must survive round-trip")
	ctxEnv, ok := ctx7["env"].(map[string]interface{})
	require.True(t, ok, "context7 env must survive round-trip")
	assert.Equal(t, "fake-key-cccc-dddd", ctxEnv["CONTEXT7_API_KEY"],
		"CONTEXT7_API_KEY must survive round-trip byte-for-byte")
}

// TestIntegration_ConfigRoundTrip_PreservesAll14Agents verifies that all 14
// agents from the fixture survive the round-trip with all their fields.
//
// GIVEN config with 14 agents, WHEN round-tripped,
// THEN all 14 agents present with all their fields.
//
// Spec: REQ-CFG-011 — Scenario: agent section preserved.
func TestIntegration_ConfigRoundTrip_PreservesAll14Agents(t *testing.T) {
	original := loadFixtureConfig(t)
	originalAgents := original.Data()["agent"].(map[string]interface{})

	tmpPath := filepath.Join(t.TempDir(), "opencode.json")
	data, err := json.MarshalIndent(original.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	reloaded, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	reloadedAgents := reloaded.Data()["agent"].(map[string]interface{})

	assert.Equal(t, len(originalAgents), len(reloadedAgents),
		"agent count must match after round-trip")
	assert.Equal(t, 14, len(reloadedAgents),
		"fixture must round-trip with exactly 14 agents")

	// Every original agent name must appear in the reloaded config.
	for name := range originalAgents {
		_, exists := reloadedAgents[name]
		assert.True(t, exists, "agent %q must exist after round-trip", name)
	}

	// The hidden and disable flags on known agents survive.
	parallelDispatch, ok := reloadedAgents["parallel-dispatch"].(map[string]interface{})
	require.True(t, ok, "parallel-dispatch must survive round-trip")
	assert.Equal(t, true, parallelDispatch["hidden"],
		"parallel-dispatch hidden flag must survive round-trip")

	build, ok := reloadedAgents["build"].(map[string]interface{})
	require.True(t, ok, "build must survive round-trip")
	assert.Equal(t, true, build["disable"],
		"build disable flag must survive round-trip")
}

// TestIntegration_ConfigRoundTrip_PreservesPermissionSection verifies that the
// permission section (edit/bash/webfetch) survives the round-trip intact.
//
// Spec: REQ-CFG-011 — Scenario: permission section preserved.
func TestIntegration_ConfigRoundTrip_PreservesPermissionSection(t *testing.T) {
	original := loadFixtureConfig(t)

	tmpPath := filepath.Join(t.TempDir(), "opencode.json")
	data, err := json.MarshalIndent(original.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	reloaded, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	perm, ok := reloaded.Data()["permission"].(map[string]interface{})
	require.True(t, ok, "permission section must survive round-trip")
	assert.Equal(t, "ask", perm["edit"], "permission.edit must survive round-trip")
	assert.Equal(t, "ask", perm["bash"], "permission.bash must survive round-trip")
	assert.Equal(t, "allow", perm["webfetch"], "permission.webfetch must survive round-trip")
}

// TestIntegration_ConfigRoundTrip_ViaConfigSaveMethod verifies the real
// Config.Save() round-trip path (not just json.MarshalIndent). This catches
// bugs in the atomic write implementation that the marshal-only test above
// would miss.
//
// Spec: REQ-CFG-011 — Scenario: Config.Save() round-trip.
func TestIntegration_ConfigRoundTrip_ViaConfigSaveMethod(t *testing.T) {
	src := loadFixtureConfig(t)

	// We need a Config whose .path points at a writable temp location so
	// Save() actually writes. Round-trip: write fixture data to temp, load
	// it, modify, Save(), reload.
	tmpPath := filepath.Join(t.TempDir(), "opencode.json")
	data, err := json.MarshalIndent(src.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o600))

	cfg, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	// Modify and save via the real atomic Save() path.
	require.NoError(t, cfg.SetAgentField("code-reviewer", "model", "openai/gpt-5.5"))
	cfg.SetGlobalModel("zai-coding-plan/glm-5.2")
	require.NoError(t, cfg.Save())

	// Reload and verify.
	reloaded, err := config.LoadConfig(tmpPath)
	require.NoError(t, err)

	val, ok := reloaded.GetAgentField("code-reviewer", "model")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5.5", val,
		"Config.Save() must persist agent field changes")

	got, ok := reloaded.GetGlobalModel()
	require.True(t, ok)
	assert.Equal(t, "zai-coding-plan/glm-5.2", got,
		"Config.Save() must persist global model changes")

	// No leftover temp file after a successful atomic save.
	_, statErr := os.Stat(tmpPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr),
		"no .tmp file must remain after a successful atomic Save")
}

// ---------------------------------------------------------------------------
// 2. Model Pipeline — REQ-OC-002/003/004
// ---------------------------------------------------------------------------

// TestIntegration_ModelPipeline_ParseAndGroup verifies the full pipeline from
// raw opencode CLI output through parsing and provider grouping.
//
// GIVEN models_output.txt fixture, WHEN parsed and grouped,
// THEN 53 models across 6 providers with correct per-provider counts.
//
// Spec: REQ-OC-003 (parse), REQ-OC-004 (group).
func TestIntegration_ModelPipeline_ParseAndGroup(t *testing.T) {
	output := readFixture(t, "models_output.txt")

	// Parse raw output into Model structs.
	models := opencode.ParseModelsOutput(output)
	require.Len(t, models, 53,
		"fixture must parse to exactly 53 models")

	// Group by provider.
	grouped := opencode.GroupByProvider(models)
	require.Len(t, grouped, 6,
		"fixture must group into exactly 6 providers")

	// Each provider has the correct model count.
	expectedCounts := map[string]int{
		"opencode":              6,
		"opencode-go":           15,
		"minimax":               7,
		"openai":                13,
		"xiaomi-token-plan-sgp": 6,
		"zai-coding-plan":       6,
	}
	for provider, count := range expectedCounts {
		assert.Contains(t, grouped, provider,
			"provider %q must be in the grouped map", provider)
		assert.Len(t, grouped[provider], count,
			"provider %q must contain %d models", provider, count)
	}

	// Total across all providers must equal 53 — no models lost.
	total := 0
	for _, providerModels := range grouped {
		total += len(providerModels)
	}
	assert.Equal(t, 53, total,
		"total models across all providers must equal 53")
}

// TestIntegration_ModelPipeline_ModelsSortedByProvider verifies that within
// each provider, models are sorted alphabetically by ID.
//
// GIVEN parsed models, THEN each provider's models are sorted alphabetically
// by ID.
//
// Spec: REQ-OC-004 — Scenario: models sorted alphabetically within provider.
func TestIntegration_ModelPipeline_ModelsSortedByProvider(t *testing.T) {
	grouped := fixtureGroupedModels(t)

	for provider, providerModels := range grouped {
		for i := 1; i < len(providerModels); i++ {
			assert.True(t,
				providerModels[i-1].ID <= providerModels[i].ID,
				"provider %q: model %q (idx %d) must be <= model %q (idx %d) by ID",
				provider,
				providerModels[i-1].ID, i-1,
				providerModels[i].ID, i)
		}
	}
}

// TestIntegration_ModelPipeline_FullNameReconstruction verifies that
// ParseModelsOutput reconstructs the FullName correctly from raw output lines.
//
// Spec: REQ-OC-003 — Scenario: FullName = provider + "/" + ID.
func TestIntegration_ModelPipeline_FullNameReconstruction(t *testing.T) {
	models := opencode.ParseModelsOutput(readFixture(t, "models_output.txt"))

	for _, m := range models {
		expected := m.Provider + "/" + m.ID
		assert.Equal(t, expected, m.FullName,
			"FullName must equal Provider/ID for model %q", m.FullName)
	}
}

// ---------------------------------------------------------------------------
// 3. Validation Flow — REQ-CFG-012
// ---------------------------------------------------------------------------

// TestIntegration_ValidationFlow_ValidAndInvalid verifies that ValidateModel
// accepts a model that exists in the parsed list and rejects one that doesn't.
//
// Spec: REQ-CFG-012 — Scenario: valid model returns true, invalid returns false.
func TestIntegration_ValidationFlow_ValidAndInvalid(t *testing.T) {
	models := opencode.ParseModelsOutput(readFixture(t, "models_output.txt"))

	// Valid models from each provider.
	validIDs := []string{
		"opencode-go/glm-5.2",
		"openai/gpt-5.5",
		"minimax/MiniMax-M3",
		"zai-coding-plan/glm-5.2",
		"xiaomi-token-plan-sgp/mimo-v2.5",
		"opencode/north-mini-code-free",
	}
	for _, id := range validIDs {
		assert.True(t, config.ValidateModel(id, models),
			"ValidateModel must return true for existing model %q", id)
	}

	// Invalid models.
	invalidIDs := []string{
		"fake/nonexistent",
		"opencode-go/nonexistent-model",
		"glm-5.2", // missing provider prefix
		"",        // empty
	}
	for _, id := range invalidIDs {
		assert.False(t, config.ValidateModel(id, models),
			"ValidateModel must return false for non-existent model %q", id)
	}
}

// TestIntegration_ValidationFlow_CaseSensitive verifies that ValidateModel
// performs exact, case-sensitive matching.
//
// Spec: REQ-CFG-012 — Scenario: case-sensitive exact match.
func TestIntegration_ValidationFlow_CaseSensitive(t *testing.T) {
	models := opencode.ParseModelsOutput(readFixture(t, "models_output.txt"))

	// Exact case must pass; mixed case must fail.
	assert.True(t, config.ValidateModel("minimax/MiniMax-M3", models),
		"exact-case match for MiniMax-M3 must pass")
	assert.False(t, config.ValidateModel("MINIMAX/minimax-m3", models),
		"uppercased match must fail (case-sensitive)")
	assert.False(t, config.ValidateModel("Minimax/MiniMax-M3", models),
		"mixed-case provider must fail (case-sensitive)")
}

// ---------------------------------------------------------------------------
// 4. Backup Flow — REQ-CFG-009/010
// ---------------------------------------------------------------------------

// TestIntegration_BackupFlow_CreateAndClean verifies the full backup lifecycle:
// create a backup, create more than retention, then clean down to retention.
//
// Spec: REQ-CFG-009 (CreateBackup), REQ-CFG-010 (CleanOldBackups).
func TestIntegration_BackupFlow_CreateAndClean(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeTestConfig(t, configPath)

	// Initial backup must succeed and exist on disk.
	backupPath, err := config.CreateBackup(configPath)
	require.NoError(t, err)
	assert.FileExists(t, backupPath, "first backup must exist on disk")

	// Create additional backups with distinct timestamps so the lexicographic
	// sort used by CleanOldBackups can tell them apart. We sleep briefly
	// between each to guarantee the YYYYMMDD-HHMMSS timestamps differ.
	for i := 0; i < 6; i++ {
		time.Sleep(1100 * time.Millisecond)
		_, err := config.CreateBackup(configPath)
		require.NoError(t, err)
	}
	// Total: 1 (initial) + 6 = 7 backups.
	assert.Equal(t, 7, countBackups(t, dir),
		"must have 7 backups before cleanup")

	// Clean down to retention 5.
	require.NoError(t, config.CleanOldBackups(configPath, 5))
	assert.Equal(t, 5, countBackups(t, dir),
		"must retain exactly 5 backups after CleanOldBackups(5)")

	// The source config file itself must be untouched (still on disk).
	assert.FileExists(t, configPath, "source config must remain after backup cleanup")
}

// TestIntegration_BackupFlow_ContentIdenticalToSource verifies that a backup
// is a byte-for-byte copy of the source at backup time.
//
// Spec: REQ-CFG-009 — Scenario: backup is byte-for-byte copy.
func TestIntegration_BackupFlow_ContentIdenticalToSource(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeTestConfig(t, configPath)

	backupPath, err := config.CreateBackup(configPath)
	require.NoError(t, err)

	src, err := os.ReadFile(configPath)
	require.NoError(t, err)
	bak, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, src, bak,
		"backup content must be byte-for-byte identical to source")
}

// TestIntegration_BackupFlow_CleanToZeroKeepsAll verifies that retention 0
// means "skip cleanup entirely" — no backups are deleted.
//
// Spec: REQ-CFG-010 — Scenario: keep=0 preserves all.
func TestIntegration_BackupFlow_CleanToZeroKeepsAll(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeTestConfig(t, configPath)

	// Create 3 backups with distinct timestamps.
	for i := 0; i < 3; i++ {
		time.Sleep(1100 * time.Millisecond)
		_, err := config.CreateBackup(configPath)
		require.NoError(t, err)
	}
	require.Equal(t, 3, countBackups(t, dir))

	require.NoError(t, config.CleanOldBackups(configPath, 0))
	assert.Equal(t, 3, countBackups(t, dir),
		"keep=0 must NOT delete any backups")
}

// TestIntegration_BackupFlow_FullConfigBackupPreservesStructure verifies that
// backing up the REAL fixture config (not a minimal stub) preserves all 14
// agents, the MCP section, and the permission section.
//
// Spec: REQ-CFG-009 — Scenario: backup preserves full config structure.
func TestIntegration_BackupFlow_FullConfigBackupPreservesStructure(t *testing.T) {
	src := loadFixtureConfig(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	data, err := json.MarshalIndent(src.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	backupPath, err := config.CreateBackup(configPath)
	require.NoError(t, err)

	bakCfg, err := config.LoadConfig(backupPath)
	require.NoError(t, err)

	bakAgents := bakCfg.Data()["agent"].(map[string]interface{})
	assert.Len(t, bakAgents, 14, "backup must preserve all 14 agents")

	bakMcp := bakCfg.Data()["mcp"].(map[string]interface{})
	assert.Contains(t, bakMcp, "zai-coding-plan", "backup must preserve zai-coding-plan MCP server")
	assert.Contains(t, bakMcp, "context7", "backup must preserve context7 MCP server")

	bakPerm := bakCfg.Data()["permission"].(map[string]interface{})
	assert.Equal(t, "ask", bakPerm["edit"], "backup must preserve permission.edit")
}

// ---------------------------------------------------------------------------
// 5. TUI State Transitions — REQ-TUI-008
//
// All TUI tests observe state through the public View() string, since the tui
// package's internal state fields are unexported. Each screen renders a
// distinctive title/header, so string inspection reliably identifies the
// active screen.
// ---------------------------------------------------------------------------

// newTUI constructs a TUI model seeded with the fixture config and grouped
// models. All TUI integration tests start from this baseline.
func newTUI(t *testing.T) tea.Model {
	t.Helper()
	cfg := loadFixtureConfig(t)
	grouped := fixtureGroupedModels(t)
	// Return as tea.Model so callers interact only with the public API.
	return tui.NewModel(cfg, grouped, 5)
}

// TestIntegration_TUI_InitialScreenIsAgentList verifies that a freshly
// constructed TUI renders the Agent List screen.
//
// Spec: REQ-TUI-001 — initial screen is AgentList.
func TestIntegration_TUI_InitialScreenIsAgentList(t *testing.T) {
	m := newTUI(t)
	assert.True(t, onAgentList(m),
		"freshly constructed TUI must render the Agent List screen")
}

// TestIntegration_TUI_AgentListToAgentDetail verifies that ENTER on a
// selectable agent transitions from AgentList to AgentDetail.
//
// Spec: REQ-TUI-008 — Scenario: ENTER transitions to AgentDetail.
func TestIntegration_TUI_AgentListToAgentDetail(t *testing.T) {
	m := newTUI(t)
	require.True(t, onAgentList(m), "precondition: must start on AgentList")

	// Move cursor down once to land on the first selectable agent.
	// selectableItems = [__global__, plan, code-reviewer, debug, docs,
	//   explore, general, orchestrator, parallel-dispatch,
	//   security-auditor, team-lead] — build is disabled so skipped.
	m = pressKey(m, keyRune('j'))

	// ENTER on an agent transitions to AgentDetail.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, onAgentDetail(m),
		"ENTER on an agent must transition to AgentDetail screen")
}

// TestIntegration_TUI_AgentListToGlobalModelSelection verifies that ENTER on
// the Global Default Model entry transitions to ModelSelection.
//
// Spec: REQ-TUI-008 — Scenario: ENTER on global opens model picker.
func TestIntegration_TUI_AgentListToGlobalModelSelection(t *testing.T) {
	m := newTUI(t)
	require.True(t, onAgentList(m))

	// Cursor starts at 0 = __global__. ENTER opens ModelSelection.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, onModelSelection(m),
		"ENTER on global must transition to ModelSelection screen")
}

// TestIntegration_TUI_AgentDetailToModelSelection verifies the navigation
// from AgentDetail → ModelSelection when ENTER is pressed on the model field.
//
// Spec: REQ-TUI-008 — Scenario: ENTER on model field opens model picker.
func TestIntegration_TUI_AgentDetailToModelSelection(t *testing.T) {
	m := newTUI(t)

	// Navigate: AgentList → (j) → AgentList cursor on agent → ENTER → AgentDetail.
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onAgentDetail(m), "precondition: must be on AgentDetail")

	// selectedField starts at 0 = "model". ENTER opens ModelSelection.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, onModelSelection(m),
		"ENTER on model field must transition to ModelSelection")
}

// TestIntegration_TUI_AgentDetailToFieldInput verifies the navigation from
// AgentDetail → FieldInput when ENTER is pressed on a non-model text field
// (temperature, top_p, color, steps).
//
// Spec: REQ-TUI-008 — Scenario: ENTER on text field opens field input.
func TestIntegration_TUI_AgentDetailToFieldInput(t *testing.T) {
	m := newTUI(t)

	// Navigate to AgentDetail.
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onAgentDetail(m))

	// Move selectedField down to "temperature" (index 1) and ENTER.
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, onFieldInput(m),
		"ENTER on temperature field must transition to FieldInput")
}

// TestIntegration_TUI_ModelSelectionBackViaESC verifies that ESC from
// ModelSelection returns to the previous screen.
//
// Spec: REQ-TUI-008 — Scenario: ESC returns to previous screen.
func TestIntegration_TUI_ModelSelectionBackViaESC(t *testing.T) {
	m := newTUI(t)
	// Enter AgentDetail then ModelSelection.
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onModelSelection(m), "precondition: must be on ModelSelection")

	// ESC returns to AgentDetail.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, onAgentDetail(m),
		"ESC from ModelSelection must return to AgentDetail")
}

// TestIntegration_TUI_AgentDetailBackViaESC verifies that ESC from
// AgentDetail returns to AgentList.
//
// Spec: REQ-TUI-008 — Scenario: ESC pops the navigation stack.
func TestIntegration_TUI_AgentDetailBackViaESC(t *testing.T) {
	m := newTUI(t)
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onAgentDetail(m))

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, onAgentList(m),
		"ESC from AgentDetail must return to AgentList")
}

// TestIntegration_TUI_SaveTransitionRequiresDirty verifies that 's' only
// transitions to SaveConfirm when there are unsaved changes. Because dirty
// is not settable from outside the package, we first make a real edit (which
// sets dirty=true internally) and then press 's'.
//
// Spec: REQ-TUI-008 + REQ-TUI-007 — 's' with dirty=true goes to SaveConfirm.
func TestIntegration_TUI_SaveTransitionRequiresDirty(t *testing.T) {
	t.Run("NotDirty_StaysOnAgentList", func(t *testing.T) {
		m := newTUI(t)
		require.False(t, isDirty(m), "precondition: fresh model must not be dirty")

		m = pressKey(m, keyRune('s'))
		assert.True(t, onAgentList(m),
			"'s' when NOT dirty must stay on AgentList")
	})

	t.Run("Dirty_TransitionsToSaveConfirm", func(t *testing.T) {
		m := newTUI(t)

		// Make a real edit to set dirty=true: navigate to global, open model
		// picker, select the first model. selectModelAtCursor sets dirty=true.
		m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // global → ModelSelection
		require.True(t, onModelSelection(m))
		m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // select first model → back to AgentList
		require.True(t, onAgentList(m), "after model select, must return to AgentList")
		require.True(t, isDirty(m), "selecting a model must mark the model dirty")

		// Now 's' must transition to SaveConfirm.
		m = pressKey(m, keyRune('s'))
		assert.True(t, onSaveConfirm(m),
			"'s' when dirty must transition to SaveConfirm")
	})
}

// TestIntegration_TUI_CtrlCAlwaysQuits verifies that Ctrl+C produces a
// tea.Quit command from every reachable screen.
//
// Spec: REQ-TUI-003 — Ctrl+C always quits.
func TestIntegration_TUI_CtrlCAlwaysQuits(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m tea.Model) tea.Model
	}{
		{
			name:  "from AgentList",
			setup: func(t *testing.T, m tea.Model) tea.Model { return m },
		},
		{
			name: "from AgentDetail",
			setup: func(t *testing.T, m tea.Model) tea.Model {
				m = pressKey(m, keyRune('j'))
				return pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "from ModelSelection",
			setup: func(t *testing.T, m tea.Model) tea.Model {
				m = pressKey(m, keyRune('j'))
				m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
				return pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "from FieldInput",
			setup: func(t *testing.T, m tea.Model) tea.Model {
				m = pressKey(m, keyRune('j'))
				m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
				m = pressKey(m, keyRune('j')) // cursor to temperature
				return pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTUI(t)
			m = tt.setup(t, m)

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			require.NotNil(t, cmd, "Ctrl+C must produce a non-nil command")
			assert.IsType(t, tea.QuitMsg{}, cmd(),
				"Ctrl+C must produce a tea.QuitMsg")
		})
	}
}

// TestIntegration_TUI_ModelSelectionUpdatesGlobalModel verifies the full
// happy-path edit: select the global model via the picker, return to
// AgentList, and verify the rendered global model value changed.
//
// Spec: REQ-TUI-005 + REQ-CFG-004 — model selection updates global model.
func TestIntegration_TUI_ModelSelectionUpdatesGlobalModel(t *testing.T) {
	m := newTUI(t)

	// Before: the global model row shows "(none)" because the fixture has no
	// top-level "model" key. The global row renders as:
	//   [Global Default Model]
	//     model: (none)
	beforeView := m.View()
	globalIdx := strings.Index(beforeView, "[Global Default Model]")
	require.GreaterOrEqual(t, globalIdx, 0,
		"precondition: global model row must exist in View")
	globalSection := beforeView[globalIdx:]
	assert.Contains(t, globalSection, "(none)",
		"precondition: fixture has no global model, so View must show (none)")

	// Navigate: ENTER on global → ModelSelection → ENTER selects first model.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onModelSelection(m))

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onAgentList(m), "after selecting, must return to AgentList")
	require.True(t, isDirty(m), "model must be dirty after an edit")

	// After: the global model row must now show a real provider/id value.
	// The global row is exactly two lines:
	//   [Global Default Model]
	//     model: <value>
	// followed by the "Primary Agents" section header. Extract only the
	// global block to avoid false matches from other agents' "(none)" rows.
	afterView := m.View()
	globalIdx = strings.Index(afterView, "[Global Default Model]")
	require.GreaterOrEqual(t, globalIdx, 0)

	// Find the end of the global block (the next section header).
	primaryIdx := strings.Index(afterView[globalIdx:], "Primary Agents")
	require.GreaterOrEqual(t, primaryIdx, 0)
	globalBlock := afterView[globalIdx : globalIdx+primaryIdx]

	assert.NotContains(t, globalBlock, "(none)",
		"after selecting a global model, the global row must NOT show (none): %q", globalBlock)
	assert.Contains(t, globalBlock, "model: ",
		"global model row must show the 'model:' label")

	// The value after "model: " must contain "/" (provider/id format).
	modelLineIdx := strings.Index(globalBlock, "model: ")
	require.GreaterOrEqual(t, modelLineIdx, 0)
	modelValue := strings.TrimSpace(globalBlock[modelLineIdx+len("model: "):])
	// Trim to first newline in case of trailing content.
	if newlineIdx := strings.Index(modelValue, "\n"); newlineIdx >= 0 {
		modelValue = modelValue[:newlineIdx]
	}
	assert.Contains(t, modelValue, "/",
		"global model value must be in provider/id format (got %q)", modelValue)
}

// TestIntegration_TUI_DisabledAgentNotSelectable verifies that the disabled
// agent (build) cannot be navigated to. Pressing 'j' from the global entry
// must skip 'build' and land on 'plan'.
//
// Spec: REQ-TUI-002 + REQ-TUI-003 — disabled agents are non-selectable.
func TestIntegration_TUI_DisabledAgentNotSelectable(t *testing.T) {
	m := newTUI(t)

	// Press 'j' once from global (cursor 0). The cursor must land on 'plan',
	// NOT on the disabled 'build'. We verify by pressing ENTER and checking
	// the AgentDetail header shows 'plan'.
	m = pressKey(m, keyRune('j'))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, onAgentDetail(m))

	view := m.View()
	assert.Contains(t, view, "Agent: plan",
		"first 'j' from global must land on 'plan' (build is disabled and skipped)")
	assert.NotContains(t, view, "Agent: build",
		"disabled agent 'build' must not be reachable via cursor navigation")
}

// ---------------------------------------------------------------------------
// 6. Atomic Save Failure — REQ-CFG-011
// ---------------------------------------------------------------------------

// TestIntegration_AtomicSaveFailure_UnwritablePath verifies that Save() to an
// unwritable location returns an error and leaves the in-memory data intact.
//
// Spec: REQ-CFG-011 — Scenario: atomic save fails without corrupting state.
func TestIntegration_AtomicSaveFailure_UnwritablePath(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "opencode.json")
	src := loadFixtureConfig(t)
	data, err := json.MarshalIndent(src.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(goodPath, data, 0o600))

	cfg, err := config.LoadConfig(goodPath)
	require.NoError(t, err)

	// Snapshot the in-memory data so we can verify it is unchanged on failure.
	beforeJSON, err := json.Marshal(cfg.Data())
	require.NoError(t, err)

	// Remove the parent directory to make the atomic rename target invalid.
	// Config.Save() writes to goodPath + ".tmp" then renames; both fail when
	// the directory is gone.
	require.NoError(t, os.RemoveAll(dir))

	err = cfg.Save()
	require.Error(t, err, "Save to a deleted directory must fail")

	// In-memory data must be unchanged.
	afterJSON, err := json.Marshal(cfg.Data())
	require.NoError(t, err)
	assert.JSONEq(t, string(beforeJSON), string(afterJSON),
		"in-memory config data must be unchanged after a failed Save")
}

// TestIntegration_AtomicSaveFailure_NoLeftoverTempFile verifies that when
// Config.Save() fails, no leftover .tmp file remains at the target path.
//
// Spec: REQ-CFG-011 — Scenario: temp file cleaned up on rename failure.
func TestIntegration_AtomicSaveFailure_NoLeftoverTempFile(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "opencode.json")
	src := loadFixtureConfig(t)
	data, err := json.MarshalIndent(src.Data(), "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(goodPath, data, 0o600))

	cfg, err := config.LoadConfig(goodPath)
	require.NoError(t, err)
	cfg.SetGlobalModel("zai-coding-plan/glm-5.2")

	// Remove the directory to force both temp-write and rename to fail.
	require.NoError(t, os.RemoveAll(dir))

	err = cfg.Save()
	require.Error(t, err)

	_, statErr := os.Stat(goodPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr),
		"no leftover .tmp file must remain after a failed atomic Save")
}

// ---------------------------------------------------------------------------
// 7. CLI Flow — REQ-CMD-003 (subprocess)
// ---------------------------------------------------------------------------

// TestIntegration_CLI_ListAgentsProducesExpectedOutput runs the CLI binary's
// list-agents flow against the fixture config and verifies the output contains
// the expected agent names and field values.
//
// We invoke the built binary as a subprocess to exercise the real CLI path
// end-to-end, including flag parsing, config loading, and formatting.
//
// Spec: REQ-CMD-003 — Scenario: list-agents output format.
func TestIntegration_CLI_ListAgentsProducesExpectedOutput(t *testing.T) {
	binary := buildBinary(t)
	configPath := filepath.Join(fixtureDir(t), "opencode.json")

	output := runBinary(t, binary, "--config", configPath, "--list-agents")

	// Header and global default model line.
	assert.Contains(t, output, "OpenCode Agents",
		"output must contain the header")
	assert.Contains(t, output, "Global Default Model:",
		"output must contain the global default model line")

	// Section headers.
	assert.Contains(t, output, "Primary Agents",
		"output must contain the Primary Agents section header")
	assert.Contains(t, output, "Subagents",
		"output must contain the Subagents section header")

	// Non-system agents must appear.
	for _, name := range []string{
		"plan", "build", "code-reviewer", "debug", "docs", "explore",
		"general", "orchestrator", "parallel-dispatch", "security-auditor",
		"team-lead",
	} {
		assert.Contains(t, output, name,
			"non-system agent %q must appear in list-agents output", name)
	}

	// System agents must NOT appear.
	for _, name := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, output, name,
			"system agent %q must NOT appear in list-agents output", name)
	}

	// Field labels for all 6 editable fields.
	for _, field := range []string{"model:", "temperature:", "top_p:", "color:", "steps:", "disable:"} {
		assert.Contains(t, output, field,
			"field label %q must appear for every agent", field)
	}

	// Known model value on code-reviewer.
	assert.Contains(t, output, "anthropic/claude-sonnet-4-20250514",
		"code-reviewer's model must appear")

	// Known temperature value on plan.
	assert.Contains(t, output, "0.4",
		"plan's temperature (0.4) must appear")

	// Markers for disabled and hidden agents.
	assert.Contains(t, output, "[DISABLED]",
		"disabled agent (build) must carry [DISABLED]")
	assert.Contains(t, output, "parallel-dispatch [H]",
		"hidden agent (parallel-dispatch) must carry [H]")
}

// TestIntegration_CLI_ListAgentsMissingConfigReturnsNonZero verifies the CLI
// exits non-zero and prints an error when the config is missing.
//
// Spec: REQ-CMD-004 — Scenario: config not found.
func TestIntegration_CLI_ListAgentsMissingConfigReturnsNonZero(t *testing.T) {
	binary := buildBinary(t)
	_, stderr, exitCode := runBinaryFull(t, binary,
		"--config", "/nonexistent/path/opencode.json", "--list-agents")

	assert.NotEqual(t, 0, exitCode,
		"CLI must exit non-zero when config is missing")
	assert.Contains(t, stderr, "Config not found",
		"stderr must contain 'Config not found' for missing config")
}

// TestIntegration_CLI_InvalidFlagReturnsExit2 verifies that an invalid flag
// produces exit code 2 (standard usage-error convention).
//
// Spec: REQ-CMD-001 — Scenario: invalid flag → exit 2.
func TestIntegration_CLI_InvalidFlagReturnsExit2(t *testing.T) {
	binary := buildBinary(t)
	_, _, exitCode := runBinaryFull(t, binary, "--not-a-real-flag")

	assert.Equal(t, 2, exitCode,
		"invalid flag must produce exit code 2")
}

// TestIntegration_CLI_BackupCountFlagAccepted verifies that the --backup-count
// flag is accepted (does not cause a usage error) alongside --list-agents.
//
// Spec: REQ-CMD-001 — backup-count flag parsing.
func TestIntegration_CLI_BackupCountFlagAccepted(t *testing.T) {
	binary := buildBinary(t)
	configPath := filepath.Join(fixtureDir(t), "opencode.json")

	output, stderr, exitCode := runBinaryFull(t, binary,
		"--config", configPath, "--list-agents", "--backup-count", "3")

	require.Equal(t, 0, exitCode,
		"valid invocation with --backup-count must succeed, stderr: %q", stderr)
	assert.Contains(t, output, "OpenCode Agents",
		"output must still contain the header with --backup-count present")
}

// ---------------------------------------------------------------------------
// CLI subprocess helpers
// ---------------------------------------------------------------------------

// binaryExt returns the platform-appropriate executable extension.
func binaryExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// projectRoot returns the absolute path to the opencode-model-selector module
// root (where go.mod lives). Integration tests run from test/, so we go up
// one level.
func projectRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("..")
	require.NoError(t, err)
	return abs
}

// buildBinary compiles the cmd/main.go entrypoint to a temp executable and
// returns its path. The binary is cleaned up automatically via t.TempDir().
func buildBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "opencode-model-selector-test"+binaryExt())

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err,
		"go build ./cmd failed: %s", string(out))
	require.FileExists(t, binaryPath, "built binary must exist")
	return binaryPath
}

// runBinary runs the binary with the given args and returns stdout. It fails
// the test if the exit code is non-zero OR if stderr is non-empty (since
// list-agents on a valid config should produce no stderr).
func runBinary(t *testing.T, binary string, args ...string) string {
	t.Helper()
	stdout, stderr, exitCode := runBinaryFull(t, binary, args...)
	require.Zero(t, exitCode,
		"unexpected non-zero exit code %d; stderr: %q", exitCode, stderr)
	require.Empty(t, stderr,
		"unexpected stderr output: %q", stderr)
	return stdout
}

// runBinaryFull runs the binary with the given args and returns stdout,
// stderr, and the exit code. Does not fail the test on non-zero exit — the
// caller decides based on expectations.
func runBinaryFull(t *testing.T, binary string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = projectRoot(t)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		// On a non-zero exit, the error embeds the exit code via ExitError.
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Failed to start or other I/O error — surface as a test failure.
			t.Fatalf("failed to run binary %q: %v (stdout=%q stderr=%q)",
				binary, err, stdoutBuf.String(), stderrBuf.String())
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode
}
