// Package tui tests — variant selection screen (tui-001).
//
// Spec coverage:
//   - Requirement: independent compatible selection
//   - Scenario: model then variant
//   - Scenario: model-only fallback
//   - Scenario: cancel does not mutate
//   - Acceptance: Variant choices limited to selected model
//   - Acceptance: Model-only fallback unchanged
//   - Acceptance: Escape non-mutating
//   - Acceptance: Preview separates model and variant
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

func sampleGroupedWithVariants() map[string][]opencode.Model {
	return map[string][]opencode.Model{
		"anthropic": {
			{
				Provider: "anthropic",
				ID:       "claude-sonnet-4-20250514",
				FullName: "anthropic/claude-sonnet-4-20250514",
				Variants: []opencode.VariantDescriptor{
					{Name: "high", Options: map[string]interface{}{"effort": "high"}},
					{Name: "medium", Options: map[string]interface{}{"effort": "medium"}},
					{Name: "low", Options: map[string]interface{}{"effort": "low"}},
				},
			},
		},
		"openai": {
			{
				Provider: "openai",
				ID:       "gpt-4o",
				FullName: "openai/gpt-4o",
				Variants: []opencode.VariantDescriptor{
					{Name: "fast"},
					{Name: "deep"},
				},
			},
		},
		"opencode-go": {
			{
				Provider: "opencode-go",
				ID:       "glm-5.2",
				FullName: "opencode-go/glm-5.2",
			},
		},
	}
}

func newTestModelSelectWithVariants(t *testing.T, fieldEditing, agentName string) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), sampleGroupedWithVariants(), 5)
	m.state = ScreenModelSelection
	m.navigationStack = []appState{ScreenAgentList}
	m.fieldEditing = fieldEditing
	m.selectedAgent = agentName
	initModelSelectionScreen(&m)
	return m
}

func TestVariantSelection_ChoicesLimitedToSelectedModel(t *testing.T) {
	m := newTestModelSelectWithVariants(t, "code-reviewer", "code-reviewer")

	// Find the anthropic model in filteredModels
	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, anthropicIdx, 0, "anthropic model must be found")
	m.modelCursor = anthropicIdx

	// Press ENTER to select the model -> should transition to ScreenVariantSelection
	updated, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenVariantSelection, updated.state, "must transition to ScreenVariantSelection")
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", updated.pendingSelectedModel.FullName)

	// Verify choices are only the variants of the selected model (high, low, medium)
	require.Len(t, updated.availableVariants, 3)
	assert.Equal(t, "high", updated.availableVariants[0].Name)
	assert.Equal(t, "low", updated.availableVariants[1].Name)
	assert.Equal(t, "medium", updated.availableVariants[2].Name)

	// Verify rendered output shows only these variants, not variants from openai (fast, deep)
	out := ansi.Strip(viewVariantSelection(updated))
	assert.Contains(t, out, "high")
	assert.Contains(t, out, "medium")
	assert.Contains(t, out, "low")
	assert.NotContains(t, out, "fast")
	assert.NotContains(t, out, "deep")
}

func TestVariantSelection_ModelOnlyFallbackUnchanged(t *testing.T) {
	m := newTestModelSelectWithVariants(t, "code-reviewer", "code-reviewer")

	// Find the opencode-go model (no variants)
	glmIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "opencode-go/glm-5.2" {
			glmIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, glmIdx, 0)
	m.modelCursor = glmIdx

	// Press ENTER to select the model -> model-only flow immediately returns to ScreenAgentList
	updated, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, updated.state, "model-only flow must return to ScreenAgentList immediately")
	assert.True(t, updated.dirty, "must be marked dirty")

	val, ok := updated.config.GetAgentModelOverride("code-reviewer")
	require.True(t, ok)
	assert.Equal(t, "opencode-go/glm-5.2", val)

	// Variant field in config must not be created or mutated
	_, varOk := updated.config.GetAgentField("code-reviewer", "variant")
	assert.False(t, varOk, "no variant field should be created for model-only fallback")
}

func TestVariantSelection_EscapeNonMutating(t *testing.T) {
	m := newTestModelSelectWithVariants(t, "code-reviewer", "code-reviewer")

	// Select anthropic model to enter variant selection
	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, anthropicIdx, 0)
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ScreenVariantSelection, inVariant.state)

	// Press ESC in variant picker
	canceled, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.Equal(t, ScreenModelSelection, canceled.state, "ESC must return to ScreenModelSelection")
	assert.False(t, canceled.dirty, "ESC must not mutate dirty state")
	assert.Empty(t, canceled.changes, "ESC must not record changes")

	// Config must remain unmodified (preserves initial inline override)
	val, ok := canceled.config.GetAgentModelOverride("code-reviewer")
	assert.True(t, ok, "initial model override must be preserved on ESC")
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", val)
}

func TestVariantSelection_EnterAppliesModelAndVariant(t *testing.T) {
	m := newTestModelSelectWithVariants(t, "code-reviewer", "code-reviewer")

	// Select anthropic model
	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ScreenVariantSelection, inVariant.state)

	// Cursor 2 is "medium" (sorted: high, low, medium)
	inVariant.variantCursor = 2
	require.Equal(t, "medium", inVariant.availableVariants[2].Name)

	// Press ENTER to confirm variant
	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, committed.state, "confirming variant must return to ScreenAgentList")
	assert.True(t, committed.dirty, "must be marked dirty")

	// Config has model and variant overrides set
	modVal, ok := committed.config.GetAgentModelOverride("code-reviewer")
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", modVal)

	varVal, ok := committed.config.GetAgentField("code-reviewer", "variant")
	require.True(t, ok)
	assert.Equal(t, "medium", varVal)

	// Changes contains both model and variant
	require.Len(t, committed.changes, 1)
	assert.Equal(t, "code-reviewer", committed.changes[0].Target)
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", committed.changes[0].NewModel)
	assert.Equal(t, "medium", committed.changes[0].NewVariant)
}

func TestVariantSelection_PreviewSeparatesModelAndVariant(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGroupedWithVariants(), 5)
	m.changes = []Change{
		{
			Target:     "code-reviewer",
			OldModel:   "anthropic/claude-sonnet-4-20250514",
			NewModel:   "openai/gpt-4o",
			OldVariant: "high",
			NewVariant: "fast",
		},
		{
			Target:   "plan",
			OldModel: "old/plan",
			NewModel: "new/plan",
		},
	}

	content := saveReviewContent(m)
	assert.Contains(t, content, "code-reviewer.model: anthropic/claude-sonnet-4-20250514 -> openai/gpt-4o")
	assert.Contains(t, content, "code-reviewer.variant: high -> fast")
	assert.Contains(t, content, "plan.model: old/plan -> new/plan")
	assert.NotContains(t, content, "plan.variant")
}

func TestVariantSelection_CurrentVariantBadge(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentModelOverride("code-reviewer", "anthropic/claude-sonnet-4-20250514"))
	require.NoError(t, cfg.SetAgentVariantOverride("code-reviewer", "medium"))

	m := NewModel(cfg, sampleGroupedWithVariants(), 5)
	m.state = ScreenVariantSelection
	m.selectedAgent = "code-reviewer"
	initVariantSelectionScreen(&m, sampleGroupedWithVariants()["anthropic"][0])

	out := ansi.Strip(viewVariantSelection(m))
	lines := strings.Split(out, "\n")
	foundBadge := false
	for _, line := range lines {
		if strings.Contains(line, "medium") && strings.Contains(line, "★ current") {
			foundBadge = true
		}
	}
	assert.True(t, foundBadge, "the current variant 'medium' MUST display the '★ current' badge")
}

func TestVariantSelection_Navigation(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGroupedWithVariants(), 5)
	m.state = ScreenVariantSelection
	m.selectedAgent = "code-reviewer"
	initVariantSelectionScreen(&m, sampleGroupedWithVariants()["anthropic"][0])
	require.Len(t, m.availableVariants, 3)

	assert.Equal(t, 0, m.variantCursor)

	// Down
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, m.variantCursor)

	// j
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Text: "j"})
	assert.Equal(t, 2, m.variantCursor)

	// Clamped at bottom
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 2, m.variantCursor)

	// Up
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 1, m.variantCursor)

	// k
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Text: "k"})
	assert.Equal(t, 0, m.variantCursor)

	// Clamped at top
	m, _ = updateVariantSelection(m, tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, m.variantCursor)
}

func TestVariantSelection_GlobalFlowModelOnlyFallback(t *testing.T) {
	m := newTestModelSelectWithVariants(t, "global", "")

	// Select anthropic model (has variants) for global
	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, anthropicIdx, 0)
	m.modelCursor = anthropicIdx

	oldGlobal, _ := m.config.GetGlobalModel()

	updated, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenVariantSelection, updated.state, "global with variants must transition to ScreenVariantSelection")
	assert.Equal(t, "anthropic/claude-sonnet-4-20250514", updated.pendingSelectedModel.FullName)
	assert.Equal(t, "global", updated.fieldEditing)
	assert.False(t, updated.dirty, "entering variant selection must not mark dirty yet")

	currentGlobal, _ := updated.config.GetGlobalModel()
	assert.Equal(t, oldGlobal, currentGlobal, "global model must not be persisted before variant selection")
}

func TestVariantSelection_ResponsiveLayout(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGroupedWithVariants(), 5)
	m.state = ScreenVariantSelection
	m.selectedAgent = "code-reviewer"
	initVariantSelectionScreen(&m, sampleGroupedWithVariants()["anthropic"][0])

	for _, width := range []int{80, 40} {
		m.width = width
		m.height = 24
		out := viewVariantSelection(m)
		assert.NotEmpty(t, out)
		stripped := ansi.Strip(out)
		assert.Contains(t, stripped, "Select Variant")
		assert.Contains(t, stripped, "Model:")
		if width >= 70 {
			assert.Contains(t, stripped, "Enter Apply variant")
		} else {
			assert.Contains(t, stripped, "Enter Apply · Esc Cancel")
		}
	}
}

func TestVariantSelection_BulkAll_AppliesModelAndVariantToAllNonDisabled(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkAll, "")

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, anthropicIdx, 0)
	m.modelCursor = anthropicIdx

	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ScreenVariantSelection, inVariant.state)
	require.Equal(t, "anthropic/claude-sonnet-4-20250514", inVariant.pendingSelectedModel.FullName)

	inVariant.variantCursor = 0
	require.Equal(t, "high", inVariant.availableVariants[0].Name)

	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, committed.state)
	assert.True(t, committed.dirty)

	primary, subagents, _ := committed.config.GetAgents()
	for _, name := range append(primary, subagents...) {
		if committed.config.IsAgentDisabled(name) {
			continue
		}
		modVal, ok := committed.config.GetAgentModelOverride(name)
		require.True(t, ok, "agent %q must have model override", name)
		assert.Equal(t, "anthropic/claude-sonnet-4-20250514", modVal)

		varVal, ok := committed.config.GetAgentField(name, "variant")
		require.True(t, ok, "agent %q must have variant override", name)
		assert.Equal(t, "high", varVal)
	}

	buildMod, ok := committed.config.GetAgentModelOverride("build")
	if ok {
		assert.NotEqual(t, "anthropic/claude-sonnet-4-20250514", buildMod)
	}
	_, buildVar := committed.config.GetAgentField("build", "variant")
	assert.False(t, buildVar)

	for _, ch := range committed.changes {
		assert.NotEqual(t, "build", ch.Target)
		assert.Equal(t, "anthropic/claude-sonnet-4-20250514", ch.NewModel)
		assert.Equal(t, "high", ch.NewVariant)
	}
}

func TestVariantSelection_BulkAll_SkipsDisabled(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkAll, "")

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})

	for _, ch := range committed.changes {
		assert.NotEqual(t, "build", ch.Target, "disabled agent 'build' must not be in changes")
	}
}

func TestVariantSelection_BulkAll_Idempotent(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkAll, "")
	require.NoError(t, m.config.SetAgentModelOverride("code-reviewer", "anthropic/claude-sonnet-4-20250514"))
	require.NoError(t, m.config.SetAgentVariantOverride("code-reviewer", "high"))

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	inVariant.variantCursor = 0
	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})

	for _, ch := range committed.changes {
		assert.NotEqual(t, "code-reviewer", ch.Target, "idempotent agent must not produce a Change")
	}
}

func TestVariantSelection_BulkList_AppliesModelAndVariantToBulkTargets(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "debug"}

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx

	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ScreenVariantSelection, inVariant.state)
	inVariant.variantCursor = 2
	require.Equal(t, "medium", inVariant.availableVariants[2].Name)

	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, committed.state)
	assert.True(t, committed.dirty)
	assert.Nil(t, committed.bulkTargets)

	for _, name := range []string{"plan", "debug"} {
		modVal, ok := committed.config.GetAgentModelOverride(name)
		require.True(t, ok)
		assert.Equal(t, "anthropic/claude-sonnet-4-20250514", modVal)

		varVal, ok := committed.config.GetAgentField(name, "variant")
		require.True(t, ok)
		assert.Equal(t, "medium", varVal)
	}

	exploreVar, ok := committed.config.GetAgentField("explore", "variant")
	if ok {
		assert.NotEqual(t, "medium", exploreVar)
	}

	require.Len(t, committed.changes, 2)
	targets := []string{committed.changes[0].Target, committed.changes[1].Target}
	assert.Contains(t, targets, "plan")
	assert.Contains(t, targets, "debug")
}

func TestVariantSelection_BulkList_SkipsDisabled(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "build"}

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	inVariant.variantCursor = 1

	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Len(t, committed.changes, 1)
	assert.Equal(t, "plan", committed.changes[0].Target)

	_, hasVar := committed.config.GetAgentField("build", "variant")
	assert.False(t, hasVar, "disabled agent 'build' must not have variant field")
}

func TestVariantSelection_BulkList_DuplicateTargetsIdempotent(t *testing.T) {
	m := newTestModelSelectWithVariants(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "plan", "debug"}

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	inVariant.variantCursor = 0

	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Len(t, committed.changes, 2, "duplicate target 'plan' must produce only one change")
}

func TestVariantSelection_BulkCancel_PreservesBulkModeOnESC(t *testing.T) {
	t.Run("bulk-all cancel preserves fieldEditing", func(t *testing.T) {
		m := newTestModelSelectWithVariants(t, fieldEditingBulkAll, "")
		anthropicIdx := -1
		for i, mod := range m.filteredModels {
			if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
				anthropicIdx = i
				break
			}
		}
		m.modelCursor = anthropicIdx
		inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Equal(t, ScreenVariantSelection, inVariant.state)

		canceled, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEsc})
		assert.Equal(t, ScreenModelSelection, canceled.state)
		assert.Equal(t, fieldEditingBulkAll, canceled.fieldEditing)
		assert.False(t, canceled.dirty)
		assert.Empty(t, canceled.changes)
	})

	t.Run("bulk-list cancel preserves bulkTargets and fieldEditing", func(t *testing.T) {
		m := newTestModelSelectWithVariants(t, fieldEditingBulkList, "")
		m.bulkTargets = []string{"plan", "debug"}

		anthropicIdx := -1
		for i, mod := range m.filteredModels {
			if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
				anthropicIdx = i
				break
			}
		}
		m.modelCursor = anthropicIdx
		inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Equal(t, ScreenVariantSelection, inVariant.state)

		canceled, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEsc})
		assert.Equal(t, ScreenModelSelection, canceled.state)
		assert.Equal(t, fieldEditingBulkList, canceled.fieldEditing)
		assert.Equal(t, []string{"plan", "debug"}, canceled.bulkTargets)
		assert.False(t, canceled.dirty)
		assert.Empty(t, canceled.changes)
	})
}

func TestVariantSelection_BulkPreservesUnknownKeysAndConfigOptions(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentField("plan", "temperature", 0.8))
	require.NoError(t, cfg.SetAgentField("plan", "custom_unknown_key", "important_value"))

	m := NewModel(cfg, sampleGroupedWithVariants(), 5)
	m.state = ScreenModelSelection
	m.navigationStack = []appState{ScreenAgentList}
	m.fieldEditing = fieldEditingBulkAll
	initModelSelectionScreen(&m)

	anthropicIdx := -1
	for i, mod := range m.filteredModels {
		if mod.FullName == "anthropic/claude-sonnet-4-20250514" {
			anthropicIdx = i
			break
		}
	}
	m.modelCursor = anthropicIdx
	inVariant, _ := updateModelSelection(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	inVariant.variantCursor = 0

	committed, _ := updateVariantSelection(inVariant, tea.KeyPressMsg{Code: tea.KeyEnter})

	tempVal, ok := committed.config.GetAgentField("plan", "temperature")
	require.True(t, ok)
	assert.Equal(t, 0.8, tempVal)

	customVal, ok := committed.config.GetAgentField("plan", "custom_unknown_key")
	require.True(t, ok)
	assert.Equal(t, "important_value", customVal)
}

func TestVariantSelection_ShowsOptionsSummary(t *testing.T) {
	grouped := map[string][]opencode.Model{
		"openai": {
			{
				Provider: "openai",
				ID:       "gpt-5",
				FullName: "openai/gpt-5",
				Variants: []opencode.VariantDescriptor{
					{Name: "high", Options: map[string]interface{}{"reasoningEffort": "high"}},
					{Name: "thinking", Options: map[string]interface{}{"thinking": map[string]interface{}{"budgetTokens": 16000}}},
					{Name: "plain"},
				},
			},
		},
	}

	m := NewModel(fixtureConfig(t), grouped, 5)
	m.state = ScreenVariantSelection
	m.selectedAgent = "code-reviewer"
	initVariantSelectionScreen(&m, grouped["openai"][0])

	out := ansi.Strip(viewVariantSelection(m))
	assert.Contains(t, out, "high  (effort: high)")
	assert.Contains(t, out, "thinking  (budget: 16000)")
	assert.Contains(t, out, "plain")
	assert.NotContains(t, out, "plain  (")
}
