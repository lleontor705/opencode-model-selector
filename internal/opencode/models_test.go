package opencode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseModelsOutput — table-driven tests for REQ-OC-003
// ---------------------------------------------------------------------------

func TestParseModelsOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Model
	}{
		{
			name:  "empty string returns empty slice not nil",
			input: "",
			// Spec: REQ-OC-003 — Scenario: Error — completely empty string input
			expected: []Model{},
		},
		{
			name:  "clean provider/model lines",
			input: "opencode-go/glm-5.2\nopenai/gpt-5.5\n",
			// Spec: REQ-OC-003 — Scenario: Happy path — clean provider/model lines
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
			},
		},
		{
			name:  "trailing whitespace stripped",
			input: "opencode-go/glm-5.2   \n",
			// Spec: REQ-OC-003 — Scenario: Edge case — trailing whitespace on lines
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
		{
			name:  "empty lines skipped",
			input: "\nopencode-go/glm-5.2\n\n\nopenai/gpt-5.5\n\n",
			// Spec: REQ-OC-003 — Scenario: Edge case — empty lines in output
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
			},
		},
		{
			name:  "line without slash separator skipped",
			input: "opencode-go/glm-5.2\nsome-random-text\nopenai/gpt-5.5\n",
			// Spec: REQ-OC-003 — Scenario: Edge case — line without / separator
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
			},
		},
		{
			name:  "duplicate models deduplicated last wins",
			input: "opencode-go/glm-5.2\nopencode-go/glm-5.2\n",
			// Spec: REQ-OC-003 — Scenario: Edge case — duplicate model lines
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
		{
			name:  "ANSI escape codes stripped",
			input: "\x1b[32mopencode-go/glm-5.2\x1b[0m\n",
			// Spec: REQ-OC-003 — Scenario: Edge case — ANSI escape codes in output
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
		{
			name:  "complex ANSI codes with multiple attributes",
			input: "\x1b[1;32mopencode-go/glm-5.2\x1b[0m\n\x1b[33mopenai/gpt-5.5\x1b[0m\n",
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
				{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
			},
		},
		{
			name:  "model ID with additional slashes splits on first slash",
			input: "provider/sub/model\n",
			// Spec: REQ-OC-003 — Scenario: Edge case — model ID containing additional slashes
			expected: []Model{
				{Provider: "provider", ID: "sub/model", FullName: "provider/sub/model"},
			},
		},
		{
			name:  "multiple models with sub-paths",
			input: "provider/sub/model\nother/normal-model\n",
			expected: []Model{
				{Provider: "provider", ID: "sub/model", FullName: "provider/sub/model"},
				{Provider: "other", ID: "normal-model", FullName: "other/normal-model"},
			},
		},
		{
			name:  "whitespace only lines treated as empty and skipped",
			input: "   \nopencode-go/glm-5.2\n\t\n",
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
		{
			name:  "leading whitespace on line stripped",
			input: "  opencode-go/glm-5.2\n",
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
		{
			name:  "no trailing newline",
			input: "opencode-go/glm-5.2",
			expected: []Model{
				{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseModelsOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseModelsOutput_ReturnsEmptySliceNotNil verifies that empty input
// returns an initialized empty slice, never nil.
//
// Spec: REQ-OC-003 — Scenario: Error — completely empty string input
func TestParseModelsOutput_ReturnsEmptySliceNotNil(t *testing.T) {
	result := ParseModelsOutput("")
	require.NotNil(t, result, "must never return nil — always an initialized slice")
	assert.Len(t, result, 0)
}

// TestParseModelsOutput_Fixture verifies parsing the real fixture with 53 lines.
//
// Spec: REQ-OC-003 — Scenario: Happy path — 53 models from 6 providers
func TestParseModelsOutput_Fixture(t *testing.T) {
	data, err := os.ReadFile("../../test/fixtures/models_output.txt")
	require.NoError(t, err, "fixture file must exist")

	models := ParseModelsOutput(string(data))

	require.Len(t, models, 53, "should parse exactly 53 models from the fixture")

	// Verify first model
	assert.Equal(t, "opencode", models[0].Provider)
	assert.Equal(t, "big-pickle", models[0].ID)
	assert.Equal(t, "opencode/big-pickle", models[0].FullName)

	// Verify last model
	assert.Equal(t, "zai-coding-plan", models[52].Provider)
	assert.Equal(t, "glm-5v-turbo", models[52].ID)
	assert.Equal(t, "zai-coding-plan/glm-5v-turbo", models[52].FullName)

	// Verify a known model in the middle
	assert.Equal(t, "opencode-go", models[9].Provider)
	assert.Equal(t, "glm-5.2", models[9].ID)
	assert.Equal(t, "opencode-go/glm-5.2", models[9].FullName)

	// All models should have non-empty Provider, ID, and FullName
	for i, m := range models {
		assert.NotEmpty(t, m.Provider, "model %d should have a provider", i)
		assert.NotEmpty(t, m.ID, "model %d should have an ID", i)
		assert.NotEmpty(t, m.FullName, "model %d should have a FullName", i)
		assert.Contains(t, m.FullName, "/", "model %d FullName should contain /", i)
	}
}

// TestParseModelsOutput_FixtureProviders verifies all 6 expected providers are present.
func TestParseModelsOutput_FixtureProviders(t *testing.T) {
	data, err := os.ReadFile("../../test/fixtures/models_output.txt")
	require.NoError(t, err)

	models := ParseModelsOutput(string(data))

	providers := make(map[string]bool)
	for _, m := range models {
		providers[m.Provider] = true
	}

	expectedProviders := []string{
		"opencode",
		"opencode-go",
		"minimax",
		"openai",
		"xiaomi-token-plan-sgp",
		"zai-coding-plan",
	}
	for _, p := range expectedProviders {
		assert.True(t, providers[p], "provider %q should be present in parsed models", p)
	}
	assert.Len(t, providers, 6, "should have exactly 6 distinct providers")
}

// ---------------------------------------------------------------------------
// GetModels — tests using the helper-process pattern for exec.Command mocking
// ---------------------------------------------------------------------------

// TestHelperProcess is a test binary that mimics the opencode CLI.
// It is invoked indirectly via exec.Command(os.Args[0], "-test.run=TestHelperProcess")
// and controlled by environment variables.
//
// This is the standard Go testing pattern for mocking exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_PROCESS") != "1" {
		t.Skip("skipping helper process in normal test run")
	}
	defer os.Exit(0)

	switch os.Getenv("GO_HELPER_MODE") {
	case "success":
		fmt.Print("opencode-go/glm-5.2\nopenai/gpt-5.5\n")
	case "success-53":
		data, err := os.ReadFile("../../test/fixtures/models_output.txt")
		if err != nil {
			fmt.Fprint(os.Stderr, "fixture error")
			os.Exit(1)
		}
		fmt.Print(string(data))
	case "empty":
		// produce no output at all
	case "fail":
		fmt.Fprint(os.Stderr, "command failed")
		os.Exit(1)
	}
}

// fakeCommandFactory returns a function that mimics exec.Command but creates
// commands that run the test binary in helper mode with the specified behavior.
func fakeCommandFactory(mode string) func(name string, args ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=TestHelperProcess", "--"}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command(os.Args[0], cmdArgs...)
		cmd.Env = []string{
			"GO_HELPER_PROCESS=1",
			"GO_HELPER_MODE=" + mode,
		}
		return cmd
	}
}

// TestGetModels_NotInstalled verifies GetModels returns Detect's error when
// opencode is not on PATH.
//
// Spec: REQ-OC-002 — Scenario: Error — opencode not installed
func TestGetModels_NotInstalled(t *testing.T) {
	origLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}
	defer func() { lookPath = origLookPath }()

	models, err := GetModels()

	require.Error(t, err)
	assert.Nil(t, models)
	assert.True(t, errors.Is(err, ErrOpencodeNotFound),
		"GetModels should propagate ErrOpencodeNotFound from Detect")
}

// TestGetModels_Success verifies GetModels returns parsed models when the
// command produces valid output.
//
// Spec: REQ-OC-002 — Scenario: Happy path — 53 models from 6 providers
func TestGetModels_Success(t *testing.T) {
	origLookPath := lookPath
	origCommand := command
	lookPath = func(file string) (string, error) {
		return "/fake/opencode", nil
	}
	command = fakeCommandFactory("success")
	defer func() {
		lookPath = origLookPath
		command = origCommand
	}()

	models, err := GetModels()

	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "opencode-go", models[0].Provider)
	assert.Equal(t, "glm-5.2", models[0].ID)
	assert.Equal(t, "openai", models[1].Provider)
	assert.Equal(t, "gpt-5.5", models[1].ID)
}

// TestGetModels_EmptyOutput verifies GetModels returns an empty slice (not nil)
// when the command produces no output.
//
// Spec: REQ-OC-002 — Scenario: Edge case — empty output
func TestGetModels_EmptyOutput(t *testing.T) {
	origLookPath := lookPath
	origCommand := command
	lookPath = func(file string) (string, error) {
		return "/fake/opencode", nil
	}
	command = fakeCommandFactory("empty")
	defer func() {
		lookPath = origLookPath
		command = origCommand
	}()

	models, err := GetModels()

	require.NoError(t, err)
	require.NotNil(t, models, "must return initialized empty slice, not nil")
	assert.Len(t, models, 0)
}

// TestGetModels_CommandFails verifies GetModels returns an error when the
// command exits with non-zero status.
//
// Spec: REQ-OC-002 — Scenario: Error — command fails to execute
func TestGetModels_CommandFails(t *testing.T) {
	origLookPath := lookPath
	origCommand := command
	lookPath = func(file string) (string, error) {
		return "/fake/opencode", nil
	}
	command = fakeCommandFactory("fail")
	defer func() {
		lookPath = origLookPath
		command = origCommand
	}()

	models, err := GetModels()

	require.Error(t, err, "command failure should produce an error")
	assert.Nil(t, models)
}

// ---------------------------------------------------------------------------
// Variant Descriptor and Catalog Integration Tests (catalog-001)
// ---------------------------------------------------------------------------

func TestModel_VariantsPreserveFullID(t *testing.T) {
	cfg := map[string]interface{}{
		"provider": map[string]interface{}{
			"anthropic": map[string]interface{}{
				"models": map[string]interface{}{
					"claude-sonnet-4-20250514": map[string]interface{}{
						"variants": map[string]interface{}{
							"high":   map[string]interface{}{"options": map[string]interface{}{"effort": "high"}},
							"medium": map[string]interface{}{"options": map[string]interface{}{"effort": "medium"}},
							"low":    map[string]interface{}{"options": map[string]interface{}{"effort": "low"}},
						},
					},
				},
			},
		},
	}

	discovered := []Model{
		{Provider: "anthropic", ID: "claude-sonnet-4-20250514", FullName: "anthropic/claude-sonnet-4-20250514"},
	}

	joined := JoinModelVariants(discovered, cfg)
	require.Len(t, joined, 1)

	m := joined[0]
	// Full identity MUST be preserved exactly
	assert.Equal(t, "anthropic", m.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", m.ID)
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", m.FullName)
	assert.NotContains(t, m.FullName, ":high", "FullName must never concatenate variant suffix")
	assert.NotContains(t, m.FullName, ":medium", "FullName must never concatenate variant suffix")
	assert.NotContains(t, m.FullName, ":low", "FullName must never concatenate variant suffix")

	require.Len(t, m.Variants, 3)
	// Deterministically sorted: high, low, medium
	assert.Equal(t, "high", m.Variants[0].Name)
	assert.Equal(t, "low", m.Variants[1].Name)
	assert.Equal(t, "medium", m.Variants[2].Name)
	assert.Equal(t, map[string]interface{}{"effort": "high"}, m.Variants[0].Options)
	assert.Equal(t, map[string]interface{}{"effort": "low"}, m.Variants[1].Options)
	assert.Equal(t, map[string]interface{}{"effort": "medium"}, m.Variants[2].Options)
}

func TestModel_ConfiguredVariantsSortedDeterministically(t *testing.T) {
	cfg := map[string]interface{}{
		"provider": map[string]interface{}{
			"testprov": map[string]interface{}{
				"models": map[string]interface{}{
					"testmodel": map[string]interface{}{
						"variants": map[string]interface{}{
							"zeta":   map[string]interface{}{"options": map[string]interface{}{"speed": "fast"}},
							"alpha":  map[string]interface{}{"options": map[string]interface{}{"speed": "slow"}},
							"medium": map[string]interface{}{},
							"beta":   nil,
						},
					},
				},
			},
		},
	}

	for i := 0; i < 50; i++ {
		variants := ExtractModelVariants(cfg, "testprov", "testmodel")
		require.Len(t, variants, 4)
		assert.Equal(t, "alpha", variants[0].Name)
		assert.Equal(t, "beta", variants[1].Name)
		assert.Equal(t, "medium", variants[2].Name)
		assert.Equal(t, "zeta", variants[3].Name)
	}
}

func TestModel_MissingOrMalformedConfigProducesNoSyntheticVariants(t *testing.T) {
	tests := []struct {
		name     string
		cfg      any
		provider string
		modelID  string
	}{
		{name: "nil config", cfg: nil, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "empty map config", cfg: map[string]interface{}{}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "non-map config", cfg: "invalid-string", provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "provider field is not a map", cfg: map[string]interface{}{"provider": 123}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "missing provider entry", cfg: map[string]interface{}{"provider": map[string]interface{}{"openai": map[string]interface{}{}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "provider entry is not a map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": "bad"}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "missing models map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"options": map[string]interface{}{"thinking": true}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "models field is not a map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": []string{"claude"}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "missing model entry", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": map[string]interface{}{"other-model": map[string]interface{}{}}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "missing variants map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": map[string]interface{}{"claude-sonnet-4-20250514": map[string]interface{}{}}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "variants field is not a map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": map[string]interface{}{"claude-sonnet-4-20250514": map[string]interface{}{"variants": "none"}}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "empty variants map", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": map[string]interface{}{"claude-sonnet-4-20250514": map[string]interface{}{"variants": map[string]interface{}{}}}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
		{name: "malformed variant entries skipped", cfg: map[string]interface{}{"provider": map[string]interface{}{"anthropic": map[string]interface{}{"models": map[string]interface{}{"claude-sonnet-4-20250514": map[string]interface{}{"variants": map[string]interface{}{"bad": 12345}}}}}}, provider: "anthropic", modelID: "claude-sonnet-4-20250514"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variants := ExtractModelVariants(tt.cfg, tt.provider, tt.modelID)
			assert.Nil(t, variants, "must return nil without synthesizing any variant entries")

			models := []Model{{Provider: tt.provider, ID: tt.modelID, FullName: tt.provider + "/" + tt.modelID}}
			joined := JoinModelVariants(models, tt.cfg)
			require.Len(t, joined, 1)
			assert.Nil(t, joined[0].Variants, "must not produce synthetic variant descriptors")
			assert.Equal(t, tt.provider+"/"+tt.modelID, joined[0].FullName)
		})
	}
}

func TestModel_FlatDiscoveryRemainsCompatible(t *testing.T) {
	output := "anthropic/claude-sonnet-4-20250514\nopenai/gpt-4o\n"
	models := ParseModelsOutput(output)
	require.Len(t, models, 2)

	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", models[0].FullName)
	assert.Nil(t, models[0].Variants, "flat output has nil variants")

	assert.Equal(t, "openai/gpt-4o", models[1].FullName)
	assert.Nil(t, models[1].Variants, "flat output has nil variants")
}

func TestVariant_DescriptorAndExtraction(t *testing.T) {
	cfg := map[string]interface{}{
		"anthropic": map[string]interface{}{
			"models": map[string]interface{}{
				"anthropic/claude-sonnet-4-20250514": map[string]interface{}{
					"variants": map[string]interface{}{
						"high": map[string]interface{}{
							"options": map[string]interface{}{"effort": "high"},
						},
						"default": map[string]interface{}{},
					},
				},
			},
		},
	}

	variants := ExtractVariants(cfg, "anthropic", "claude-sonnet-4-20250514")
	require.Len(t, variants, 2)
	assert.Equal(t, "default", variants[0].Name)
	assert.Nil(t, variants[0].Options)
	assert.Equal(t, "high", variants[1].Name)
	assert.Equal(t, map[string]interface{}{"effort": "high"}, variants[1].Options)
}

func TestVariant_SortVariants(t *testing.T) {
	input := []VariantDescriptor{
		{Name: "medium"},
		{Name: "high"},
		{Name: "low"},
	}
	SortVariants(input)
	assert.Equal(t, "high", input[0].Name)
	assert.Equal(t, "low", input[1].Name)
	assert.Equal(t, "medium", input[2].Name)
}

func TestParseModelsVerboseOutput(t *testing.T) {
	sampleVerbose := `
openai/gpt-5.6-sol
{
  "id": "gpt-5.6-sol",
  "providerID": "openai",
  "name": "GPT-5.6 Sol",
  "capabilities": {
    "reasoning": true,
    "toolcall": true
  },
  "variants": {
    "none": {
      "reasoningEffort": "none"
    },
    "high": {
      "reasoningEffort": "high"
    },
    "low": {
      "reasoningEffort": "low"
    }
  }
}
anthropic/claude-sonnet-4-5-20250929
{
  "id": "claude-sonnet-4-5-20250929",
  "providerID": "anthropic",
  "name": "Claude Sonnet 4.5",
  "capabilities": {
    "reasoning": true
  },
  "variants": {
    "high": {
      "thinking": {
        "type": "enabled",
        "budgetTokens": 16000
      }
    },
    "max": {
      "thinking": {
        "type": "enabled",
        "budgetTokens": 32000
      }
    }
  }
}
`
	models := ParseModelsVerboseOutput(sampleVerbose)
	require.Len(t, models, 2)

	assert.Equal(t, "openai", models[0].Provider)
	assert.Equal(t, "gpt-5.6-sol", models[0].ID)
	assert.Equal(t, "openai/gpt-5.6-sol", models[0].FullName)
	require.Len(t, models[0].Variants, 3)
	assert.Equal(t, "high", models[0].Variants[0].Name)
	assert.Equal(t, map[string]interface{}{"reasoningEffort": "high"}, models[0].Variants[0].Options)
	assert.Equal(t, "low", models[0].Variants[1].Name)
	assert.Equal(t, "none", models[0].Variants[2].Name)

	assert.Equal(t, "anthropic", models[1].Provider)
	assert.Equal(t, "claude-sonnet-4-5-20250929", models[1].ID)
	assert.Equal(t, "anthropic/claude-sonnet-4-5-20250929", models[1].FullName)
	require.Len(t, models[1].Variants, 2)
	assert.Equal(t, "high", models[1].Variants[0].Name)
	assert.Equal(t, "max", models[1].Variants[1].Name)
}

func TestJoinModelVariants_MergesRuntimeAndCustomVariants(t *testing.T) {
	runtimeModels := []Model{
		{
			Provider: "openai",
			ID:       "gpt-5",
			FullName: "openai/gpt-5",
			Variants: []VariantDescriptor{
				{Name: "low", Options: map[string]interface{}{"reasoningEffort": "low"}},
				{Name: "high", Options: map[string]interface{}{"reasoningEffort": "high"}},
			},
		},
	}

	customCfg := map[string]interface{}{
		"provider": map[string]interface{}{
			"openai": map[string]interface{}{
				"models": map[string]interface{}{
					"gpt-5": map[string]interface{}{
						"variants": map[string]interface{}{
							// Custom overrides runtime "high" with textVerbosity
							"high": map[string]interface{}{"reasoningEffort": "high", "textVerbosity": "low"},
							// Custom adds new "thinking" variant
							"thinking": map[string]interface{}{"reasoningEffort": "xhigh"},
						},
					},
				},
			},
		},
	}

	joined := JoinModelVariants(runtimeModels, customCfg)
	require.Len(t, joined, 1)
	m := joined[0]

	require.Len(t, m.Variants, 3)
	// Sorted: high, low, thinking
	assert.Equal(t, "high", m.Variants[0].Name)
	assert.Equal(t, "low", m.Variants[1].Name)
	assert.Equal(t, "thinking", m.Variants[2].Name)

	// Custom "high" overrides runtime "high"
	assert.Equal(t, map[string]interface{}{"reasoningEffort": "high", "textVerbosity": "low"}, m.Variants[0].Options)
	// Runtime "low" preserved
	assert.Equal(t, map[string]interface{}{"reasoningEffort": "low"}, m.Variants[1].Options)
	// Custom "thinking" added
	assert.Equal(t, map[string]interface{}{"reasoningEffort": "xhigh"}, m.Variants[2].Options)
}
