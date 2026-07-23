// Package tui tests — state machine and Model construction.
//
// These tests follow strict TDD: they were written BEFORE the production code
// in styles.go and model.go, and drive its design. Coverage focuses on the
// core state machine: NewModel initialization, appState transitions, and
// Update() handling of q / Ctrl+C / ESC / 's' at the global dispatcher level.
//
// Spec: REQ-TUI-001 (TUI initialization), REQ-TUI-008 (screen transitions).
package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// fixtureConfig loads the sanitized opencode.json fixture from the repo's
// test/fixtures directory. The fixture contains 14 agents (3 system, 2 primary,
// 9 subagents), with `build` disabled and `parallel-dispatch` hidden.
func fixtureConfig(t *testing.T) *config.Config {
	t.Helper()
	p := filepath.Join("..", "..", "test", "fixtures", "opencode.json")
	abs, err := filepath.Abs(p)
	require.NoError(t, err, "failed to resolve fixture path")
	cfg, err := config.LoadConfig(abs)
	require.NoError(t, err, "fixture config must load")
	return cfg
}

// sampleGrouped returns a small grouped-models map suitable for Model
// construction tests. Two providers, two models total.
func sampleGrouped() map[string][]opencode.Model {
	return map[string][]opencode.Model{
		"opencode-go": {
			{Provider: "opencode-go", ID: "glm-5.2", FullName: "opencode-go/glm-5.2"},
		},
		"openai": {
			{Provider: "openai", ID: "gpt-5.5", FullName: "openai/gpt-5.5"},
		},
	}
}

// ---------------------------------------------------------------------------
// NewModel — construction and initial state (REQ-TUI-001)
// ---------------------------------------------------------------------------

// TestNewModel_InitialStateIsAgentList verifies that a freshly constructed
// Model starts on the Agent List screen.
//
// Spec: REQ-TUI-001 — Happy path — TUI launches to agent list.
func TestNewModel_InitialStateIsAgentList(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	assert.Equal(t, ScreenAgentList, m.state,
		"initial state MUST be ScreenAgentList")
}

// TestNewModel_NilConfigDoesNotPanic verifies that passing a nil config does
// not crash — the constructor must handle the missing config gracefully
// (later screens render an error/empty state).
//
// Spec: REQ-TUI-001 — Error — nil config passed.
func TestNewModel_NilConfigDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		m := NewModel(nil, sampleGrouped(), 5)
		assert.Equal(t, ScreenAgentList, m.state,
			"even with nil config the model starts on the agent list screen")
	})
}

// TestNewModel_EmptyModelsDoesNotPanic verifies that an empty models map is
// handled without panicking. Model-selection screens will later show a
// "No models available" message.
//
// Spec: REQ-TUI-001 — Edge case — TUI with no available models.
func TestNewModel_EmptyModelsDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		m := NewModel(fixtureConfig(t), map[string][]opencode.Model{}, 5)
		assert.Empty(t, m.flatModels, "flatModels must be empty when no models are provided")
	})
}

// TestNewModel_NilModelsDoesNotPanic verifies that a nil models map is also
// safe — groupedModels must be initialized to a non-nil empty map so later
// range loops do not panic.
func TestNewModel_NilModelsDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		m := NewModel(fixtureConfig(t), nil, 5)
		assert.NotNil(t, m.groupedModels, "groupedModels must NEVER be nil — range loops would panic")
		assert.Empty(t, m.flatModels)
	})
}

// TestNewModel_EditableFields verifies the 6-field schema used by the Agent
// Detail screen.
//
// Spec: REQ-TUI-004 — Happy path — show 6 editable fields.
func TestNewModel_EditableFields(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	expected := []string{"model", "temperature", "top_p", "color", "steps", "disable"}
	assert.Equal(t, expected, m.editableFields,
		"editableFields must be the 6 fields shown on the Agent Detail screen, in this order")
}

// TestNewModel_AgentListsPopulatedFromConfig verifies that primaryAgents,
// subagents, and disabledAgents are seeded from the loaded config.
//
// Spec: REQ-CFG-008 (filtered through REQ-TUI-002 grouping).
func TestNewModel_AgentListsPopulatedFromConfig(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)

	// Fixture primary agents (non-system): build, plan.
	assert.Contains(t, m.primaryAgents, "build")
	assert.Contains(t, m.primaryAgents, "plan")

	// Fixture subagents include code-reviewer.
	assert.Contains(t, m.subagents, "code-reviewer")
	assert.Contains(t, m.subagents, "explore")

	// Fixture disabled: build only.
	assert.Contains(t, m.disabledAgents, "build")
}

// TestNewModel_SystemAgentsExcluded verifies that compactación, title, and
// summary never appear in any user-facing agent list.
//
// Spec: REQ-CFG-008 + REQ-TUI-002 — system agents filtered out.
func TestNewModel_SystemAgentsExcluded(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	for _, sys := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, m.primaryAgents, sys, "system agent %q must not be in primaryAgents", sys)
		assert.NotContains(t, m.subagents, sys, "system agent %q must not be in subagents", sys)
		assert.NotContains(t, m.disabledAgents, sys, "system agent %q must not be in disabledAgents", sys)
	}
}

// TestNewModel_BackupCountStored verifies the retention value flows through
// to the model so the save-confirm screen can use it later.
func TestNewModel_BackupCountStored(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 3)
	assert.Equal(t, 3, m.backupCount)
}

// TestNewModel_FlatModelsBuilt verifies that the grouped map is flattened
// into a single slice used by the fuzzy filter in model selection.
func TestNewModel_FlatModelsBuilt(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	// Two providers, one model each → 2 flat models.
	assert.Len(t, m.flatModels, 2)
}

// TestNewModel_DefaultCursorZero verifies the cursor starts at the top of
// the agent list.
func TestNewModel_DefaultCursorZero(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	assert.Equal(t, 0, m.agentCursor)
	assert.False(t, m.dirty, "freshly constructed model must not be dirty")
}

// TestNewModel_TextInputsInitialized verifies that the bubbles/textinput
// sub-components are usable (not zero-value) so subsequent screen handlers
// can call Update on them without panicking.
func TestNewModel_TextInputsInitialized(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	// A zero-value textinput.Model would panic on Update; calling Value()
	// on a New()'d model returns an empty string safely.
	assert.Equal(t, "", m.filterInput.Value())
	assert.Equal(t, "", m.fieldInput.Value())
}

// ---------------------------------------------------------------------------
// Init (REQ-TUI-001)
// ---------------------------------------------------------------------------

// TestInit_DoesNotPanic verifies Init() can be called safely. The command
// returned may be nil or a real command; the contract here is "no panic".
func TestInit_DoesNotPanic(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	require.NotPanics(t, func() {
		_ = m.Init()
	})
}

// ---------------------------------------------------------------------------
// Update — global key dispatcher (REQ-TUI-003, REQ-TUI-007, REQ-TUI-008)
// ---------------------------------------------------------------------------

// TestUpdate_CtrlC_Quits verifies that Ctrl+C always produces a tea.Quit
// command regardless of the current screen.
func TestUpdate_CtrlC_Quits(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "Ctrl+C MUST produce a non-nil command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "Ctrl+C MUST produce a tea.QuitMsg")
}

// TestUpdate_CtrlC_DirtyOnAgentList_ShowsConfirmation verifies that Ctrl+C on
// the Agent List screen with unsaved changes routes to the screen handler so
// the quit-confirmation overlay is shown instead of quitting immediately.
//
// Spec: REQ-TUI-003 — Ctrl+C respects the dirty guard on AgentList.
func TestUpdate_CtrlC_DirtyOnAgentList_ShowsConfirmation(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := newM.(Model)
	assert.True(t, result.quitConfirm,
		"Ctrl+C on AgentList with dirty MUST set quitConfirm, not quit")
	assert.Nil(t, cmd,
		"Ctrl+C on AgentList with dirty MUST NOT quit immediately")
}

// TestUpdate_Q_Quits verifies that pressing 'q' produces tea.Quit at the
// top level. The dirty-check confirmation flow is deferred to G2-T5; for
// the core dispatcher 'q' simply quits.
func TestUpdate_Q_Quits(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd, "'q' MUST produce a non-nil command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'q' MUST produce a tea.QuitMsg")
}

// TestUpdate_ESC_PopsToPreviousState verifies that ESC returns the model to
// the screen stored in previousState (single-deep navigation stack).
//
// Spec: REQ-TUI-008 — Edge case — ESC from model selection / agent detail.
func TestUpdate_ESC_PopsToPreviousState(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	// Simulate being on a sub-screen entered from AgentList.
	m.state = ScreenAgentDetail
	m.navigationStack = []appState{ScreenAgentList}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result, ok := newM.(Model)
	require.True(t, ok, "Update must return the same Model type")
	assert.Equal(t, ScreenAgentList, result.state,
		"ESC MUST pop back to previousState")
}

// TestUpdate_ESC_FromAgentList_StaysAtRoot verifies that ESC on a clean root
// screen quits without mutating the root state.
func TestUpdate_ESC_FromAgentList_StaysAtRoot(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newM.(Model)
	assert.Equal(t, ScreenAgentList, result.state,
		"ESC on the root screen must stay on the root screen")
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

// TestUpdate_S_NotDirty_StaysOnScreen verifies that pressing 's' when there
// are no unsaved changes does NOT transition to the save-confirm screen.
//
// Spec: REQ-TUI-007 — Edge case — save with no changes.
func TestUpdate_S_NotDirty_StaysOnScreen(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	require.False(t, m.dirty, "precondition: model must not be dirty")

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	result := newM.(Model)
	assert.Equal(t, ScreenAgentList, result.state,
		"'s' with no dirty state must NOT transition to ScreenSaveConfirm")
}

// TestUpdate_S_Dirty_TransitionsToSaveConfirm verifies that pressing 's'
// when dirty transitions to the Save Confirm screen and remembers the
// origin screen in previousState.
//
// Spec: REQ-TUI-007 — Happy path — save all changes.
func TestUpdate_S_Dirty_TransitionsToSaveConfirm(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	result := newM.(Model)
	assert.Equal(t, ScreenSaveConfirm, result.state,
		"'s' with dirty state MUST transition to ScreenSaveConfirm")
	assert.Equal(t, []appState{ScreenAgentList}, result.navigationStack,
		"navigation stack must record the screen we came from so ESC works")
}

// TestUpdate_OtherKey_NoOp verifies that an unmapped key does not change
// state or produce a panic. Screen-specific handlers added in G2-T2..T5
// will handle j/k/enter/etc.
func TestUpdate_OtherKey_NoOp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	result := newM.(Model)
	assert.Equal(t, ScreenAgentList, result.state,
		"unmapped keys must not change state at the core dispatcher")
}

// TestUpdate_WindowSizeMsg_SetsDimensions verifies the model captures the
// terminal size so screen rendering can wrap and pad correctly.
func TestUpdate_WindowSizeMsg_SetsDimensions(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	result := newM.(Model)
	assert.Equal(t, 100, result.width)
	assert.Equal(t, 40, result.height)
}

// ---------------------------------------------------------------------------
// View — placeholder dispatch (REQ-TUI-002 + remaining REQ-TUI-00x screens)
// ---------------------------------------------------------------------------

// TestView_AllStatesReturnNonEmpty verifies that View() returns a non-empty
// string for every appState. The actual screen content is implemented in
// subsequent tasks (G2-T2..T5); for now placeholders ensure the dispatcher
// covers every state without panic.
func TestView_AllStatesReturnNonEmpty(t *testing.T) {
	cfg := fixtureConfig(t)
	states := []appState{
		ScreenAgentList,
		ScreenAgentDetail,
		ScreenModelSelection,
		ScreenFieldInput,
		ScreenSaveConfirm,
	}
	for _, st := range states {
		m := NewModel(cfg, sampleGrouped(), 5)
		m.state = st
		out := m.View()
		assert.NotEmpty(t, out, "state %d MUST render a non-empty placeholder", st)
	}
}

// TestView_NilConfigDoesNotPanic verifies that a model constructed with a
// nil config renders an error message rather than crashing.
func TestView_NilConfigDoesNotPanic(t *testing.T) {
	m := NewModel(nil, sampleGrouped(), 5)
	require.NotPanics(t, func() {
		out := m.View()
		assert.NotEmpty(t, out)
	})
}

func TestUpdate_NestedPickerEscReturnsDetailThenAgentList(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "code-reviewer")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenAgentDetail, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenModelSelection, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentDetail, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentList, m.state)
}

func TestUpdate_NestedFieldInputEscReturnsDetailThenAgentList(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "code-reviewer")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenAgentDetail, m.state)
	m.detailCursor = 1

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenFieldInput, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentDetail, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentList, m.state)
}

func TestUpdate_SaveConfirmRepeatedSDoesNotCorruptCancelTarget(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenAgentDetail
	m.navigationStack = []appState{ScreenAgentList}
	m.dirty = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	require.Equal(t, ScreenSaveConfirm, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	require.Equal(t, ScreenSaveConfirm, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentDetail, m.state)
}

func TestModelPicker_PrintableQsjkReachFilterInput(t *testing.T) {
	m := newModelSelectModel(t, "global", "")

	for _, r := range []rune{'q', 's', 'j', 'k'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	assert.Equal(t, ScreenModelSelection, m.state)
	assert.Equal(t, "qsjk", m.filterInput.Value())
}

func TestFieldInput_PrintableQsReachTextInput(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")

	for _, r := range []rune{'q', 's'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	assert.Equal(t, ScreenFieldInput, m.state)
	assert.Equal(t, "qs", m.fieldInput.Value())
}

func TestAgentList_CursorRestoredAfterModelPicker(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	items := selectableItems(m)
	wantCursor := indexOf(items, "code-reviewer")
	m.agentCursor = wantCursor

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	assert.Equal(t, ScreenAgentList, m.state)
	assert.Equal(t, wantCursor, m.agentCursor)
}

func TestModelPicker_CursorIndependentFromAgentListCursor(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	items := selectableItems(m)
	wantAgentCursor := indexOf(items, "code-reviewer")
	m.agentCursor = wantAgentCursor

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenModelSelection, m.state)
	assert.Equal(t, 0, m.modelCursor, "model picker starts at its own first result")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 1, m.modelCursor, "model picker cursor moves independently")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, wantAgentCursor, m.agentCursor)
}
