package config

import "github.com/lleontor705/opencode-model-selector/internal/opencode"

// ValidateModel checks if a model ID exists in the available models list.
//
// Uses exact, case-sensitive match on the FullName field of each Model.
// Returns true only if modelID exactly equals the FullName of some entry in
// available; returns false for partial matches, case-insensitive matches,
// empty inputs, or an empty available list (REQ-CFG-012).
func ValidateModel(modelID string, available []opencode.Model) bool {
	if modelID == "" || len(available) == 0 {
		return false
	}

	for _, m := range available {
		if m.FullName == modelID {
			return true
		}
	}

	return false
}
