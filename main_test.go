package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// ---------------------------------------------------------------------------
// parseFlags — Flag Parsing and Dispatch Decision
//
// All scenarios map to REQ-CMD-001. The parseFlags function is the pure,
// testable extraction of flag parsing and mode resolution. It does NOT call
// any external dependencies.
// ---------------------------------------------------------------------------

// TestParseFlags_NoFlags_DefaultsToTUI verifies that no flags results in TUI
// mode with default backup count of 5 and empty config path.
//
// Spec: REQ-CMD-001 — Scenario: Happy path — no flags launches TUI
func TestParseFlags_NoFlags_DefaultsToTUI(t *testing.T) {
	opts, code := parseFlags([]string{})

	require.Equal(t, 0, code, "no flags should return exit code 0")
	assert.Equal(t, modeTUI, opts.mode, "default mode should be TUI")
	assert.Equal(t, "", opts.configPath, "default config path should be empty")
	assert.Equal(t, 5, opts.backupCount, "default backup count should be 5")
}

// TestParseFlags_ListModels verifies that --list-models selects list-models
// mode.
//
// Spec: REQ-CMD-001 — Scenario: Edge case — --list-models flag
func TestParseFlags_ListModels(t *testing.T) {
	opts, code := parseFlags([]string{"--list-models"})

	require.Equal(t, 0, code)
	assert.Equal(t, modeListModels, opts.mode)
}

// TestParseFlags_ListAgents verifies that --list-agents selects list-agents
// mode.
//
// Spec: REQ-CMD-001 — Scenario: Edge case — --list-agents flag
func TestParseFlags_ListAgents(t *testing.T) {
	opts, code := parseFlags([]string{"--list-agents"})

	require.Equal(t, 0, code)
	assert.Equal(t, modeListAgents, opts.mode)
}

// TestParseFlags_ConfigOverride verifies that --config sets the config path
// and does not change the default TUI mode.
//
// Spec: REQ-CMD-001 — Scenario: Edge case — --config override
func TestParseFlags_ConfigOverride(t *testing.T) {
	opts, code := parseFlags([]string{"--config", "/custom/path.json"})

	require.Equal(t, 0, code)
	assert.Equal(t, "/custom/path.json", opts.configPath)
	assert.Equal(t, modeTUI, opts.mode, "config alone should still default to TUI")
}

// TestParseFlags_BackupCount3 verifies that --backup-count 3 sets retention.
//
// Spec: REQ-CMD-001 — Scenario: Edge case — --backup-count override
func TestParseFlags_BackupCount3(t *testing.T) {
	opts, code := parseFlags([]string{"--backup-count", "3"})

	require.Equal(t, 0, code)
	assert.Equal(t, 3, opts.backupCount)
}

// TestParseFlags_BackupCount0 verifies that --backup-count 0 disables backups.
//
// Spec: REQ-CMD-001 — Scenario: Edge case — --backup-count 0
func TestParseFlags_BackupCount0(t *testing.T) {
	opts, code := parseFlags([]string{"--backup-count", "0"})

	require.Equal(t, 0, code)
	assert.Equal(t, 0, opts.backupCount, "0 disables backups")
}

// TestParseFlags_MultipleModes_ListModelsPrecedence verifies that when both
// --list-models and --list-agents are set, list-models takes precedence.
//
// Spec: REQ-CMD-001 — "if multiple are set ... list-models, then list-agents"
func TestParseFlags_MultipleModes_ListModelsPrecedence(t *testing.T) {
	opts, code := parseFlags([]string{"--list-models", "--list-agents"})

	require.Equal(t, 0, code)
	assert.Equal(t, modeListModels, opts.mode,
		"list-models must take precedence over list-agents")
}

// TestParseFlags_InvalidFlag_ReturnsExit2 verifies that an unknown flag causes
// parseFlags to return exit code 2.
//
// Spec: REQ-CMD-001 — Scenario: Error — invalid flag value
func TestParseFlags_InvalidFlag_ReturnsExit2(t *testing.T) {
	_, code := parseFlags([]string{"--invalid-flag"})

	assert.Equal(t, 2, code, "invalid flag should return exit code 2")
}

// TestParseFlags_InvalidBackupCount_ReturnsExit2 verifies that a non-integer
// --backup-count value causes parseFlags to return exit code 2.
//
// Spec: REQ-CMD-001 — Scenario: Error — invalid flag value (non-integer)
func TestParseFlags_InvalidBackupCount_ReturnsExit2(t *testing.T) {
	_, code := parseFlags([]string{"--backup-count", "abc"})

	assert.Equal(t, 2, code, "non-integer backup-count should return exit code 2")
}

// TestParseFlags_ConfigWithListModels verifies that --config and --list-models
// can be combined, with both values correctly parsed.
func TestParseFlags_ConfigWithListModels(t *testing.T) {
	opts, code := parseFlags([]string{"--config", "/my/config.json", "--list-models"})

	require.Equal(t, 0, code)
	assert.Equal(t, "/my/config.json", opts.configPath)
	assert.Equal(t, modeListModels, opts.mode)
}

// TestParseFlags_AllFlagsCombined verifies that all flags work together.
func TestParseFlags_AllFlagsCombined(t *testing.T) {
	opts, code := parseFlags([]string{
		"--config", "/custom.json",
		"--list-models",
		"--backup-count", "10",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, "/custom.json", opts.configPath)
	assert.Equal(t, modeListModels, opts.mode)
	assert.Equal(t, 10, opts.backupCount)
}

// TestParseFlags_BackupCountNegativeAllowed verifies that negative values are
// parsed (validation happens in config, not in flag parsing).
func TestParseFlags_BackupCountNegativeAllowed(t *testing.T) {
	opts, code := parseFlags([]string{"--backup-count", "-1"})

	require.Equal(t, 0, code, "flag parsing should accept negative integers")
	assert.Equal(t, -1, opts.backupCount)
}

// ---------------------------------------------------------------------------
// run — Entry Point Dispatch
// ---------------------------------------------------------------------------

// TestRun_FlagError_ReturnsExit2 verifies that run() propagates exit code 2
// from parseFlags when an invalid flag is passed, without calling any
// external dependencies.
func TestRun_FlagError_ReturnsExit2(t *testing.T) {
	code := run([]string{"--invalid-flag"})

	assert.Equal(t, 2, code, "run should return 2 for invalid flags")
}

// ---------------------------------------------------------------------------
// Test helpers for formatModels / formatAgents tests
// ---------------------------------------------------------------------------

// loadTestModels loads the fixture models_output.txt for integration-style tests.
func loadTestModels(t *testing.T) []opencode.Model {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("test", "fixtures", "models_output.txt"))
	require.NoError(t, err, "fixture file must exist")
	data, err := os.ReadFile(abs)
	require.NoError(t, err, "failed to read fixture")
	return opencode.ParseModelsOutput(string(data))
}

// loadTestConfig loads the fixture opencode.json for integration-style tests.
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("test", "fixtures", "opencode.json"))
	require.NoError(t, err, "failed to resolve fixture path")
	cfg, err := config.LoadConfig(abs)
	require.NoError(t, err)
	return cfg
}

// ---------------------------------------------------------------------------
// formatModels — Model Listing Output (REQ-CMD-002)
//
// formatModels writes the grouped model listing to an io.Writer, making it
// testable with bytes.Buffer.
// ---------------------------------------------------------------------------

// TestFormatModels_EmptyPrintsNoModelsAvailable verifies that an empty model
// slice produces "No models available".
//
// Spec: REQ-CMD-002 — Scenario: Edge case — no models available
func TestFormatModels_EmptyPrintsNoModelsAvailable(t *testing.T) {
	var buf bytes.Buffer
	err := formatModels(&buf, []opencode.Model{})

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No models available")
}

// TestFormatModels_NilPrintsNoModelsAvailable verifies that a nil model slice
// also produces "No models available".
//
// Spec: REQ-CMD-002 — Scenario: Edge case — nil input
func TestFormatModels_NilPrintsNoModelsAvailable(t *testing.T) {
	var buf bytes.Buffer
	err := formatModels(&buf, nil)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No models available")
}

// TestFormatModels_HeaderContainsTotalCount verifies the header shows the
// correct model count.
//
// Spec: REQ-CMD-002 — Scenario: Happy path — output format
func TestFormatModels_HeaderContainsTotalCount(t *testing.T) {
	models := loadTestModels(t)
	var buf bytes.Buffer
	err := formatModels(&buf, models)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Available Models (53 total)")
}

// TestFormatModels_AllProviderHeadersPresent verifies that all 6 provider
// section headers appear with their model counts.
//
// Spec: REQ-CMD-002 — Scenario: Happy path — providers grouped
func TestFormatModels_AllProviderHeadersPresent(t *testing.T) {
	models := loadTestModels(t)
	var buf bytes.Buffer
	err := formatModels(&buf, models)
	require.NoError(t, err)

	output := buf.String()
	expectedProviders := map[string]int{
		"opencode":              6,
		"opencode-go":           15,
		"minimax":               7,
		"openai":                13,
		"xiaomi-token-plan-sgp": 6,
		"zai-coding-plan":       6,
	}
	for provider, count := range expectedProviders {
		header := fmt.Sprintf("%s/ (%d)", provider, count)
		assert.Contains(t, output, header,
			"provider section header for %q with count %d must appear", provider, count)
	}
}

// TestFormatModels_ModelsAppearUnderCorrectProvider verifies that a specific
// model appears under its provider section.
//
// Spec: REQ-CMD-002 — Scenario: models listed under correct provider
func TestFormatModels_ModelsAppearUnderCorrectProvider(t *testing.T) {
	models := loadTestModels(t)
	var buf bytes.Buffer
	err := formatModels(&buf, models)
	require.NoError(t, err)

	output := buf.String()

	// glm-5.2 is under opencode-go/
	assert.Contains(t, output, "glm-5.2",
		"glm-5.2 model must appear in output")
	goIdx := strings.Index(output, "opencode-go/ (")
	require.GreaterOrEqual(t, goIdx, 0,
		"opencode-go section header must exist")
	glmIdx := strings.Index(output[goIdx:], "glm-5.2")
	assert.GreaterOrEqual(t, glmIdx, 0,
		"glm-5.2 must appear under opencode-go/ section")

	// Big model from minimax/
	assert.Contains(t, output, "MiniMax-M3",
		"MiniMax-M3 model must appear in output")
	minimaxIdx := strings.Index(output, "minimax/ (")
	require.GreaterOrEqual(t, minimaxIdx, 0)
	mmIdx := strings.Index(output[minimaxIdx:], "MiniMax-M3")
	assert.GreaterOrEqual(t, mmIdx, 0,
		"MiniMax-M3 must appear under minimax/ section")
}

// TestFormatModels_FooterContainsTotalAndProviderCount verifies the footer
// line shows the correct total and provider count.
//
// Spec: REQ-CMD-002 — Scenario: footer with totals
func TestFormatModels_FooterContainsTotalAndProviderCount(t *testing.T) {
	models := loadTestModels(t)
	var buf bytes.Buffer
	err := formatModels(&buf, models)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Total: 53 models across 6 providers")
}

// TestFormatModels_ProvidersSortedAlphabetically verifies that provider
// sections appear in alphabetical order.
//
// Spec: REQ-CMD-002 — "Providers sorted alphabetically for deterministic output"
func TestFormatModels_ProvidersSortedAlphabetically(t *testing.T) {
	models := loadTestModels(t)
	var buf bytes.Buffer
	err := formatModels(&buf, models)
	require.NoError(t, err)

	output := buf.String()
	// Expected alphabetical order of section headers
	providers := []string{
		"minimax", "openai", "opencode", "opencode-go",
		"xiaomi-token-plan-sgp", "zai-coding-plan",
	}

	indices := make([]int, len(providers))
	for i, p := range providers {
		idx := strings.Index(output, p+"/ (")
		require.GreaterOrEqual(t, idx, 0, "provider %q header must exist", p)
		indices[i] = idx
	}

	for i := 1; i < len(indices); i++ {
		assert.Less(t, indices[i-1], indices[i],
			"provider %q must appear before %q", providers[i-1], providers[i])
	}
}

// ---------------------------------------------------------------------------
// formatAgents — Agent Listing Output (REQ-CMD-003)
//
// formatAgents writes the agent listing to an io.Writer, making it testable
// with bytes.Buffer.
// ---------------------------------------------------------------------------

// TestFormatAgents_HeaderPresent verifies the output contains the header.
//
// Spec: REQ-CMD-003 — Scenario: header
func TestFormatAgents_HeaderPresent(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "OpenCode Agents")
}

// TestFormatAgents_GlobalModelNotSet verifies "(none)" appears when no global
// model is configured.
//
// Spec: REQ-CMD-003 — Scenario: global default model not set
func TestFormatAgents_GlobalModelNotSet(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Global Default Model: (none)")
}

// TestFormatAgents_GlobalModelSet verifies the global model value appears when
// set.
//
// Spec: REQ-CMD-003 — Scenario: global default model set
func TestFormatAgents_GlobalModelSet(t *testing.T) {
	cfg := loadTestConfig(t)
	cfg.SetGlobalModel("opencode-go/glm-5.2")
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Global Default Model: opencode-go/glm-5.2")
}

// TestFormatAgents_PrimaryAgentsSection verifies the primary agents section
// exists and contains the known primary agents.
//
// Spec: REQ-CMD-003 — Scenario: primary agents section
func TestFormatAgents_PrimaryAgentsSection(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Primary Agents")
	assert.Contains(t, output, "build")
	assert.Contains(t, output, "plan")
}

// TestFormatAgents_SubagentsSection verifies the subagents section exists and
// contains known subagents.
//
// Spec: REQ-CMD-003 — Scenario: subagents section
func TestFormatAgents_SubagentsSection(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Subagents")
	assert.Contains(t, output, "code-reviewer")
	assert.Contains(t, output, "parallel-dispatch")
}

// TestFormatAgents_AgentWithModelShowsValue verifies that an agent with a model
// field shows the model value.
//
// Spec: REQ-CMD-003 — Scenario: agent model field
func TestFormatAgents_AgentWithModelShowsValue(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "anthropic/claude-sonnet-4-20250514",
		"code-reviewer model must appear in output")
}

// TestFormatAgents_AgentWithoutModelShowsNone verifies that an agent without a
// model shows "(none)" for the model field.
//
// Spec: REQ-CMD-003 — Scenario: agent without model
func TestFormatAgents_AgentWithoutModelShowsNone(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	// build has no model field — should show "(none)" in its section
	buildIdx := strings.Index(output, "build")
	require.GreaterOrEqual(t, buildIdx, 0, "build must appear")
	buildSection := output[buildIdx:]
	// Within build's section, "model:" must show "(none)"
	modelIdx := strings.Index(buildSection, "model:")
	require.GreaterOrEqual(t, modelIdx, 0, "model field must appear")
	assert.Contains(t, buildSection[modelIdx:], "(none)",
		"build model must be (none)")
}

// TestFormatAgents_TemperatureShownAsFloat verifies temperature shows as float.
//
// Spec: REQ-CMD-003 — "Temperature/top_p show as float"
func TestFormatAgents_TemperatureShownAsFloat(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	// plan has temperature: 0.4
	assert.Contains(t, buf.String(), "0.4",
		"temperature must show as 0.4")
	// Ensure we're NOT showing "0.4000" or similar
	assert.NotContains(t, buf.String(), "0.4000",
		"temperature must not have trailing zeros")
}

// TestFormatAgents_DisabledAgentMarked verifies that disabled agents show the
// [DISABLED] marker.
//
// Spec: REQ-CMD-003 — Scenario: disabled agent marker
func TestFormatAgents_DisabledAgentMarked(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "[DISABLED]",
		"disabled agent (build) must have [DISABLED] marker")
}

// TestFormatAgents_HiddenAgentMarked verifies that hidden agents show the [H]
// marker next to the agent name.
//
// Spec: REQ-CMD-003 — Scenario: hidden agent marker
func TestFormatAgents_HiddenAgentMarked(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "parallel-dispatch [H]",
		"hidden agent (parallel-dispatch) must have [H] marker after name")
}

// TestFormatAgents_AllSixFieldsShown verifies that all 6 editable field labels
// appear in the output.
//
// Spec: REQ-CMD-003 — "Each agent shows all 6 editable fields"
func TestFormatAgents_AllSixFieldsShown(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	fields := []string{"model:", "temperature:", "top_p:", "color:", "steps:", "disable:"}
	for _, field := range fields {
		assert.Contains(t, output, field, "field %q must appear", field)
	}
}

// TestFormatAgents_SystemAgentsExcluded verifies that system agents do not
// appear in the output.
//
// Spec: REQ-CMD-003 — Scenario: system agents excluded
func TestFormatAgents_SystemAgentsExcluded(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	for _, name := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, output, name,
			"system agent %q must NOT appear in output", name)
	}
}

// TestFormatAgents_NonSystemAgentCount verifies that exactly 11 non-system
// agents appear in the output (14 total - 3 system).
//
// Spec: REQ-CMD-003 — Scenario: only non-system agents shown
func TestFormatAgents_NonSystemAgentCount(t *testing.T) {
	cfg := loadTestConfig(t)
	var buf bytes.Buffer
	err := formatAgents(&buf, cfg)
	require.NoError(t, err)

	output := buf.String()
	// All 11 non-system agents must appear
	expectedAgents := []string{
		"build", "plan", // primary
		"code-reviewer", "debug", "docs", "explore", "general",
		"orchestrator", "parallel-dispatch", "security-auditor", "team-lead", // subagents
	}
	for _, name := range expectedAgents {
		assert.Contains(t, output, name,
			"non-system agent %q must appear", name)
	}
}

// ---------------------------------------------------------------------------
// run — Startup Flow and Error Handling (REQ-CMD-004, REQ-CMD-005)
//
// These tests verify the full run() startup flow: config resolution, config
// loading, opencode detection, and dispatch routing with proper exit codes
// and error messages.
//
// Testing notes:
//   - list-agents mode does NOT call opencode.Detect/GetModels, so it is
//     fully testable in CI.
//   - list-models and TUI modes require opencode on PATH. We cannot control
//     the test environment's PATH from the main package, so we verify the
//     routing structure indirectly (list-agents succeeds → proves opencode
//     is NOT required for that path).
//   - runTUI is a thin wrapper over tea.NewProgram().Run() which takes over
//     the terminal — it cannot be tested in automated CI.
// ---------------------------------------------------------------------------

// captureStderr temporarily redirects os.Stderr to a buffer, runs fn, then
// restores the original stderr. Returns whatever was written during fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

// captureStdout temporarily redirects os.Stdout to a buffer, runs fn, then
// restores the original stdout. Returns whatever was written during fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

// resolveFixtureConfigPath returns the absolute path to the test opencode.json
// fixture. Centralizes the path resolution for run() integration tests.
func resolveFixtureConfigPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("test", "fixtures", "opencode.json"))
	require.NoError(t, err)
	return abs
}

// TestRun_ConfigNotFound_ReturnsExit1_PrintsConfigNotFound verifies that when
// the config file does not exist, run() returns exit code 1 and prints a
// user-friendly "Config not found at {path}" message to stderr.
//
// Spec: REQ-CMD-004 — "Config file not found → stderr: 'Config not found at {path}', exit 1"
func TestRun_ConfigNotFound_ReturnsExit1_PrintsConfigNotFound(t *testing.T) {
	bogusPath := "/nonexistent/path/to/opencode.json"

	stderr := captureStderr(t, func() {
		code := run([]string{"--config", bogusPath, "--list-agents"})
		assert.Equal(t, 1, code, "missing config should return exit code 1")
	})

	assert.Contains(t, stderr, "Config not found at",
		"stderr must contain 'Config not found at' for missing config")
	assert.Contains(t, stderr, bogusPath,
		"stderr must contain the resolved config path")
}

// TestRun_ConfigMalformed_ReturnsExit1_PrintsError verifies that when the
// config file exists but contains invalid JSON, run() returns exit code 1
// and prints "Error loading config:" with the parse error to stderr.
//
// Spec: REQ-CMD-004 — "Config malformed → stderr: 'Error loading config: {parse error}', exit 1"
func TestRun_ConfigMalformed_ReturnsExit1_PrintsError(t *testing.T) {
	// Create a temp file with invalid JSON.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bad.json")
	err := os.WriteFile(tmpFile, []byte("{ this is not valid json"), 0o644)
	require.NoError(t, err)

	stderr := captureStderr(t, func() {
		code := run([]string{"--config", tmpFile, "--list-agents"})
		assert.Equal(t, 1, code, "malformed config should return exit code 1")
	})

	assert.Contains(t, stderr, "Error loading config:",
		"stderr must contain 'Error loading config:' prefix for parse errors")
}

// TestRun_ConfigMalformed_DoesNotPrintConfigNotFound verifies that a JSON parse
// error is NOT classified as "Config not found" — it must use the generic
// "Error loading config" path.
//
// Spec: REQ-CMD-004 — ErrConfigNotFound only fires for missing files, not corrupt ones.
func TestRun_ConfigMalformed_DoesNotPrintConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "corrupt.json")
	err := os.WriteFile(tmpFile, []byte("{broken"), 0o644)
	require.NoError(t, err)

	stderr := captureStderr(t, func() {
		run([]string{"--config", tmpFile, "--list-agents"})
	})

	assert.NotContains(t, stderr, "Config not found",
		"parse error must NOT trigger the 'Config not found' message")
}

// TestRun_ListAgentsSucceedsWithValidConfig verifies that list-agents mode
// completes successfully (exit 0) when a valid config is provided, WITHOUT
// requiring opencode to be installed.
//
// Spec: REQ-CMD-004 — "list-agents works without opencode installed"
func TestRun_ListAgentsSucceedsWithValidConfig(t *testing.T) {
	configPath := resolveFixtureConfigPath(t)

	var code int
	_ = captureStdout(t, func() {
		code = run([]string{"--config", configPath, "--list-agents"})
	})

	assert.Equal(t, 0, code,
		"list-agents with valid config should succeed (exit 0) without opencode")
}

// TestRun_NoPanic_OnAnyErrorPath verifies that run() never panics on any
// error condition — all errors must be handled gracefully.
//
// Spec: REQ-CMD-004 — "No panics on any startup error"
func TestRun_NoPanic_OnAnyErrorPath(t *testing.T) {
	configPath := resolveFixtureConfigPath(t)

	// These should all complete without panicking.
	assert.NotPanics(t, func() {
		_ = captureStderr(t, func() {
			run([]string{"--config", "/nonexistent.json", "--list-agents"})
		})
	}, "missing config should not panic")

	assert.NotPanics(t, func() {
		tmpDir := t.TempDir()
		badFile := filepath.Join(tmpDir, "bad.json")
		_ = os.WriteFile(badFile, []byte("{invalid"), 0o644)
		_ = captureStderr(t, func() {
			run([]string{"--config", badFile, "--list-agents"})
		})
	}, "malformed config should not panic")

	assert.NotPanics(t, func() {
		_ = captureStdout(t, func() {
			run([]string{"--config", configPath, "--list-agents"})
		})
	}, "valid list-agents should not panic")
}

// TestRun_DefaultModeIsTUI verifies that running with no mode flags resolves
// to TUI mode. We verify this at the flag-parsing level, NOT by calling run()
// directly — when opencode IS installed, run() launches the actual TUI which
// blocks forever waiting for terminal input (untestable in CI).
//
// Spec: REQ-CMD-001 — "When no mode flags are set, default is TUI"
func TestRun_DefaultModeIsTUI(t *testing.T) {
	opts, code := parseFlags([]string{})

	require.Equal(t, 0, code, "no flags should return exit code 0")
	assert.Equal(t, modeTUI, opts.mode,
		"default mode with no flags must be TUI")
}

// ---------------------------------------------------------------------------
// parseFlags — Apply-Model Mode (REQ-CLI-001, REQ-CLI-002)
//
// Tests for --apply-model and --agents flags, their mutual requirement, and
// dispatch precedence against existing modes.
// ---------------------------------------------------------------------------

// TestParseFlags_ApplyModel_ParsedCorrectly verifies that --apply-model and
// --agents are parsed and mode resolves to modeApplyModel.
//
// Spec: REQ-CLI-001 — Scenario: Happy path — flag parsed
func TestParseFlags_ApplyModel_ParsedCorrectly(t *testing.T) {
	opts, code := parseFlags([]string{"--apply-model", "openai/gpt-5", "--agents", "all"})

	require.Equal(t, 0, code)
	assert.Equal(t, "openai/gpt-5", opts.applyModel)
	assert.Equal(t, "all", opts.agentsCSV)
	assert.Equal(t, modeApplyModel, opts.mode)
}

// TestParseFlags_ApplyModel_ValueWithSlashesPreserved verifies that model IDs
// containing slashes and dashes are stored verbatim.
//
// Spec: REQ-CLI-001 — Scenario: Edge case — model ID containing slashes and dashes
func TestParseFlags_ApplyModel_ValueWithSlashesPreserved(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "anthropic/claude-3-5-sonnet-20241022",
		"--agents", "all",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, "anthropic/claude-3-5-sonnet-20241022", opts.applyModel,
		"model ID with slashes and dashes must be preserved verbatim")
}

// TestParseFlags_AgentsAll_ParsedCorrectly verifies that --agents all is
// stored as the literal string "all".
//
// Spec: REQ-CLI-002 — Scenario: Happy path — all
func TestParseFlags_AgentsAll_ParsedCorrectly(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "openai/gpt-5",
		"--agents", "all",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, "all", opts.agentsCSV)
	assert.Equal(t, modeApplyModel, opts.mode)
}

// TestParseFlags_AgentsCSV_RawStored verifies that a comma-separated --agents
// value is stored verbatim (trimming of whitespace happens in runApplyModel).
//
// Spec: REQ-CLI-002 — Scenario: Happy path — CSV with whitespace
func TestParseFlags_AgentsCSV_RawStored(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "openai/gpt-5",
		"--agents", "build, plan , explore",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, "build, plan , explore", opts.agentsCSV,
		"CSV string must be stored raw; trimming happens in runApplyModel")
	assert.Equal(t, modeApplyModel, opts.mode)
}

// TestParseFlags_ApplyModelWithoutAgents_Exits2 verifies that --apply-model
// without --agents is a usage error (exit 2).
//
// Spec: REQ-CLI-002 — Scenario: Error — --apply-model without --agents
func TestParseFlags_ApplyModelWithoutAgents_Exits2(t *testing.T) {
	_, code := parseFlags([]string{"--apply-model", "openai/gpt-5"})

	assert.Equal(t, 2, code,
		"--apply-model without --agents must return exit code 2")
}

// TestParseFlags_AgentsWithoutApplyModel_Exits2 verifies that --agents without
// --apply-model is also a usage error (exit 2).
//
// Spec: REQ-CLI-002 — Scenario: Error — --agents without --apply-model
func TestParseFlags_AgentsWithoutApplyModel_Exits2(t *testing.T) {
	_, code := parseFlags([]string{"--agents", "all"})

	assert.Equal(t, 2, code,
		"--agents without --apply-model must return exit code 2")
}

// TestParseFlags_Precedence_ApplyModelOverTUI verifies that apply-model
// outranks TUI (the default) but is below list-*.
//
// Spec: REQ-CLI-001 — Dispatch precedence: list-models > list-agents > apply-model > TUI
func TestParseFlags_Precedence_ApplyModelOverTUI(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "openai/gpt-5",
		"--agents", "all",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, modeApplyModel, opts.mode,
		"apply-model must take precedence over TUI default")
}

// TestParseFlags_Precedence_ListModelsOverApplyModel verifies that list-models
// outranks apply-model.
//
// Spec: REQ-CLI-001 — Dispatch precedence: list-models > list-agents > apply-model > TUI
func TestParseFlags_Precedence_ListModelsOverApplyModel(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "openai/gpt-5",
		"--agents", "all",
		"--list-models",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, modeListModels, opts.mode,
		"list-models must take precedence over apply-model")
}

// TestParseFlags_Precedence_ListAgentsOverApplyModel verifies that list-agents
// outranks apply-model.
//
// Spec: REQ-CLI-001 — Dispatch precedence: list-models > list-agents > apply-model > TUI
func TestParseFlags_Precedence_ListAgentsOverApplyModel(t *testing.T) {
	opts, code := parseFlags([]string{
		"--apply-model", "openai/gpt-5",
		"--agents", "all",
		"--list-agents",
	})

	require.Equal(t, 0, code)
	assert.Equal(t, modeListAgents, opts.mode,
		"list-agents must take precedence over apply-model")
}

// ---------------------------------------------------------------------------
// runApplyModel / applyModelWithModels — CLI Bulk Apply (REQ-CLI-003..008)
//
// These tests exercise the CLI bulk-apply logic via the testable inner function
// applyModelWithModels, which accepts models as a parameter. The outer
// runApplyModel is a thin wrapper that calls opencode.Detect + GetModels and
// delegates — it cannot be unit-tested from the main package because the
// opencode command/lookPath indirection variables are unexported.
// ---------------------------------------------------------------------------

// makeTempConfig copies the fixture opencode.json to a temp dir and loads it.
// Returns a Config whose Path() points to the writable temp copy.
func makeTempConfig(t *testing.T) *config.Config {
	t.Helper()
	fixturePath := resolveFixtureConfigPath(t)
	src, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "opencode.json")
	require.NoError(t, os.WriteFile(dst, src, 0o644))
	cfg, err := config.LoadConfig(dst)
	require.NoError(t, err)
	return cfg
}

// mockModels returns a small set of models for apply-model tests.
func mockModels() []opencode.Model {
	return []opencode.Model{
		{Provider: "openai", ID: "gpt-5", FullName: "openai/gpt-5"},
		{Provider: "anthropic", ID: "claude-sonnet-4-20250514", FullName: "anthropic/claude-sonnet-4-20250514"},
	}
}

// makeEmptyConfig creates a config with ONLY system agents (no targetable
// agents), used for the empty-target-set test.
func makeEmptyConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "opencode.json")
	content := `{
  "agent": {
    "compactación": { "mode": "primary", "model": "old/model" },
    "title":        { "mode": "primary", "model": "old/model" },
    "summary":      { "mode": "primary", "model": "old/model" }
  }
}`
	require.NoError(t, os.WriteFile(dst, []byte(content), 0o644))
	cfg, err := config.LoadConfig(dst)
	require.NoError(t, err)
	return cfg
}

// createFakeBackups creates N fake backup files in the same directory as
// configPath, using timestamps that sort lexicographically (oldest first).
func createFakeBackups(t *testing.T, configPath string, count int) {
	t.Helper()
	dir := filepath.Dir(configPath)
	for i := 0; i < count; i++ {
		ts := fmt.Sprintf("2025010%d-120000", i)
		bp := filepath.Join(dir, fmt.Sprintf("opencode.json.backup.%s", ts))
		require.NoError(t, os.WriteFile(bp, []byte("{}"), 0o644))
	}
}

// countBackups returns the number of backup files in the config directory.
func countBackups(t *testing.T, configPath string) int {
	t.Helper()
	dir := filepath.Dir(configPath)
	matches, err := filepath.Glob(filepath.Join(dir, "opencode.json.backup.*"))
	require.NoError(t, err)
	return len(matches)
}

// TestRunApplyModel_Success_All verifies that a valid model applied to "all"
// mutates every non-system, non-disabled agent, creates a backup, and saves.
//
// Spec: REQ-CLI-003 — Scenario: Happy path — model valid
// Spec: REQ-CLI-004 — Scenario: Happy path — backup created
// Spec: REQ-CLI-005 — Scenario: Happy path — save succeeds
func TestRunApplyModel_Success_All(t *testing.T) {
	cfg := makeTempConfig(t)
	models := mockModels()

	err := applyModelWithModels(cfg, "openai/gpt-5", "all", 5, models)

	require.NoError(t, err)

	primary, subagents, _ := cfg.GetAgents()
	for _, name := range append(primary, subagents...) {
		if cfg.IsAgentDisabled(name) {
			continue
		}
		val, ok := cfg.GetAgentField(name, "model")
		require.True(t, ok, "agent %s must have model field set", name)
		assert.Equal(t, "openai/gpt-5", val.(string),
			"agent %s must have openai/gpt-5", name)
	}

	assert.Equal(t, 1, countBackups(t, cfg.Path()),
		"exactly one backup must be created")
}

// TestRunApplyModel_Success_CSV verifies that a CSV list applies only to named
// agents and leaves others untouched.
//
// Spec: REQ-CLI-002 — Scenario: Happy path — CSV
func TestRunApplyModel_Success_CSV(t *testing.T) {
	cfg := makeTempConfig(t)
	models := mockModels()

	err := applyModelWithModels(cfg, "openai/gpt-5", "plan, code-reviewer", 5, models)

	require.NoError(t, err)

	planVal, ok := cfg.GetAgentField("plan", "model")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", planVal)

	crVal, ok := cfg.GetAgentField("code-reviewer", "model")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", crVal)

	exploreVal, ok := cfg.GetAgentField("explore", "model")
	if ok {
		exploreStr, _ := exploreVal.(string)
		assert.NotEqual(t, "openai/gpt-5", exploreStr,
			"explore must NOT be modified by CSV apply")
	}
}

// TestRunApplyModel_InvalidModel_Exits1 verifies that an unknown model returns
// an error without mutating config or creating a backup.
//
// Spec: REQ-CLI-003 — Scenario: Error — unknown model
func TestRunApplyModel_InvalidModel(t *testing.T) {
	cfg := makeTempConfig(t)
	models := mockModels()

	err := applyModelWithModels(cfg, "bogus/model", "all", 5, models)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model")

	crVal, ok := cfg.GetAgentField("code-reviewer", "model")
	if ok {
		assert.Equal(t, "anthropic/claude-sonnet-4-20250514", crVal.(string),
			"config must NOT be mutated on validation failure")
	}
	assert.Equal(t, 0, countBackups(t, cfg.Path()),
		"no backup must be created on validation failure")
}

// TestRunApplyModel_EmptyTargets verifies that an empty target set (config with
// only system agents) exits successfully without creating a backup or saving.
//
// Spec: REQ-CLI-007 — Scenario: Happy path — config has zero non-system agents
func TestRunApplyModel_EmptyTargets(t *testing.T) {
	cfg := makeEmptyConfig(t)
	models := mockModels()

	stdout := captureStdout(t, func() {
		err := applyModelWithModels(cfg, "openai/gpt-5", "all", 5, models)
		assert.NoError(t, err)
	})

	assert.Contains(t, stdout, "0 agents updated")
	assert.Equal(t, 0, countBackups(t, cfg.Path()),
		"no backup for empty target set")
}

// TestRunApplyModel_BackupRetention verifies that --backup-count 2 keeps only
// the 2 newest backups after save.
//
// Spec: REQ-CLI-006 — Scenario: Happy path — retention enforced
func TestRunApplyModel_BackupRetention(t *testing.T) {
	cfg := makeTempConfig(t)
	createFakeBackups(t, cfg.Path(), 3)
	models := mockModels()

	err := applyModelWithModels(cfg, "openai/gpt-5", "all", 2, models)

	require.NoError(t, err)
	assert.Equal(t, 2, countBackups(t, cfg.Path()),
		"exactly 2 backups must remain after retention")
}

// TestRunApplyModel_BackupDisabled verifies that backupCount=0 skips both
// backup creation and cleanup while save still works.
//
// Spec: REQ-CLI-004 — Scenario: Edge case — backupCount == 0
// Spec: REQ-CLI-006 — "--backup-count 0 skips both backup creation AND cleanup"
func TestRunApplyModel_BackupDisabled(t *testing.T) {
	cfg := makeTempConfig(t)
	createFakeBackups(t, cfg.Path(), 2)
	models := mockModels()

	err := applyModelWithModels(cfg, "openai/gpt-5", "all", 0, models)

	require.NoError(t, err)

	assert.Equal(t, 2, countBackups(t, cfg.Path()),
		"backupCount=0 must NOT create new backups or clean old ones")

	planVal, ok := cfg.GetAgentField("plan", "model")
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5", planVal,
		"save must still run even with backupCount=0")
}

// TestRunApplyModel_SkipsDisabled verifies that disabled agents appear in the
// skipped summary and are not mutated.
//
// Spec: REQ-CLI-008 — Scenario: Happy path — 3 applied, 1 skipped
// Spec: REQ-BULK-003 — disabled agents skipped
func TestRunApplyModel_SkipsDisabled(t *testing.T) {
	cfg := makeTempConfig(t)
	models := mockModels()

	stdout := captureStdout(t, func() {
		err := applyModelWithModels(cfg, "openai/gpt-5", "all", 5, models)
		assert.NoError(t, err)
	})

	assert.Contains(t, stdout, "Skipped")
	assert.Contains(t, stdout, "build",
		"disabled agent 'build' must appear in skipped list")
}

// TestRunApplyModel_StdoutSummary verifies the output format matches REQ-CLI-008.
//
// Spec: REQ-CLI-008 — Scenario: Happy path — summary format
func TestRunApplyModel_StdoutSummary(t *testing.T) {
	cfg := makeTempConfig(t)
	models := mockModels()

	stdout := captureStdout(t, func() {
		err := applyModelWithModels(cfg, "openai/gpt-5", "plan", 5, models)
		assert.NoError(t, err)
	})

	assert.Contains(t, stdout, "Model openai/gpt-5 applied to")
	assert.Contains(t, stdout, "plan")
	assert.Contains(t, stdout, "✓")
}
