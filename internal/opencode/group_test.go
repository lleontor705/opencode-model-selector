package opencode

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GroupByProvider — table-driven tests for REQ-OC-004
// ---------------------------------------------------------------------------

func TestGroupByProvider(t *testing.T) {
	tests := []struct {
		name               string
		input              []Model
		expectedKeys       []string
		expectedByProvider map[string][]Model
	}{
		{
			name:  "empty input returns empty map not nil",
			input: []Model{},
			// Spec: REQ-OC-004 — Scenario: Edge case — empty input slice
			expectedKeys:       []string{},
			expectedByProvider: map[string][]Model{},
		},
		{
			name: "single provider single model",
			input: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
			// Spec: REQ-OC-004 — Scenario: Single provider with single model
			expectedKeys: []string{"opencode-go"},
			expectedByProvider: map[string][]Model{
				"opencode-go": {
					{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				},
			},
		},
		{
			name: "multiple providers unsorted input becomes sorted by ID",
			input: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				{Provider: "openai", ID: "gpt-5.4", FullName: "openai/gpt-5.4"},
				{Provider: "opencode-go", ID: "deepseek-v4-pro", FullName: "opencode-go/deepseek-v4-pro"},
				{Provider: "openai", ID: "gpt-5.3-codex-spark", FullName: "openai/gpt-5.3-codex-spark"},
			},
			// Spec: REQ-OC-004 — Scenario: Multiple providers, models sorted alphabetically by ID
			expectedKeys: []string{"openai", "opencode-go"},
			expectedByProvider: map[string][]Model{
				"opencode-go": {
					{Provider: "opencode-go", ID: "deepseek-v4-pro", FullName: "opencode-go/deepseek-v4-pro"},
					{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				},
				"openai": {
					{Provider: "openai", ID: "gpt-5.3-codex-spark", FullName: "openai/gpt-5.3-codex-spark"},
					{Provider: "openai", ID: "gpt-5.4", FullName: "openai/gpt-5.4"},
				},
			},
		},
		{
			name: "single provider multiple models unsorted becomes sorted by ID",
			input: []Model{
				{Provider: "openai", ID: "gpt-5.6-terra", FullName: "openai/gpt-5.6-terra"},
				{Provider: "openai", ID: "gpt-5.4", FullName: "openai/gpt-5.4"},
				{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
			},
			// Spec: REQ-OC-004 — Scenario: Models sorted alphabetically by ID within their slice
			expectedKeys: []string{"openai"},
			expectedByProvider: map[string][]Model{
				"openai": {
					{Provider: "openai", ID: "gpt-5.4", FullName: "openai/gpt-5.4"},
					{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
					{Provider: "openai", ID: "gpt-5.6-terra", FullName: "openai/gpt-5.6-terra"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupByProvider(tt.input)

			// Map must never be nil — always initialized even for empty input
			require.NotNil(t, result, "must never return nil — always an initialized map")

			// Verify the key set matches exactly
			gotKeys := make([]string, 0, len(result))
			for k := range result {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)
			assert.Equal(t, tt.expectedKeys, gotKeys)

			// Verify each provider's models match exactly (order-sensitive)
			for provider, expectedModels := range tt.expectedByProvider {
				assert.Equal(t, expectedModels, result[provider],
					"provider %q models should match expected order", provider)
			}
		})
	}
}

// TestGroupByProvider_ReturnsEmptyMapNotNil verifies that empty input returns
// an initialized map, never nil.
//
// Spec: REQ-OC-004 — Scenario: Edge case — empty input slice
func TestGroupByProvider_ReturnsEmptyMapNotNil(t *testing.T) {
	result := GroupByProvider([]Model{})
	require.NotNil(t, result, "must never return nil — always an initialized map")
	assert.Len(t, result, 0)

	// Also verify with an explicit empty (default zero-value) slice
	result = GroupByProvider(nil)
	require.NotNil(t, result, "must never return nil even for nil input")
	assert.Len(t, result, 0)
}

// TestGroupByProvider_Fixture verifies grouping the real fixture with 53
// models from 6 providers.
//
// Spec: REQ-OC-004 — Scenario: 53 models from 6 providers
func TestGroupByProvider_Fixture(t *testing.T) {
	data, err := os.ReadFile("../../test/fixtures/models_output.txt")
	require.NoError(t, err, "fixture file must exist")

	models := ParseModelsOutput(string(data))
	require.Len(t, models, 53, "fixture must parse to exactly 53 models")

	grouped := GroupByProvider(models)

	// Spec: returns map with 6 keys
	require.Len(t, grouped, 6, "should group into exactly 6 provider keys")

	// Verify each expected provider exists with the correct model count
	expectedCounts := map[string]int{
		"opencode":              6,
		"opencode-go":           15,
		"minimax":               7,
		"openai":                13,
		"xiaomi-token-plan-sgp": 6,
		"zai-coding-plan":       6,
	}
	for provider, count := range expectedCounts {
		assert.Contains(t, grouped, provider, "provider %q must be a key", provider)
		assert.Len(t, grouped[provider], count,
			"provider %q should contain %d models", provider, count)
	}

	// Verify models within each provider are sorted alphabetically by ID
	for provider, providerModels := range grouped {
		for i := 1; i < len(providerModels); i++ {
			assert.True(t,
				providerModels[i-1].ID <= providerModels[i].ID,
				"provider %q: model at index %d (%q) should be <= model at index %d (%q) alphabetically by ID",
				provider, i-1, providerModels[i-1].ID, i, providerModels[i].ID)
		}
	}

	// Spot-check a specific provider's first and last entry
	openaiModels := grouped["openai"]
	require.NotEmpty(t, openaiModels)
	assert.Equal(t, "gpt-5.3-codex-spark", openaiModels[0].ID,
		"first openai model alphabetically should be gpt-5.3-codex-spark")
	assert.Equal(t, "gpt-5.6-terra-fast", openaiModels[len(openaiModels)-1].ID,
		"last openai model alphabetically should be gpt-5.6-terra-fast")

	// Verify all models across groups still sum to 53 (no models lost)
	total := 0
	for _, providerModels := range grouped {
		total += len(providerModels)
	}
	assert.Equal(t, 53, total, "total models across all providers must equal 53")
}
