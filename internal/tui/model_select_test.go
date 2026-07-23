// Package tui tests — model selection screen (REQ-TUI-005).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in model_select.go, and drive its design. Coverage focuses on:
//   - matchesFilter: case-insensitive substring match on provider + ID + FullName
//   - applyFilter: rebuild filteredModels from flatModels, clamp cursor
//   - Rendering: grouped-by-provider display, filter input, current-model marker,
//     empty states (no models / no filter results), cursor highlight.
//   - Navigation: j/k, arrows, ENTER (global + per-agent), ESC, typing, backspace.
//
// Spec: REQ-TUI-005 (model selection screen with grouped display and filter).
package tui

import (
	"sort"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// richGrouped returns a multi-provider, multi-model map suitable for model
// selection tests. Mirrors the layout in the task spec mockup.
func richGrouped() map[string][]opencode.Model {
	return map[string][]opencode.Model{
		"opencode-go": {
			{Provider: "opencode-go", ID: "glm-5.1", FullName: "opencode-go/glm-5.1"},
			{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
		},
		"zai-coding-plan": {
			{Provider: "zai-coding-plan", ID: "glm-4.5-air", FullName: "zai-coding-plan/glm-4.5-air"},
			{Provider: "zai-coding-plan", ID: "glm-4.7", FullName: "zai-coding-plan/glm-4.7"},
			{Provider: "zai-coding-plan", ID: "glm-5-turbo", FullName: "zai-coding-plan/glm-5-turbo"},
		},
	}
}

// newModelSelectModel constructs a Model positioned on the Model Selection
// screen. It mirrors the initialization that updateAgentList / updateAgentDetail
// perform when transitioning to ScreenModelSelection.
//
//   - fieldEditing "global" → editing the global default model
//   - fieldEditing "model"  → editing the per-agent model (requires agentName)
func newModelSelectModel(t *testing.T, fieldEditing, agentName string) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	m.state = ScreenModelSelection
	m.navigationStack = []appState{ScreenAgentList}
	m.fieldEditing = fieldEditing
	m.selectedAgent = agentName
	m.filterInput = textinput.New()
	m.filterInput.Placeholder = "Type to filter..."
	m.filterInput.Focus()
	m.modelCursor = 0
	// Initialize filteredModels to the full flat list (show all initially).
	m.filteredModels = append([]opencode.Model(nil), m.flatModels...)
	sort.Slice(m.filteredModels, func(i, j int) bool {
		return m.filteredModels[i].FullName < m.filteredModels[j].FullName
	})
	return m
}

// ---------------------------------------------------------------------------
// matchesFilter (REQ-TUI-005 — filter logic)
// ---------------------------------------------------------------------------

// TestMatchesFilter_EmptyFilterMatchesAll verifies that an empty filter string
// matches every model.
func TestMatchesFilter_EmptyFilterMatchesAll(t *testing.T) {
	m := opencode.Model{Provider: "openai", ID: "gpt-5", FullName: "openai/gpt-5"}
	assert.True(t, matchesFilter(m, ""),
		"empty filter MUST match all models")
}

// TestMatchesFilter_MatchesProvider verifies a substring match on Provider.
func TestMatchesFilter_MatchesProvider(t *testing.T) {
	m := opencode.Model{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"}
	assert.True(t, matchesFilter(m, "opencode-go"),
		"filter matching the provider MUST return true")
}

// TestMatchesFilter_MatchesID verifies a substring match on ID.
func TestMatchesFilter_MatchesID(t *testing.T) {
	m := opencode.Model{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"}
	assert.True(t, matchesFilter(m, "glm-5.2"),
		"filter matching the ID MUST return true")
}

// TestMatchesFilter_MatchesFullName verifies a substring match on FullName.
func TestMatchesFilter_MatchesFullName(t *testing.T) {
	m := opencode.Model{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"}
	assert.True(t, matchesFilter(m, "opencode-go/glm"),
		"filter matching the FullName MUST return true")
}

// TestMatchesFilter_CaseInsensitive verifies the match is case-insensitive.
func TestMatchesFilter_CaseInsensitive(t *testing.T) {
	m := opencode.Model{Provider: "OpenAI", ID: "GPT-5", FullName: "OpenAI/GPT-5"}
	assert.True(t, matchesFilter(m, "openai"),
		"lowercase filter MUST match uppercase provider")
	assert.True(t, matchesFilter(m, "gpt"),
		"lowercase filter MUST match uppercase ID")
}

// TestMatchesFilter_NoMatch verifies a non-matching filter returns false.
func TestMatchesFilter_NoMatch(t *testing.T) {
	m := opencode.Model{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"}
	assert.False(t, matchesFilter(m, "ZZZNOMATCH"),
		"a filter that matches nothing MUST return false")
}

// TestMatchesFilter_PartialMatch verifies partial substrings match.
func TestMatchesFilter_PartialMatch(t *testing.T) {
	m := opencode.Model{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"}
	assert.True(t, matchesFilter(m, "glm"),
		"partial substring 'glm' MUST match the ID 'glm-5.2'")
	assert.True(t, matchesFilter(m, "5.2"),
		"partial substring '5.2' MUST match the ID 'glm-5.2'")
}

// ---------------------------------------------------------------------------
// applyFilter (REQ-TUI-005 — filter rebuild)
// ---------------------------------------------------------------------------

// TestApplyFilter_EmptyFilterShowsAll verifies that with an empty filter,
// filteredModels contains every model from flatModels.
func TestApplyFilter_EmptyFilterShowsAll(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("")
	m = applyFilter(m)
	assert.Len(t, m.filteredModels, len(m.flatModels),
		"empty filter MUST show all models")
}

// TestApplyFilter_GlmFilterNarrows verifies that filter "glm" narrows the list
// to only models containing "glm" in provider, ID, or FullName.
func TestApplyFilter_GlmFilterNarrows(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("glm")
	m = applyFilter(m)
	// Every model in the fixture contains "glm" in its ID.
	for _, model := range m.filteredModels {
		assert.True(t, matchesFilter(model, "glm"),
			"filtered model %q MUST match filter 'glm'", model.FullName)
	}
	assert.NotEmpty(t, m.filteredModels,
		"with 'glm' filter there MUST be results")
}

// TestApplyFilter_ProviderFilterNarrows verifies that filtering by provider
// name returns only that provider's models.
func TestApplyFilter_ProviderFilterNarrows(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("opencode-go")
	m = applyFilter(m)
	for _, model := range m.filteredModels {
		assert.Equal(t, "opencode-go", model.Provider,
			"filtering by 'opencode-go' MUST only return opencode-go models, got %q", model.FullName)
	}
	assert.Len(t, m.filteredModels, 2,
		"opencode-go provider has exactly 2 models in the fixture")
}

// TestApplyFilter_NoMatchProducesEmpty verifies that a non-matching filter
// produces an empty filteredModels slice.
func TestApplyFilter_NoMatchProducesEmpty(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("ZZZNOMATCH")
	m = applyFilter(m)
	assert.Empty(t, m.filteredModels,
		"a non-matching filter MUST produce an empty filteredModels slice")
}

// TestApplyFilter_ClampsCursor verifies that if the cursor was beyond the
// filtered list length, it is clamped to a valid index.
func TestApplyFilter_ClampsCursor(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 4                     // beyond the 5 total models
	m.filterInput.SetValue("opencode-go") // narrows to 2
	m = applyFilter(m)
	assert.Less(t, m.modelCursor, len(m.filteredModels),
		"cursor MUST be clamped to a valid index after filtering")
	assert.GreaterOrEqual(t, m.modelCursor, 0,
		"cursor MUST never be negative")
}

// TestApplyFilter_EmptyListCursorStaysZero verifies cursor is 0 when the
// filtered list is empty.
func TestApplyFilter_EmptyListCursorStaysZero(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("ZZZNOMATCH")
	m = applyFilter(m)
	assert.Equal(t, 0, m.modelCursor,
		"cursor MUST be 0 when filtered list is empty")
}

// ---------------------------------------------------------------------------
// Rendering — viewModelSelection (REQ-TUI-005)
// ---------------------------------------------------------------------------

// TestViewModelSelection_ShowsAllModelsGrouped verifies that with no filter,
// all models appear in the rendered output, organized by provider sections.
func TestViewModelSelection_ShowsAllModelsGrouped(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)

	// Every model FullName should appear.
	for _, model := range m.flatModels {
		assert.Contains(t, out, model.ID,
			"model ID %q MUST appear in the rendered output", model.ID)
	}
}

// TestViewModelSelection_ShowsProviderHeaders verifies that provider names
// appear as section headers.
func TestViewModelSelection_ShowsProviderHeaders(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)

	assert.Contains(t, out, "opencode-go",
		"provider 'opencode-go' MUST appear as a section header")
	assert.Contains(t, out, "zai-coding-plan",
		"provider 'zai-coding-plan' MUST appear as a section header")
}

// TestViewModelSelection_FilterInputVisible verifies the filter input is shown
// at the top of the screen.
func TestViewModelSelection_FilterInputVisible(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)
	// The word "Filter" or the filter input view should be present.
	assert.True(t,
		containsAny(out, "Filter", "filter", "Type to filter"),
		"the filter input label/prompt MUST appear in the rendered output")
}

// TestViewModelSelection_FilterTextShown verifies that typed filter text
// appears in the rendered output.
func TestViewModelSelection_FilterTextShown(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("glm-5")
	out := viewModelSelection(m)
	assert.Contains(t, out, "glm-5",
		"the filter text 'glm-5' MUST appear in the rendered output")
}

// TestViewModelSelection_FilteredResultsOnly verifies that when a filter is
// applied, only matching models are rendered.
func TestViewModelSelection_FilteredResultsOnly(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("opencode-go")
	m = applyFilter(m)
	out := viewModelSelection(m)

	// opencode-go models MUST appear.
	assert.Contains(t, out, "glm-5.1",
		"opencode-go model 'glm-5.1' MUST appear when filtering by 'opencode-go'")
	assert.Contains(t, out, "glm-5.2",
		"opencode-go model 'glm-5.2' MUST appear when filtering by 'opencode-go'")

	// zai-coding-plan models MUST NOT appear.
	assert.NotContains(t, out, "glm-4.5-air",
		"non-matching model 'glm-4.5-air' MUST NOT appear in filtered output")
	assert.NotContains(t, out, "glm-4.7",
		"non-matching model 'glm-4.7' MUST NOT appear in filtered output")
}

// TestViewModelSelection_NoMatchMessage verifies that when the filter produces
// no results, a "No models match filter" message is shown.
func TestViewModelSelection_NoMatchMessage(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("ZZZNOMATCH")
	m = applyFilter(m)
	out := viewModelSelection(m)
	assert.Contains(t, out, "No models match",
		"a 'No models match' message MUST be shown when the filter has no results")
}

// TestViewModelSelection_NoModelsAvailable verifies that when the models list
// is empty, a "No models available" message is shown.
func TestViewModelSelection_NoModelsAvailable(t *testing.T) {
	m := NewModel(fixtureConfig(t), map[string][]opencode.Model{}, 5)
	m.state = ScreenModelSelection
	m.fieldEditing = "global"
	m.filterInput = textinput.New()
	m.filterInput.Focus()
	m.filteredModels = nil
	out := viewModelSelection(m)
	assert.Contains(t, out, "No models available",
		"a 'No models available' message MUST be shown when the flat list is empty")
}

// TestViewModelSelection_CurrentModelMarkerForGlobal verifies that the currently
// assigned global model shows a checkmark marker.
func TestViewModelSelection_CurrentModelMarkerForGlobal(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.SetGlobalModel("opencode-go/glm-5.2")
	m := NewModel(cfg, richGrouped(), 5)
	m.state = ScreenModelSelection
	m.fieldEditing = "global"
	m.filterInput = textinput.New()
	m.filterInput.Focus()
	m.filteredModels = append([]opencode.Model(nil), m.flatModels...)
	sort.Slice(m.filteredModels, func(i, j int) bool {
		return m.filteredModels[i].FullName < m.filteredModels[j].FullName
	})
	out := viewModelSelection(m)
	assert.Contains(t, out, "glm-5.2",
		"the current global model ID MUST appear in the output")
	assert.True(t, containsAny(out, "\u2713", "current"),
		"the current model MUST be marked with a checkmark (\u2713) or 'current' indicator")
}

// TestViewModelSelection_CurrentModelMarkerForAgent verifies that the currently
// assigned per-agent model shows a checkmark marker.
func TestViewModelSelection_CurrentModelMarkerForAgent(t *testing.T) {
	m := newModelSelectModel(t, "model", "code-reviewer")
	// code-reviewer has model "anthropic/claude-sonnet-4-20250514" in the fixture,
	// which is NOT in richGrouped — so no checkmark should appear for the fixture
	// models. Instead, set it to a model that IS in richGrouped.
	require.NoError(t, m.config.SetAgentField("code-reviewer", "model", "opencode-go/glm-5.2"))
	out := viewModelSelection(m)
	assert.True(t, containsAny(out, "\u2713", "current"),
		"the current per-agent model MUST be marked with a checkmark or 'current' indicator")
}

// TestViewModelSelection_CursorHighlight verifies that the cursor position is
// visually indicated (with '►' or '>' marker).
func TestViewModelSelection_CursorHighlight(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)
	assert.True(t,
		containsAny(out, "\u25ba", ">"),
		"the cursor position MUST be visually indicated with a marker")
}

// TestViewModelSelection_ReturnsNonEmpty verifies a basic non-empty contract.
func TestViewModelSelection_ReturnsNonEmpty(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)
	assert.NotEmpty(t, out, "viewModelSelection MUST return a non-empty string")
}

// TestViewModelSelection_HelpFooter verifies the help text mentions filter,
// select, and cancel keys.
func TestViewModelSelection_HelpFooter(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := viewModelSelection(m)
	assert.Contains(t, out, "Enter Apply model · Esc Cancel",
		"help footer MUST explain that Enter applies the model")
}

// ---------------------------------------------------------------------------
// Navigation — updateModelSelection (REQ-TUI-005)
// ---------------------------------------------------------------------------

// TestUpdateModelSelection_J_MovesCursorDown verifies that 'j' increments the
// cursor in the filtered list.
func TestUpdateModelSelection_CtrlN_MovesCursorDown(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	require.Equal(t, 0, m.modelCursor)

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyCtrlN})
	assert.Equal(t, 1, newM.modelCursor,
		"cursor MUST be 1 after pressing Ctrl+N")
}

// TestUpdateModelSelection_DownArrow_MovesDown verifies Down arrow works like j.
func TestUpdateModelSelection_DownArrow_MovesDown(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, newM.modelCursor,
		"Down arrow MUST move cursor down")
}

// TestUpdateModelSelection_J_StopsAtBottom verifies 'j' does not exceed the
// last index.
func TestUpdateModelSelection_CtrlN_StopsAtBottom(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = len(m.filteredModels) - 1 // last index
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyCtrlN})
	assert.Equal(t, len(m.filteredModels)-1, newM.modelCursor,
		"cursor MUST NOT exceed the last index when pressing Ctrl+N")
}

// TestUpdateModelSelection_K_MovesCursorUp verifies that 'k' decrements cursor.
func TestUpdateModelSelection_CtrlP_MovesCursorUp(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 2
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	assert.Equal(t, 1, newM.modelCursor,
		"cursor MUST be 1 after pressing Ctrl+P from 2")
}

// TestUpdateModelSelection_UpArrow_MovesUp verifies Up arrow works like k.
func TestUpdateModelSelection_UpArrow_MovesUp(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 2
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, newM.modelCursor,
		"Up arrow MUST move cursor up")
}

// TestUpdateModelSelection_K_AtTopStaysAtZero verifies 'k' at cursor 0 stays.
func TestUpdateModelSelection_CtrlP_AtTopStaysAtZero(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 0
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	assert.Equal(t, 0, newM.modelCursor,
		"cursor MUST stay at 0 when pressing Ctrl+P at the top")
}

// ---------------------------------------------------------------------------
// ENTER — model selection (REQ-TUI-005)
// ---------------------------------------------------------------------------

// TestUpdateModelSelection_EnterOnGlobal_SetsGlobalModel verifies that ENTER
// when editing the global model calls SetGlobalModel, sets dirty, and returns
// to previousState.
func TestUpdateModelSelection_EnterOnGlobal_SetsGlobalModel(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.navigationStack = []appState{ScreenAgentList}
	require.False(t, m.dirty)

	// Cursor 0 → first model in sorted filteredModels.
	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, ScreenAgentList, newM.state,
		"ENTER MUST return to previousState (ScreenAgentList)")

	gm, ok := newM.config.GetGlobalModel()
	require.True(t, ok, "a global model MUST be set after ENTER")
	assert.NotEmpty(t, gm, "the global model MUST not be empty")
	assert.True(t, newM.dirty,
		"dirty MUST be true after selecting a model")
}

// TestUpdateModelSelection_EnterOnAgent_SetsAgentModel verifies that ENTER
// when editing a per-agent model calls SetAgentField, sets dirty, and returns
// to previousState.
func TestUpdateModelSelection_EnterOnAgent_SetsAgentModel(t *testing.T) {
	m := newModelSelectModel(t, "model", "code-reviewer")
	m.navigationStack = []appState{ScreenAgentDetail}
	require.False(t, m.dirty)

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, ScreenAgentDetail, newM.state,
		"ENTER MUST return to previousState (ScreenAgentDetail)")

	val, ok := newM.config.GetAgentField("code-reviewer", "model")
	require.True(t, ok, "the agent model MUST be set after ENTER")
	s, ok := val.(string)
	require.True(t, ok)
	assert.NotEmpty(t, s, "the agent model MUST not be empty")
	assert.True(t, newM.dirty,
		"dirty MUST be true after selecting a model")
}

// TestUpdateModelSelection_Enter_SelectsCorrectModel verifies that ENTER
// selects the model at the current cursor position.
func TestUpdateModelSelection_Enter_SelectsCorrectModel(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	// Move cursor to index 1.
	m.modelCursor = 1
	expected := m.filteredModels[1].FullName

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyEnter})
	gm, ok := newM.config.GetGlobalModel()
	require.True(t, ok)
	assert.Equal(t, expected, gm,
		"ENTER MUST select the model at cursor position (%q), got %q", expected, gm)
}

// ---------------------------------------------------------------------------
// ESC — cancel (REQ-TUI-005, REQ-TUI-008)
// ---------------------------------------------------------------------------

// TestUpdateModelSelection_Esc_ReturnsToPrevious verifies ESC cancels and
// returns to previousState without changes.
func TestUpdateModelSelection_Esc_ReturnsToPrevious(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.navigationStack = []appState{ScreenAgentList}
	dirtyBefore := m.dirty

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ScreenAgentList, newM.state,
		"ESC MUST return to previousState without changes")
	assert.Equal(t, dirtyBefore, newM.dirty,
		"ESC MUST NOT change dirty state")
}

// ---------------------------------------------------------------------------
// Typing — filter input update (REQ-TUI-005)
// ---------------------------------------------------------------------------

// TestUpdateModelSelection_TypingUpdatesFilter verifies that typing a character
// updates the filter input value.
func TestUpdateModelSelection_TypingUpdatesFilter(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	require.Empty(t, m.filterInput.Value())

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, "g", newM.filterInput.Value(),
		"typing 'g' MUST update the filter input to 'g'")
}

// TestUpdateModelSelection_TypingResetsCursor verifies that typing a character
// resets the cursor to 0.
func TestUpdateModelSelection_TypingResetsCursor(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 3

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, newM.modelCursor,
		"typing MUST reset cursor to 0")
}

// TestUpdateModelSelection_TypingAppliesFilter verifies that typing applies the
// filter to filteredModels.
func TestUpdateModelSelection_TypingAppliesFilter(t *testing.T) {
	m := newModelSelectModel(t, "global", "")

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o', 'p', 'e', 'n', 'c', 'o', 'd', 'e'}})
	// After typing "opencode" every remaining model MUST match.
	for _, model := range newM.filteredModels {
		assert.True(t, matchesFilter(model, "opencode"),
			"after typing 'opencode', model %q MUST match", model.FullName)
	}
}

// TestUpdateModelSelection_BackspaceDeletesFromFilter verifies that Backspace
// removes a character from the filter input.
func TestUpdateModelSelection_BackspaceDeletesFromFilter(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.filterInput.SetValue("glm")
	require.Equal(t, "glm", m.filterInput.Value())

	newM, _ := updateModelSelection(m, tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "gl", newM.filterInput.Value(),
		"Backspace MUST delete the last character from the filter")
}

// ---------------------------------------------------------------------------
// Dispatcher integration — Update() routes to updateModelSelection
// ---------------------------------------------------------------------------

// TestUpdate_DispatchesToModelSelection verifies that the global Update()
// function routes ScreenModelSelection key presses to updateModelSelection.
func TestUpdate_DispatchesToModelSelection(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	result, ok := newM.(Model)
	require.True(t, ok, "Update must return the same Model type")
	assert.Equal(t, 1, result.modelCursor,
		"global Update() MUST dispatch ScreenModelSelection to updateModelSelection")
}

// TestView_DispatchesToModelSelection verifies that the global View() function
// routes ScreenModelSelection to viewModelSelection.
func TestView_DispatchesToModelSelection(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	out := m.View()
	assert.Contains(t, out, "opencode-go",
		"global View() MUST dispatch ScreenModelSelection to viewModelSelection")
}

// ---------------------------------------------------------------------------
// Transition initialization (REQ-TUI-005 — entering the screen)
// ---------------------------------------------------------------------------

// TestUpdateAgentList_EnterOnGlobal_InitializesFilterInput verifies that
// transitioning from the agent list to model selection initializes the filter
// input (focused, empty value) and filteredModels.
func TestUpdateAgentList_EnterOnGlobal_InitializesFilterInput(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	m.agentCursor = 0 // __global__
	// Pre-pollute the filter input to verify it gets reset.
	m.filterInput.SetValue("stale")

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state)
	assert.Empty(t, newM.filterInput.Value(),
		"filterInput MUST be reset to empty when entering model selection")
	assert.NotEmpty(t, newM.filteredModels,
		"filteredModels MUST be initialized with all models when entering model selection")
	assert.Equal(t, 0, newM.modelCursor,
		"cursor MUST be reset to 0 when entering model selection")
}

// TestUpdateAgentDetail_EnterOnModel_InitializesFilterInput verifies that
// transitioning from agent detail to model selection initializes the filter
// input and filteredModels.
func TestUpdateAgentDetail_EnterOnModel_InitializesFilterInput(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	m.state = ScreenAgentDetail
	m.selectedAgent = "code-reviewer"
	m.detailCursor = 0 // "model"
	m.navigationStack = []appState{ScreenAgentList}
	m.filterInput.SetValue("stale")

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state)
	assert.Empty(t, newM.filterInput.Value(),
		"filterInput MUST be reset to empty when entering model selection from detail")
	assert.NotEmpty(t, newM.filteredModels,
		"filteredModels MUST be initialized with all models when entering model selection from detail")
	assert.Equal(t, 0, newM.modelCursor,
		"cursor MUST be reset to 0 when entering model selection from detail")
}
