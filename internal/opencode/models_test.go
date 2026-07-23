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
