package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// MergeAgents (T-B4)
//
// Per-field precedence (highest non-empty wins): global md < project md <
// inline-JSON agent. The merged body/prompt uses the markdown body (project
// over global), overridden by an inline-JSON "prompt" when present. Unknown
// markdown frontmatter fields are carried into Raw. The result is a unified
// MergedAgent map that GetAgents can return alongside JSON agents.
// ---------------------------------------------------------------------------

// mdLayer builds a single-entry markdown layer for compact test setup.
func mdLayer(name, body string, raw map[string]interface{}) map[string]MDAgent {
	return map[string]MDAgent{name: {Name: name, Body: body, Raw: raw}}
}

// asJSONObj is a compact map[string]interface{} literal helper.
func asJSONObj(kvs ...interface{}) map[string]interface{} {
	m := map[string]interface{}{}
	for i := 0; i+1 < len(kvs); i += 2 {
		m[kvs[i].(string)] = kvs[i+1]
	}
	return m
}

// TestMerge_PrecedenceGlobalProjectJSON verifies that for a recognized field,
// precedence is inline-JSON > project md > global md (highest non-empty wins).
func TestMerge_PrecedenceGlobalProjectJSON(t *testing.T) {
	global := mdLayer("build", "", asJSONObj("model", "global-model"))
	project := mdLayer("build", "", asJSONObj("model", "project-model"))
	jsonAgents := map[string]interface{}{
		"build": asJSONObj("model", "json-model"),
	}

	merged := MergeAgents(global, project, jsonAgents)
	m := merged["build"]
	require.NotNil(t, m)
	assert.Equal(t, "json-model", m.Fields["model"],
		"inline-JSON model must win over both md layers")

	// JSON absent -> project wins.
	merged = MergeAgents(global, project, nil)
	assert.Equal(t, "project-model", merged["build"].Fields["model"],
		"project md model must win when JSON is absent")

	// Project field absent -> global fills it.
	merged = MergeAgents(global, nil, nil)
	assert.Equal(t, "global-model", merged["build"].Fields["model"],
		"global md model must be used when project and JSON are absent")
}

// TestMerge_JSONPromptOverridesMDBody verifies that an inline-JSON "prompt"
// string overrides the merged markdown body, and that without a JSON prompt the
// merged md body (project over global) is used.
func TestMerge_JSONPromptOverridesMDBody(t *testing.T) {
	global := mdLayer("a", "global body", nil)
	project := mdLayer("a", "project body", nil)

	// No JSON -> project body over global body.
	merged := MergeAgents(global, project, nil)
	assert.Equal(t, "project body", merged["a"].Prompt,
		"project md body must override global md body")

	// JSON prompt present -> overrides md body.
	jsonAgents := map[string]interface{}{
		"a": asJSONObj("prompt", "json prompt"),
	}
	merged = MergeAgents(global, project, jsonAgents)
	assert.Equal(t, "json prompt", merged["a"].Prompt,
		"inline-JSON prompt must override the merged md body")

	// Project body empty -> global body used.
	merged = MergeAgents(global, nil, nil)
	assert.Equal(t, "global body", merged["a"].Prompt,
		"global md body fills when project body is empty")
}

// TestMerge_GlobalFillsAbsentProjectField verifies that a field present in
// global md but absent in project md is carried into the merged result.
func TestMerge_GlobalFillsAbsentProjectField(t *testing.T) {
	global := mdLayer("a", "", asJSONObj("description", "global desc"))
	project := mdLayer("a", "", asJSONObj("model", "pm")) // no description

	merged := MergeAgents(global, project, nil)
	m := merged["a"]
	require.NotNil(t, m)
	assert.Equal(t, "global desc", m.Fields["description"],
		"global description must fill the absent project field")
	assert.Equal(t, "pm", m.Fields["model"],
		"project model must still win where present")
}

// TestMerge_PreservesRawUnknownFields verifies that unknown markdown frontmatter
// fields (e.g. steps, color) are carried into Raw, with project over global.
func TestMerge_PreservesRawUnknownFields(t *testing.T) {
	global := mdLayer("a", "", asJSONObj("steps", 1, "color", "red"))
	project := mdLayer("a", "", asJSONObj("color", "blue", "custom", "x"))

	merged := MergeAgents(global, project, nil)
	m := merged["a"]
	require.NotNil(t, m)
	assert.Equal(t, 1, m.Raw["steps"], "global-only unknown field must survive")
	assert.Equal(t, "blue", m.Raw["color"], "project unknown field must override global")
	assert.Equal(t, "x", m.Raw["custom"], "project-only unknown field must survive")
}

// TestMerge_MdOnlyFlag verifies that an agent with no inline-JSON backing is
// flagged MdOnly, and a JSON-backed agent is not.
func TestMerge_MdOnlyFlag(t *testing.T) {
	global := mdLayer("mdonly", "", asJSONObj("model", "m"))
	jsonAgents := map[string]interface{}{
		"hybrid": asJSONObj("model", "j"),
	}

	merged := MergeAgents(global, nil, jsonAgents)
	assert.True(t, merged["mdonly"].MdOnly, "pure-markdown agent must be MdOnly")
	assert.False(t, merged["mdonly"].HasInlineJSON)
	assert.False(t, merged["hybrid"].MdOnly, "JSON-backed agent must not be MdOnly")
	assert.True(t, merged["hybrid"].HasInlineJSON)
}

// TestMerge_ModeResolution verifies the merged Mode accessor reflects precedence
// and defaults to "all" when absent everywhere.
func TestMerge_ModeResolution(t *testing.T) {
	// global primary, project none -> primary.
	merged := MergeAgents(mdLayer("a", "", asJSONObj("mode", "primary")), nil, nil)
	assert.Equal(t, "primary", merged["a"].Mode())

	// JSON subagent overrides md primary.
	jsonAgents := map[string]interface{}{"a": asJSONObj("mode", "subagent")}
	merged = MergeAgents(mdLayer("a", "", asJSONObj("mode", "primary")), nil, jsonAgents)
	assert.Equal(t, "subagent", merged["a"].Mode(),
		"JSON mode must override md mode")

	// No mode anywhere -> default "all".
	merged = MergeAgents(mdLayer("a", "", nil), nil, nil)
	assert.Equal(t, "all", merged["a"].Mode())
}

// TestMerge_SystemAgentsIncludedButUnfiltered verifies the merger does not
// itself drop system agents (filtering is GetAgents' job), so a md file named
// e.g. "summary.md" still appears.
func TestMerge_SystemAgentsIncludedButUnfiltered(t *testing.T) {
	global := mdLayer("summary", "", asJSONObj("model", "m"))
	merged := MergeAgents(global, nil, nil)
	assert.Contains(t, merged, "summary",
		"merger keeps all names; GetAgents filters system agents")
}

// TestMerge_DisableFromJSON verifies the Disabled accessor reads the JSON-only
// disable flag (markdown has no disable field).
func TestMerge_DisableFromJSON(t *testing.T) {
	jsonAgents := map[string]interface{}{
		"a": asJSONObj("disable", true),
	}
	merged := MergeAgents(nil, nil, jsonAgents)
	assert.True(t, merged["a"].Disabled(), "disable must come from JSON layer")

	merged = MergeAgents(mdLayer("a", "", nil), nil, nil)
	assert.False(t, merged["a"].Disabled(), "md-only agent defaults to not disabled")
}

// ---------------------------------------------------------------------------
// Model / Variant / Options independent merge and resolution (mem-001)
// ---------------------------------------------------------------------------

func TestMerge_ModelVariantIndependentPrecedence(t *testing.T) {
	// Scenario 1: Project MD has model, Global MD has variant, no JSON
	global1 := mdLayer("agent1", "", asJSONObj("variant", "global-high", "model", "global-model"))
	project1 := mdLayer("agent1", "", asJSONObj("model", "project-model")) // no variant

	merged1 := MergeAgents(global1, project1, nil)
	m1 := merged1["agent1"]
	require.NotNil(t, m1)
	assert.Equal(t, "project-model", m1.Fields["model"], "project model must override global model")
	assert.Equal(t, "global-high", m1.Fields["variant"], "global variant must fill absent project variant")

	// Scenario 2: Inherited MD model, inline JSON sets variant only
	json2 := map[string]interface{}{
		"agent1": asJSONObj("variant", "json-medium"),
	}
	merged2 := MergeAgents(global1, project1, json2)
	m2 := merged2["agent1"]
	require.NotNil(t, m2)
	assert.Equal(t, "project-model", m2.Fields["model"], "inherited project model remains effective")
	assert.Equal(t, "json-medium", m2.Fields["variant"], "inline JSON variant wins over MD variant")

	// Scenario 3: Global MD has variant, inline JSON sets model only
	json3 := map[string]interface{}{
		"agent1": asJSONObj("model", "json-model"),
	}
	merged3 := MergeAgents(global1, nil, json3)
	m3 := merged3["agent1"]
	require.NotNil(t, m3)
	assert.Equal(t, "json-model", m3.Fields["model"], "inline JSON model wins")
	assert.Equal(t, "global-high", m3.Fields["variant"], "global MD variant remains effective")

	// Scenario 4: Options layer merge
	globalOpt := mdLayer("agent2", "", asJSONObj("options", map[string]interface{}{"effort": "low", "temp": 0.5}))
	projectOpt := mdLayer("agent2", "", asJSONObj("options", map[string]interface{}{"effort": "medium"}))
	jsonOpt := map[string]interface{}{
		"agent2": asJSONObj("options", map[string]interface{}{"effort": "high"}),
	}
	mergedOpt := MergeAgents(globalOpt, projectOpt, jsonOpt)
	mOpt := mergedOpt["agent2"]
	require.NotNil(t, mOpt)
	assert.Equal(t, map[string]interface{}{"effort": "high"}, mOpt.Fields["options"],
		"inline JSON options override lower layers")
}

func TestMerge_ModelVariantUnknownKeysPreserved(t *testing.T) {
	global := mdLayer("custom", "", asJSONObj("custom_md_field", "global_val", "variant", "low"))
	project := mdLayer("custom", "", asJSONObj("custom_proj_field", "proj_val", "model", "pm"))
	jsonAgents := map[string]interface{}{
		"custom": asJSONObj("custom_json_field", 12345),
	}

	merged := MergeAgents(global, project, jsonAgents)
	m := merged["custom"]
	require.NotNil(t, m)

	assert.Equal(t, "pm", m.Fields["model"])
	assert.Equal(t, "low", m.Fields["variant"])
	assert.Equal(t, "global_val", m.Fields["custom_md_field"])
	assert.Equal(t, "proj_val", m.Fields["custom_proj_field"])
	assert.Equal(t, 12345, m.Fields["custom_json_field"])
}

func TestResolveModel_IndependentOfVariant(t *testing.T) {
	globalMD := map[string]MDAgent{
		"worker": {Name: "worker", Raw: map[string]interface{}{"model": "global/model", "variant": "high"}},
	}
	projectMD := map[string]MDAgent{
		"worker": {Name: "worker", Raw: map[string]interface{}{"variant": "medium"}},
	}
	inlineJSON := map[string]interface{}{
		"worker": map[string]interface{}{"variant": "low"},
	}

	// ResolveModel returns clean model ID without variant pollution
	model, prov, ok := ResolveModel("worker", globalMD, projectMD, inlineJSON, "fallback/model")
	require.True(t, ok)
	assert.Equal(t, "global/model", model)
	assert.Equal(t, ModelProvenanceGlobalMarkdown, prov)
	assert.NotContains(t, model, ":high")
	assert.NotContains(t, model, ":medium")
	assert.NotContains(t, model, ":low")
}
