// Package tui tests — multi-select screen (REQ-TUI-002, REQ-TUI-003).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in agent_multi_select.go, and drive its design. Coverage focuses on:
//   - initAgentMultiSelectScreen: item population, disabled exclusion, hidden inclusion
//   - updateAgentMultiSelect: cursor movement, SPACE toggle, ENTER transitions, ESC cancel
//   - viewAgentMultiSelect: checkbox rendering, selected count
//   - 'm' key on AgentList transitions to multi-select
//   - Flow B end-to-end integration
//
// Spec: REQ-TUI-002 ('m' key starts Flow B), REQ-TUI-003 (checkbox list render).
package tui

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/config"
)

// newMultiSelectModel constructs a Model positioned on ScreenAgentMultiSelect
// with items already populated. Mirrors what 'm' key + initAgentMultiSelectScreen
// would produce.
func newMultiSelectModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	m.state = ScreenAgentMultiSelect
	m.navigationStack = []appState{ScreenAgentList}
	initAgentMultiSelectScreen(&m)
	return m
}

// expectedMultiSelectItems returns the sorted agent names that should appear
// in the multi-select list for the fixture config (primary + subagents minus
// disabled, system agents already excluded by GetAgents).
func expectedMultiSelectItems() []string {
	return []string{
		"code-reviewer",
		"debug",
		"docs",
		"explore",
		"general",
		"orchestrator",
		"parallel-dispatch",
		"plan",
		"security-auditor",
		"team-lead",
	}
}

// ---------------------------------------------------------------------------
// initAgentMultiSelectScreen (REQ-TUI-003 — item population)
// ---------------------------------------------------------------------------

// TestInitAgentMultiSelect_PopulatesItemsSorted verifies that the multi-select
// items are populated from primary + subagents and sorted alphabetically.
func TestInitAgentMultiSelect_PopulatesItemsSorted(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	initAgentMultiSelectScreen(&m)

	expected := expectedMultiSelectItems()
	assert.Equal(t, expected, m.multiSelectItems,
		"multiSelectItems MUST be sorted primary+subagents from the fixture")
	assert.Len(t, m.multiSelectChecked, len(expected),
		"multiSelectChecked MUST be parallel to multiSelectItems")
	assert.Equal(t, 0, m.multiSelectCursor,
		"cursor MUST start at 0")
}

// TestInitAgentMultiSelect_ExcludesDisabled verifies that disabled agents are
// not included in the multi-select items list.
func TestInitAgentMultiSelect_ExcludesDisabled(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	initAgentMultiSelectScreen(&m)

	for _, name := range m.multiSelectItems {
		assert.False(t, m.config.IsAgentDisabled(name),
			"disabled agent %q MUST NOT appear in multiSelectItems", name)
	}
	assert.NotContains(t, m.multiSelectItems, "build",
		"disabled agent 'build' MUST NOT appear in multiSelectItems")
}

// TestInitAgentMultiSelect_IncludesHidden verifies that hidden agents ARE
// included in the multi-select items list (REQ-TUI-003, Business Rule 4).
func TestInitAgentMultiSelect_IncludesHidden(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	initAgentMultiSelectScreen(&m)

	assert.Contains(t, m.multiSelectItems, "parallel-dispatch",
		"hidden agent 'parallel-dispatch' MUST appear in multiSelectItems")
}

// TestInitAgentMultiSelect_ItemsAreSorted verifies items are alphabetically sorted.
func TestInitAgentMultiSelect_ItemsAreSorted(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	initAgentMultiSelectScreen(&m)

	sorted := make([]string, len(m.multiSelectItems))
	copy(sorted, m.multiSelectItems)
	sort.Strings(sorted)
	assert.Equal(t, sorted, m.multiSelectItems,
		"multiSelectItems MUST be sorted alphabetically")
}

// ---------------------------------------------------------------------------
// updateAgentMultiSelect — cursor movement (REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestUpdateAgentMultiSelect_CursorMovement verifies j/k and arrow keys move
// the cursor within the multi-select list.
func TestUpdateAgentMultiSelect_CursorMovement(t *testing.T) {
	tests := []struct {
		name   string
		key    tea.KeyMsg
		start  int
		expect int
	}{
		{"j moves down", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, 0, 1},
		{"Down moves down", tea.KeyMsg{Type: tea.KeyDown}, 0, 1},
		{"k at top stays 0", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, 0, 0},
		{"Up at top stays 0", tea.KeyMsg{Type: tea.KeyUp}, 0, 0},
		{"k moves up from 2", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, 2, 1},
		{"Up moves up from 2", tea.KeyMsg{Type: tea.KeyUp}, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultiSelectModel(t)
			m.multiSelectCursor = tt.start
			newM, _ := updateAgentMultiSelect(m, tt.key)
			assert.Equal(t, tt.expect, newM.multiSelectCursor,
				"cursor MUST be %d after key", tt.expect)
		})
	}
}

// TestUpdateAgentMultiSelect_CursorStopsAtBottom verifies cursor cannot go
// past the last item.
func TestUpdateAgentMultiSelect_CursorStopsAtBottom(t *testing.T) {
	m := newMultiSelectModel(t)
	m.multiSelectCursor = len(m.multiSelectItems) - 1
	newM, _ := updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, len(m.multiSelectItems)-1, newM.multiSelectCursor,
		"cursor MUST stop at last item")
}

// ---------------------------------------------------------------------------
// updateAgentMultiSelect — SPACE toggle (REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestUpdateAgentMultiSelect_SpaceToggles verifies SPACE toggles the checkbox
// at the current cursor position.
func TestUpdateAgentMultiSelect_SpaceToggles(t *testing.T) {
	m := newMultiSelectModel(t)
	require.False(t, m.multiSelectChecked[0])

	newM, _ := updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.True(t, newM.multiSelectChecked[0],
		"SPACE MUST check the item at cursor")

	newM2, _ := updateAgentMultiSelect(newM, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.False(t, newM2.multiSelectChecked[0],
		"SPACE again MUST uncheck the item at cursor")
}

// ---------------------------------------------------------------------------
// updateAgentMultiSelect — ENTER transitions (REQ-TUI-002, REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestUpdateAgentMultiSelect_EnterWithNoSelection_StaysOnScreen verifies that
// ENTER with zero checked items is a no-op — the screen stays as-is.
func TestUpdateAgentMultiSelect_EnterWithNoSelection_StaysOnScreen(t *testing.T) {
	m := newMultiSelectModel(t)
	for i := range m.multiSelectChecked {
		m.multiSelectChecked[i] = false
	}

	newM, _ := updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentMultiSelect, newM.state,
		"ENTER with no selection MUST stay on ScreenAgentMultiSelect")
	assert.Empty(t, newM.bulkTargets,
		"ENTER with no selection MUST NOT populate bulkTargets")
}

// TestUpdateAgentMultiSelect_EnterWithSelection_TransitionsAndPopulatesBulkTargets
// verifies that ENTER with checked items transitions to ScreenModelSelection
// with fieldEditing="bulk-list" and bulkTargets populated with the checked names.
func TestUpdateAgentMultiSelect_EnterWithSelection_TransitionsAndPopulatesBulkTargets(t *testing.T) {
	m := newMultiSelectModel(t)
	m.multiSelectChecked[0] = true
	m.multiSelectChecked[2] = true

	newM, _ := updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, ScreenModelSelection, newM.state,
		"ENTER with selections MUST transition to ScreenModelSelection")
	assert.Equal(t, fieldEditingBulkList, newM.fieldEditing,
		"fieldEditing MUST be 'bulk-list' after ENTER with selections")
	assert.Len(t, newM.bulkTargets, 2,
		"bulkTargets MUST contain exactly the 2 checked items")
	assert.Contains(t, newM.bulkTargets, m.multiSelectItems[0])
	assert.Contains(t, newM.bulkTargets, m.multiSelectItems[2])
}

// ---------------------------------------------------------------------------
// updateAgentMultiSelect — ESC cancel (REQ-TUI-002, REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestUpdateAgentMultiSelect_EscClearsStateAndPops verifies ESC clears
// multi-select state and pops back to the previous screen.
func TestUpdateAgentMultiSelect_EscClearsStateAndPops(t *testing.T) {
	m := newMultiSelectModel(t)
	m.multiSelectChecked[0] = true
	require.Len(t, m.navigationStack, 1)

	newM, _ := updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ScreenAgentList, newM.state,
		"ESC MUST pop back to the previous screen (ScreenAgentList)")
	assert.Nil(t, newM.multiSelectItems,
		"ESC MUST clear multiSelectItems")
	assert.Nil(t, newM.multiSelectChecked,
		"ESC MUST clear multiSelectChecked")
	assert.Empty(t, newM.bulkTargets,
		"ESC MUST NOT populate bulkTargets")
}

// ---------------------------------------------------------------------------
// viewAgentMultiSelect — rendering (REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestViewAgentMultiSelect_RendersCheckboxes verifies the view renders
// checkbox indicators for each item.
func TestViewAgentMultiSelect_RendersCheckboxes(t *testing.T) {
	m := newMultiSelectModel(t)
	out := viewAgentMultiSelect(m)
	assert.NotEmpty(t, out, "view MUST return non-empty output")
	assert.Contains(t, out, "[ ]",
		"unchecked checkbox MUST be rendered as '[ ]'")

	m.multiSelectChecked[0] = true
	out = viewAgentMultiSelect(m)
	assert.Contains(t, out, "[x]",
		"checked checkbox MUST be rendered as '[x]'")
}

// TestViewAgentMultiSelect_ShowsSelectedCount verifies the status bar shows
// the number of selected agents.
func TestViewAgentMultiSelect_ShowsSelectedCount(t *testing.T) {
	m := newMultiSelectModel(t)
	m.multiSelectChecked[0] = true
	m.multiSelectChecked[1] = true
	out := viewAgentMultiSelect(m)
	assert.Contains(t, out, "2",
		"selected count MUST appear in the rendered output")
}

// ---------------------------------------------------------------------------
// 'm' key on AgentList (REQ-TUI-002)
// ---------------------------------------------------------------------------

// TestAgentList_MKey_TransitionsToMultiSelect verifies pressing 'm' on the
// AgentList screen transitions to ScreenAgentMultiSelect.
func TestAgentList_MKey_TransitionsToMultiSelect(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	require.Equal(t, ScreenAgentList, m.state)

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	assert.Equal(t, ScreenAgentMultiSelect, newM.state,
		"'m' key MUST transition to ScreenAgentMultiSelect")
	assert.NotEmpty(t, newM.multiSelectItems,
		"multiSelectItems MUST be populated by 'm' key transition")
	assert.Len(t, newM.multiSelectChecked, len(newM.multiSelectItems),
		"multiSelectChecked MUST be initialized parallel to items")
}

// ---------------------------------------------------------------------------
// Dispatcher routing — Update() and View() route to ScreenAgentMultiSelect
// ---------------------------------------------------------------------------

// TestUpdate_DispatchesToAgentMultiSelect verifies the global Update() routes
// ScreenAgentMultiSelect key presses to updateAgentMultiSelect.
func TestUpdate_DispatchesToAgentMultiSelect(t *testing.T) {
	m := newMultiSelectModel(t)
	require.Equal(t, 0, m.multiSelectCursor)

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result, ok := newModel.(Model)
	require.True(t, ok, "Update must return the same Model type")
	assert.Equal(t, 1, result.multiSelectCursor,
		"global Update() MUST dispatch ScreenAgentMultiSelect to updateAgentMultiSelect")
}

// TestView_DispatchesToAgentMultiSelect verifies the global View() routes
// ScreenAgentMultiSelect to viewAgentMultiSelect.
func TestView_DispatchesToAgentMultiSelect(t *testing.T) {
	m := newMultiSelectModel(t)
	out := m.View()
	assert.Contains(t, out, "[ ]",
		"global View() MUST dispatch ScreenAgentMultiSelect to viewAgentMultiSelect")
}

// ---------------------------------------------------------------------------
// G3-T3: selectModelAtCursor bulk-list branch (REQ-TUI-004)
// ---------------------------------------------------------------------------

// TestSelectModelAtCursor_BulkList_AppliesOnlyToBulkTargets verifies that
// bulk-list mode applies the model only to the agents in bulkTargets.
func TestSelectModelAtCursor_BulkList_AppliesOnlyToBulkTargets(t *testing.T) {
	m := newModelSelectModel(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "debug"}
	targetModel := m.filteredModels[m.modelCursor].FullName

	result := selectModelAtCursor(m)

	for _, name := range m.bulkTargets {
		val, ok := result.config.GetAgentField(name, "model")
		require.True(t, ok, "target %q MUST have a model", name)
		s, _ := val.(string)
		assert.Equal(t, targetModel, s,
			"target %q MUST have model %q", name, targetModel)
	}

	val, ok := result.config.GetAgentField("explore", "model")
	if ok {
		s, _ := val.(string)
		assert.NotEqual(t, targetModel, s,
			"non-target agent 'explore' MUST NOT have the bulk model")
	}
}

// TestSelectModelAtCursor_BulkList_ClearsBulkTargets verifies bulkTargets is
// cleared after commit.
func TestSelectModelAtCursor_BulkList_ClearsBulkTargets(t *testing.T) {
	m := newModelSelectModel(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "debug"}

	result := selectModelAtCursor(m)
	assert.Nil(t, result.bulkTargets,
		"bulkTargets MUST be cleared after bulk-list commit")
}

// TestSelectModelAtCursor_BulkList_SkipsDisabled verifies that if a disabled
// agent is in bulkTargets, it is skipped (not mutated).
func TestSelectModelAtCursor_BulkList_SkipsDisabled(t *testing.T) {
	m := newModelSelectModel(t, fieldEditingBulkList, "")
	m.bulkTargets = []string{"plan", "build"}
	targetModel := m.filteredModels[m.modelCursor].FullName

	result := selectModelAtCursor(m)

	val, _ := result.config.GetAgentField("build", "model")
	if val != nil {
		s, _ := val.(string)
		assert.NotEqual(t, targetModel, s,
			"disabled agent 'build' MUST NOT be mutated by bulk-list")
	}

	assert.Len(t, result.changes, 1,
		"bulk-list with 1 disabled target MUST produce only 1 change")
	assert.Equal(t, "plan", result.changes[0].Target)
}

// (TestPopScreen_ClearsBulkListSentinel lives in model_test.go — REQ-TUI-006.)

// ---------------------------------------------------------------------------
// G3-T3: Flow B end-to-end integration (REQ-TUI-002, REQ-TUI-004)
// ---------------------------------------------------------------------------

// copyFixtureToTemp copies the fixture opencode.json to a temp dir and returns
// the path. Used by integration tests that need to verify disk writes.
func copyFixtureToTemp(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "test", "fixtures", "opencode.json")
	abs, err := filepath.Abs(src)
	require.NoError(t, err)

	content, err := os.ReadFile(abs)
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), "opencode.json")
	require.NoError(t, os.WriteFile(dst, content, 0644))
	return dst
}

// TestFlowB_EndToEnd verifies the complete Flow B sequence:
// 'm' → SPACE ×2 → ENTER → pick model → changes recorded.
//
// Spec: REQ-TUI-002 (Flow B), REQ-TUI-004 (RecordChange per target).
func TestFlowB_EndToEnd(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)

	// Step 1: 'm' on AgentList → ScreenAgentMultiSelect
	m, _ = updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	require.Equal(t, ScreenAgentMultiSelect, m.state,
		"step 1: 'm' MUST transition to multi-select")

	// Step 2: SPACE on item 0, move down, SPACE on item 1
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.True(t, m.multiSelectChecked[0],
		"step 2a: first SPACE MUST check item 0")

	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.True(t, m.multiSelectChecked[1],
		"step 2b: second SPACE MUST check item 1")

	targets := []string{m.multiSelectItems[0], m.multiSelectItems[1]}

	// Step 3: ENTER → ScreenModelSelection with bulk-list
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, ScreenModelSelection, m.state,
		"step 3: ENTER MUST transition to ScreenModelSelection")
	require.Equal(t, fieldEditingBulkList, m.fieldEditing,
		"step 3: fieldEditing MUST be bulk-list")
	require.Len(t, m.bulkTargets, 2,
		"step 3: bulkTargets MUST contain 2 items")

	// Step 4: Select model at cursor → RecordChange per target
	selectedModel := m.filteredModels[m.modelCursor]
	result := selectModelAtCursor(m)
	require.Len(t, result.changes, 2,
		"step 4: bulk-list MUST produce exactly 2 Changes")

	for _, ch := range result.changes {
		assert.Equal(t, "model", ch.Field,
			"each change MUST be on the 'model' field")
		assert.Equal(t, selectedModel.FullName, ch.NewVal,
			"each change NewVal MUST be the selected model")
	}

	// Verify the config was mutated in-memory for both targets.
	for _, target := range targets {
		val, ok := result.config.GetAgentField(target, "model")
		require.True(t, ok, "target %q MUST have a model after bulk-list", target)
		s, _ := val.(string)
		assert.Equal(t, selectedModel.FullName, s,
			"target %q model MUST be the selected model", target)
	}
}

// TestFlowB_SaveConfirmWritesToDisk verifies the full Flow B including disk
// persistence via config.Save().
func TestFlowB_SaveConfirmWritesToDisk(t *testing.T) {
	dstPath := copyFixtureToTemp(t)
	cfg, err := config.LoadConfig(dstPath)
	require.NoError(t, err)

	m := NewModel(cfg, richGrouped(), 0)

	// Flow B: 'm' → check 2 → ENTER → pick model
	m, _ = updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	checkedAgents := []string{m.multiSelectItems[0], m.multiSelectItems[1]}

	m, _ = updateAgentMultiSelect(m, tea.KeyMsg{Type: tea.KeyEnter})
	selectedModel := m.filteredModels[m.modelCursor]
	result := selectModelAtCursor(m)
	require.Len(t, result.changes, 2)

	// Save to disk.
	require.NoError(t, result.config.Save())

	// Reload and verify the two checked agents have the new model.
	reloaded, err := config.LoadConfig(dstPath)
	require.NoError(t, err)
	for _, name := range checkedAgents {
		val, ok := reloaded.GetAgentField(name, "model")
		require.True(t, ok)
		s, _ := val.(string)
		assert.Equal(t, selectedModel.FullName, s,
			"agent %q MUST have the selected model on disk after save", name)
	}
}
