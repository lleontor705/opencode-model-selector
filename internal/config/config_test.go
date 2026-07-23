package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixturePath returns the absolute path to a test fixture file.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	// Tests run from internal/config/, so fixtures are two levels up.
	p := filepath.Join("..", "..", "test", "fixtures", name)
	abs, err := filepath.Abs(p)
	require.NoError(t, err, "failed to resolve fixture path")
	return abs
}

// ---------------------------------------------------------------------------
// LoadConfig
// ---------------------------------------------------------------------------

func TestLoadConfig_ValidFile(t *testing.T) {
	path := fixturePath(t, "opencode.json")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	data := cfg.Data()
	assert.Contains(t, data, "$schema", "Data must contain $schema")
	assert.Contains(t, data, "agent", "Data must contain agent section")
	assert.Contains(t, data, "permission", "Data must contain permission section")
	assert.Contains(t, data, "mcp", "Data must contain mcp section")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestLoadConfig_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid json"), 0o600))

	_, err := LoadConfig(path)
	require.Error(t, err)

	// Must be a JSON parse error, NOT ErrConfigNotFound.
	assert.NotErrorIs(t, err, ErrConfigNotFound)
}

func TestLoadConfig_PathAccessor(t *testing.T) {
	path := fixturePath(t, "opencode.json")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, path, cfg.Path(), "Path() must return the path passed to LoadConfig")
}

// ---------------------------------------------------------------------------
// GetConfigPath
// ---------------------------------------------------------------------------

func TestGetConfigPath_DefaultPath(t *testing.T) {
	got, err := GetConfigPath("")
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.Join(home, ".config", "opencode", "opencode.json")

	assert.Equal(t, expected, got)
}

func TestGetConfigPath_OverridePath(t *testing.T) {
	got, err := GetConfigPath("/custom/path/opencode.json")
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/opencode.json", got)
}

func TestGetConfigPath_OverrideIsRelativeSafe(t *testing.T) {
	// An override is returned verbatim — no normalization.
	got, err := GetConfigPath("relative/opencode.json")
	require.NoError(t, err)
	assert.Equal(t, "relative/opencode.json", got)
}

// ---------------------------------------------------------------------------
// Save — atomic write, permissions, round-trip
// ---------------------------------------------------------------------------

func TestSave_WritesAtomically(t *testing.T) {
	srcPath := fixturePath(t, "opencode.json")
	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)

	dir := t.TempDir()
	cfg.path = filepath.Join(dir, "opencode.json")

	require.NoError(t, cfg.Save())

	// Final file exists.
	info, err := os.Stat(cfg.path)
	require.NoError(t, err, "saved file must exist")
	assert.False(t, info.IsDir())

	// No leftover temp file.
	_, err = os.Stat(cfg.path + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file must not remain after successful rename")

	// Content is valid JSON with 2-space indentation.
	content, err := os.ReadFile(cfg.path)
	require.NoError(t, err)
	var roundTripped map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &roundTripped))
	assert.Contains(t, roundTripped, "$schema")
	assert.Contains(t, roundTripped, "agent")
}

func TestSave_FilePermissionsUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("0o600 permission bits are not enforced on Windows")
	}

	srcPath := fixturePath(t, "opencode.json")
	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)

	dir := t.TempDir()
	cfg.path = filepath.Join(dir, "opencode.json")
	require.NoError(t, cfg.Save())

	info, err := os.Stat(cfg.path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"file must have 0o600 permissions on Unix")
}

func TestSave_RoundTripPreservesAPIKeys(t *testing.T) {
	srcPath := fixturePath(t, "opencode.json")
	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)

	dir := t.TempDir()
	cfg.path = filepath.Join(dir, "opencode.json")
	require.NoError(t, cfg.Save())

	// Reload and verify MCP API keys are identical.
	reloaded, err := LoadConfig(cfg.path)
	require.NoError(t, err)

	mcp, ok := reloaded.Data()["mcp"].(map[string]interface{})
	require.True(t, ok, "mcp section must exist after round-trip")

	zai, ok := mcp["zai-coding-plan"].(map[string]interface{})
	require.True(t, ok, "zai-coding-plan server must exist")

	env, ok := zai["env"].(map[string]interface{})
	require.True(t, ok, "env section must exist")

	assert.Equal(t, "fake-key-aaaa-bbbb", env["Z_AI_API_KEY"],
		"Z_AI_API_KEY must survive round-trip exactly")

	ctx7, ok := mcp["context7"].(map[string]interface{})
	require.True(t, ok, "context7 server must exist")
	ctxEnv, ok := ctx7["env"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "fake-key-cccc-dddd", ctxEnv["CONTEXT7_API_KEY"],
		"CONTEXT7_API_KEY must survive round-trip exactly")
}

func TestSave_RoundTripPreservesAllAgents(t *testing.T) {
	srcPath := fixturePath(t, "opencode.json")
	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)

	originalAgents := cfg.Data()["agent"].(map[string]interface{})

	dir := t.TempDir()
	cfg.path = filepath.Join(dir, "opencode.json")
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(cfg.path)
	require.NoError(t, err)

	reloadedAgents := reloaded.Data()["agent"].(map[string]interface{})

	// Same number of agents.
	assert.Equal(t, len(originalAgents), len(reloadedAgents),
		"agent count must be identical after round-trip")
	assert.Equal(t, 14, len(reloadedAgents), "fixture must have 14 agents")

	// Every original agent name present after round-trip.
	for name := range originalAgents {
		_, exists := reloadedAgents[name]
		assert.True(t, exists, "agent %q must exist after round-trip", name)
	}
}

func TestSave_RoundTripPreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")

	cfg := &Config{
		path: path,
		data: map[string]interface{}{
			"agent":    map[string]interface{}{},
			"futureField": 42,
		},
	}
	require.NoError(t, cfg.Save())

	reloaded, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, float64(42), reloaded.Data()["futureField"],
		"unknown field must survive round-trip")
}

func TestSave_UnwritableDestination(t *testing.T) {
	// Path inside a nonexistent directory — WriteFile for temp will fail.
	cfg := &Config{
		path: filepath.Join(t.TempDir(), "nonexistent-subdir", "opencode.json"),
		data: map[string]interface{}{"key": "value"},
	}

	err := cfg.Save()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWriteFailed)
}

func TestSave_OriginalUnchangedOnFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	originalContent := `{"$schema":"test","agent":{}}`
	require.NoError(t, os.WriteFile(configPath, []byte(originalContent), 0o600))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Redirect path to an unwritable location.
	cfg.path = filepath.Join(dir, "nonexistent-subdir", "opencode.json")
	err = cfg.Save()
	require.Error(t, err)

	// Original file must be unchanged.
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.JSONEq(t, originalContent, string(content),
		"original file must be unchanged when Save fails")
}
