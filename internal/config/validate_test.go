package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"opencode-model-selector/internal/opencode"
)

// ---------------------------------------------------------------------------
// ValidateModel — REQ-CFG-012
//
// ValidateModel checks if a model ID exists in the available models list.
// Uses exact, case-sensitive match on the FullName field.
// ---------------------------------------------------------------------------

func TestValidateModel(t *testing.T) {
	available := []opencode.Model{
		{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
		{Provider: "openai", ID: "gpt-4o", FullName: "openai/gpt-4o"},
		{Provider: "anthropic", ID: "claude-3.5-sonnet", FullName: "anthropic/claude-3.5-sonnet"},
	}

	tests := []struct {
		name     string
		modelID  string
		available []opencode.Model
		want     bool
	}{
		{
			name:     "model in available list returns true",
			modelID:  "opencode-go/glm-5.2",
			available: available,
			want:     true,
		},
		{
			name:     "model not in available list returns false",
			modelID:  "fake/model",
			available: available,
			want:     false,
		},
		{
			name:     "different case returns false (case-sensitive)",
			modelID:  "OpenCode-Go/GLM-5.2",
			available: available,
			want:     false,
		},
		{
			name:     "empty available list returns false",
			modelID:  "any/model",
			available: []opencode.Model{},
			want:     false,
		},
		{
			name:     "empty modelID returns false",
			modelID:  "",
			available: available,
			want:     false,
		},
		{
			name:     "partial match without provider prefix returns false",
			modelID:  "glm-5.2",
			available: available,
			want:     false,
		},
		{
			name:     "match by FullName field returns true",
			modelID:  "anthropic/claude-3.5-sonnet",
			available: available,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateModel(tt.modelID, tt.available)
			assert.Equal(t, tt.want, got)
		})
	}
}
