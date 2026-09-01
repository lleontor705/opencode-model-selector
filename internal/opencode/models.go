package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// ErrParseFailed is a sentinel error for parse failures in opencode model
// output. Kept for API completeness — ParseModelsOutput itself does not error
// (it returns an empty slice for invalid input), but GetModels may wrap it.
var ErrParseFailed = errors.New("failed to parse opencode models output")

// VariantDescriptor describes a named variant configuration for a model.
type VariantDescriptor struct {
	Name    string                 `json:"name"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Variant is an alias for VariantDescriptor.
type Variant = VariantDescriptor

// Model represents a single model returned by the opencode CLI or joined
// with configured variant descriptors.
//
//   - Provider: the provider prefix (e.g. "opencode-go", "openai")
//   - ID:       the model identifier after the first "/" (e.g. "glm-5.2")
//   - FullName: the complete "provider/id" string (e.g. "opencode-go/glm-5.2")
//   - Variants: optional configured variant descriptors for this model
type Model struct {
	Provider string              `json:"provider"`
	ID       string              `json:"id"`
	FullName string              `json:"full_name"`
	Variants []VariantDescriptor `json:"variants,omitempty"`
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

	// Try verbose model listing first to discover native runtime variants and capabilities
	cmdVerbose := command(path, "models", "--verbose")
	if output, err := cmdVerbose.Output(); err == nil && len(output) > 0 {
		models := ParseModelsVerboseOutput(string(output))
		if len(models) > 0 {
			return models, nil
		}
	}

	// Fallback to flat "opencode models" output
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

type verboseModelJSON struct {
	ID           string                            `json:"id"`
	ProviderID   string                            `json:"providerID"`
	Name         string                            `json:"name"`
	Variants     map[string]map[string]interface{} `json:"variants"`
	Capabilities map[string]interface{}            `json:"capabilities"`
	Options      map[string]interface{}            `json:"options"`
}

// ParseModelsVerboseOutput parses structured stdout text from "opencode models --verbose".
// It extracts model identities, native capabilities, and built-in variants.
func ParseModelsVerboseOutput(output string) []Model {
	if output == "" {
		return []Model{}
	}

	cleaned := ansiRegex.ReplaceAllString(output, "")
	lines := strings.Split(cleaned, "\n")

	seen := make(map[string]int)
	var models []Model

	var currentFullName string
	var jsonLines []string
	inJSON := false
	braceCount := 0

	flushModel := func() {
		if currentFullName == "" {
			return
		}

		idx := strings.Index(currentFullName, "/")
		if idx == -1 {
			currentFullName = ""
			jsonLines = nil
			inJSON = false
			braceCount = 0
			return
		}

		provider := currentFullName[:idx]
		modelID := currentFullName[idx+1:]

		var variants []VariantDescriptor
		if len(jsonLines) > 0 {
			jsonStr := strings.Join(jsonLines, "\n")
			var vData verboseModelJSON
			if err := json.Unmarshal([]byte(jsonStr), &vData); err == nil {
				if vData.ProviderID != "" {
					provider = vData.ProviderID
				}
				if vData.ID != "" {
					modelID = vData.ID
				}
				for vName, vOpts := range vData.Variants {
					if strings.TrimSpace(vName) == "" {
						continue
					}
					var desc VariantDescriptor
					desc.Name = vName
					if len(vOpts) > 0 {
						optsCopy := make(map[string]interface{}, len(vOpts))
						for k, v := range vOpts {
							optsCopy[k] = v
						}
						desc.Options = optsCopy
					}
					variants = append(variants, desc)
				}
				SortVariants(variants)
			}
		}

		m := Model{
			Provider: provider,
			ID:       modelID,
			FullName: currentFullName,
			Variants: variants,
		}

		if existingIdx, exists := seen[m.FullName]; exists {
			models[existingIdx] = m
		} else {
			seen[m.FullName] = len(models)
			models = append(models, m)
		}

		currentFullName = ""
		jsonLines = nil
		inJSON = false
		braceCount = 0
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if !inJSON {
			if strings.HasPrefix(trimmed, "{") {
				inJSON = true
				braceCount = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				jsonLines = append(jsonLines, line)
				if braceCount <= 0 {
					flushModel()
				}
			} else if strings.Contains(trimmed, "/") && !strings.Contains(trimmed, " ") {
				if currentFullName != "" {
					flushModel()
				}
				currentFullName = trimmed
			}
		} else {
			jsonLines = append(jsonLines, line)
			braceCount += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if braceCount <= 0 {
				flushModel()
			}
		}
	}

	if currentFullName != "" {
		flushModel()
	}

	if models == nil {
		return []Model{}
	}
	return models
}

// SortVariants sorts variant descriptors deterministically in ascending order by Name.
func SortVariants(variants []VariantDescriptor) {
	slices.SortFunc(variants, func(a, b VariantDescriptor) int {
		return strings.Compare(a.Name, b.Name)
	})
}

// ExtractModelVariants extracts and deterministically sorts variant descriptors
// for the given provider and modelID from configuration data (such as loaded config
// or a provider configuration map).
//
// If providerConfig is nil, not a map, or does not contain valid variants for the
// requested model, it returns nil without creating any synthetic entries.
func ExtractModelVariants(providerConfig any, provider, modelID string) []VariantDescriptor {
	if providerConfig == nil || provider == "" || modelID == "" {
		return nil
	}

	rawMap, ok := providerConfig.(map[string]interface{})
	if !ok {
		return nil
	}

	// Drill into "provider" top-level block if present
	var provMap map[string]interface{}
	if nested, exists := rawMap["provider"]; exists {
		if nm, ok := nested.(map[string]interface{}); ok {
			provMap = nm
		} else {
			return nil
		}
	} else {
		provMap = rawMap
	}

	provEntryRaw, exists := provMap[provider]
	if !exists || provEntryRaw == nil {
		return nil
	}
	provEntry, ok := provEntryRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	modelsRaw, exists := provEntry["models"]
	if !exists || modelsRaw == nil {
		return nil
	}
	modelsMap, ok := modelsRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	// Look up by modelID or provider/modelID
	var modelData map[string]interface{}
	if entryRaw, exists := modelsMap[modelID]; exists && entryRaw != nil {
		if entryMap, ok := entryRaw.(map[string]interface{}); ok {
			modelData = entryMap
		}
	}
	if modelData == nil {
		fullName := provider + "/" + modelID
		if entryRaw, exists := modelsMap[fullName]; exists && entryRaw != nil {
			if entryMap, ok := entryRaw.(map[string]interface{}); ok {
				modelData = entryMap
			}
		}
	}
	if modelData == nil {
		return nil
	}

	variantsRaw, exists := modelData["variants"]
	if !exists || variantsRaw == nil {
		return nil
	}
	variantsMap, ok := variantsRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	if len(variantsMap) == 0 {
		return nil
	}

	var variants []VariantDescriptor
	for varName, varRaw := range variantsMap {
		if strings.TrimSpace(varName) == "" {
			continue
		}
		var desc VariantDescriptor
		desc.Name = varName

		if varMap, ok := varRaw.(map[string]interface{}); ok {
			if optsRaw, ok := varMap["options"].(map[string]interface{}); ok && len(optsRaw) > 0 {
				optsCopy := make(map[string]interface{}, len(optsRaw))
				for k, v := range optsRaw {
					optsCopy[k] = v
				}
				desc.Options = optsCopy
			} else if len(varMap) > 0 {
				optsCopy := make(map[string]interface{}, len(varMap))
				for k, v := range varMap {
					optsCopy[k] = v
				}
				desc.Options = optsCopy
			}
		} else if varRaw == nil {
			// Variant with no options
		} else {
			// Malformed entry - skip so no synthetic/corrupt entries are produced
			continue
		}
		variants = append(variants, desc)
	}

	if len(variants) == 0 {
		return nil
	}

	SortVariants(variants)
	return variants
}

// ExtractVariants is an alias for ExtractModelVariants.
func ExtractVariants(providerConfig any, provider, modelID string) []VariantDescriptor {
	return ExtractModelVariants(providerConfig, provider, modelID)
}

// JoinModelVariants joins discovered models (which may include native runtime variants)
// with configured variant descriptors extracted from provider configuration data.
// Custom configured variants take precedence over runtime variants with the same name.
// It returns a new slice of Model instances with full identities preserved.
func JoinModelVariants(models []Model, providerConfig any) []Model {
	if len(models) == 0 {
		return []Model{}
	}
	result := make([]Model, len(models))
	for i, m := range models {
		customVariants := ExtractModelVariants(providerConfig, m.Provider, m.ID)
		if len(customVariants) == 0 {
			result[i] = m
			continue
		}

		variantMap := make(map[string]VariantDescriptor)
		for _, v := range m.Variants {
			variantMap[v.Name] = v
		}
		for _, cv := range customVariants {
			variantMap[cv.Name] = cv // custom config wins
		}

		merged := make([]VariantDescriptor, 0, len(variantMap))
		for _, v := range variantMap {
			merged = append(merged, v)
		}
		SortVariants(merged)

		result[i] = Model{
			Provider: m.Provider,
			ID:       m.ID,
			FullName: m.FullName,
			Variants: merged,
		}
	}
	return result
}
