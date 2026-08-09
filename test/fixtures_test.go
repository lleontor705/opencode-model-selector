package test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
)

// assertRequiredSections asserts the parsed config has the four required
// top-level sections. Shared by the .json and .jsonc fixture validators so
// both formats are held to the same semantic contract.
func assertRequiredSections(t *testing.T, config map[string]interface{}) {
	t.Helper()
	assert.Contains(t, config, "$schema", "config must have $schema")
	assert.Contains(t, config, "agent", "config must have agent section")
	assert.Contains(t, config, "permission", "config must have permission section")
	assert.Contains(t, config, "mcp", "config must have mcp section")
}

// TestOpencodeJSONFixtureIsValid loads the opencode.json fixture and verifies
// it is STRICTLY valid JSON (no comments, no trailing commas) with the required
// top-level sections. Validation for .json files is intentionally strict:
// standard `encoding/json` rejects anything that is not canonical JSON.
func TestOpencodeJSONFixtureIsValid(t *testing.T) {
	data, err := os.ReadFile("fixtures/opencode.json")
	require.NoError(t, err, "opencode.json fixture must exist")

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err, "opencode.json must be valid standard JSON")

	assertRequiredSections(t, config)
}

// TestOpencodeJSONCFixtureStandardizesToValidJSON loads the opencode.jsonc
// fixture — which contains line comments, a block comment, and trailing commas
// — and verifies it standardizes (via hujson) to valid JSON with the required
// top-level sections. This keeps .jsonc validation strict WITHOUT weakening
// the strict-JSON check that applies to .json files.
func TestOpencodeJSONCFixtureStandardizesToValidJSON(t *testing.T) {
	data, err := os.ReadFile("fixtures/opencode.jsonc")
	require.NoError(t, err, "opencode.jsonc fixture must exist")

	// Confirm the fixture actually exercises JSONC-only syntax so this test
	// stays meaningful if someone ever rewrites the fixture as plain JSON.
	body := string(data)
	require.True(t, strings.Contains(body, "//"),
		"opencode.jsonc fixture must contain at least one // line comment")
	require.True(t, strings.Contains(body, "/*"),
		"opencode.jsonc fixture must contain at least one /* block comment */")

	standardized, err := hujson.Standardize(data)
	require.NoError(t, err, "opencode.jsonc must standardize without error")

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(standardized, &config),
		"standardized opencode.jsonc must be valid JSON")

	assertRequiredSections(t, config)
}

// TestOpencodeJSONFixtureAgentCount verifies the fixture has 14 agents.
func TestOpencodeJSONFixtureAgentCount(t *testing.T) {
	data, err := os.ReadFile("fixtures/opencode.json")
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentSection, ok := config["agent"].(map[string]interface{})
	require.True(t, ok, "agent section must be a JSON object")
	assert.Len(t, agentSection, 14, "fixture must have exactly 14 agents")
}

// TestModelsOutputFixture verifies the models_output.txt fixture has exactly
// 53 lines, each containing a "/" separator.
func TestModelsOutputFixture(t *testing.T) {
	data, err := os.ReadFile("fixtures/models_output.txt")
	require.NoError(t, err, "models_output.txt fixture must exist")

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Equal(t, 53, len(lines), "models_output.txt must have exactly 53 lines")

	for i, line := range lines {
		assert.True(t, strings.Contains(line, "/"),
			"line %d must contain a '/' separator: %q", i+1, line)
	}
}

// TestModelsOutputFixtureProviders verifies all 6 expected providers appear.
func TestModelsOutputFixtureProviders(t *testing.T) {
	data, err := os.ReadFile("fixtures/models_output.txt")
	require.NoError(t, err)

	expectedProviders := []string{
		"opencode",
		"opencode-go",
		"minimax",
		"openai",
		"xiaomi-token-plan-sgp",
		"zai-coding-plan",
	}

	content := string(data)
	for _, provider := range expectedProviders {
		assert.True(t, strings.Contains(content, provider+"/"),
			"fixture must contain provider %q", provider)
	}
}
