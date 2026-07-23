package test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpencodeJSONFixtureIsValid loads the opencode.json fixture and verifies
// it is valid JSON with the required top-level sections.
func TestOpencodeJSONFixtureIsValid(t *testing.T) {
	data, err := os.ReadFile("fixtures/opencode.json")
	require.NoError(t, err, "opencode.json fixture must exist")

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	require.NoError(t, err, "opencode.json must be valid JSON")

	assert.Contains(t, config, "$schema", "config must have $schema")
	assert.Contains(t, config, "agent", "config must have agent section")
	assert.Contains(t, config, "permission", "config must have permission section")
	assert.Contains(t, config, "mcp", "config must have mcp section")
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
