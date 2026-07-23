package opencode

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ErrParseFailed is a sentinel error for parse failures in opencode model
// output. Kept for API completeness — ParseModelsOutput itself does not error
// (it returns an empty slice for invalid input), but GetModels may wrap it.
var ErrParseFailed = errors.New("failed to parse opencode models output")

// Model represents a single model returned by the opencode CLI.
//
//   - Provider: the provider prefix (e.g. "opencode-go", "openai")
//   - ID:       the model identifier after the first "/" (e.g. "glm-5.2")
//   - FullName: the complete "provider/id" string (e.g. "opencode-go/glm-5.2")
type Model struct {
	Provider string
	ID       string
	FullName string
}

// command is a package-level indirection over exec.Command to enable
// deterministic testing via the helper-process pattern.
var command = exec.Command

// ansiRegex matches ANSI/VT100 escape sequences (CSI) and removes them
// defensively before line parsing.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// GetModels detects the opencode binary and executes "opencode models",
// returning the parsed list of available models. If opencode is not installed,
// the detection error is propagated. If the command fails to execute or exits
// with a non-zero status, the underlying error is returned.
func GetModels() ([]Model, error) {
	path, err := Detect()
	if err != nil {
		return nil, err
	}

	cmd := command(path, "models")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode models command failed: %w", err)
	}

	return ParseModelsOutput(string(output)), nil
}

// ParseModelsOutput parses raw stdout text from "opencode models" into a slice
// of Model structs. It implements the parsing rules from REQ-OC-003:
//
//   - ANSI escape codes are stripped defensively before parsing
//   - Each line is trimmed of leading/trailing whitespace
//   - Empty lines and lines without a "/" separator are skipped
//   - Lines are split on the FIRST "/" only (IDs may contain additional slashes)
//   - Duplicate lines are deduplicated (last occurrence wins)
//   - An empty input string returns an initialized empty slice (not nil)
func ParseModelsOutput(output string) []Model {
	if output == "" {
		return []Model{}
	}

	// Strip ANSI escape codes defensively
	cleaned := ansiRegex.ReplaceAllString(output, "")

	lines := strings.Split(cleaned, "\n")

	// Dedup tracking: FullName -> index in the models slice (last wins on overwrite)
	seen := make(map[string]int)
	var models []Model

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.Index(line, "/")
		if idx == -1 {
			continue // skip lines without "/" separator
		}

		model := Model{
			Provider: line[:idx],
			ID:       line[idx+1:],
			FullName: line,
		}

		if existingIdx, exists := seen[model.FullName]; exists {
			models[existingIdx] = model // last occurrence wins
		} else {
			seen[model.FullName] = len(models)
			models = append(models, model)
		}
	}

	if models == nil {
		return []Model{}
	}
	return models
}
