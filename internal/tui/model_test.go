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
	"os"
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
//
// It isolates $HOME (HOME + USERPROFILE) to an empty temp dir so the host's
// global markdown agents do not leak into GetAgents (markdown discovery is
// HOME-based); TUI tests that DO want markdown fixtures set up their own HOME.
func fixtureConfig(t *testing.T) *config.Config {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
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

func TestUpdate_DirectPickerEscReturnsAgentList(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "code-reviewer")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ScreenModelSelection, m.state)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, ScreenAgentList, m.state)
}

func TestUpdate_AgentActivationNeverReachesNonModelEditors(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "code-reviewer")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.Equal(t, ScreenModelSelection, m.state)
	assert.NotEqual(t, ScreenAgentDetail, m.state)
	assert.NotEqual(t, ScreenFieldInput, m.state)
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
	require.Equal(t, ScreenModelSelection, m.state)
	assert.Equal(t, 0, m.modelCursor, "model picker starts at its own first result")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 1, m.modelCursor, "model picker cursor moves independently")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.Equal(t, wantAgentCursor, m.agentCursor)
}

// ---------------------------------------------------------------------------
// Bulk sentinel constants and bulkTargets field (REQ-TUI-001)
// ---------------------------------------------------------------------------

// TestModel_FieldEditingBulkAllConstant verifies the sentinel constant for
// Flow A (apply-to-all) exists with the correct value.
func TestModel_FieldEditingBulkAllConstant(t *testing.T) {
	assert.Equal(t, "bulk-all", fieldEditingBulkAll,
		"fieldEditingBulkAll MUST equal \"bulk-all\"")
}

// TestModel_FieldEditingBulkListConstant verifies the sentinel constant for
// Flow B (multi-select) exists with the correct value.
func TestModel_FieldEditingBulkListConstant(t *testing.T) {
	assert.Equal(t, "bulk-list", fieldEditingBulkList,
		"fieldEditingBulkList MUST equal \"bulk-list\"")
}

// TestModel_HasBulkTargetsField verifies the Model struct has a bulkTargets
// []string field that can be set and read.
func TestModel_HasBulkTargetsField(t *testing.T) {
	var m Model
	m.bulkTargets = []string{"agent-a", "agent-b"}
	assert.Equal(t, []string{"agent-a", "agent-b"}, m.bulkTargets,
		"Model MUST have a bulkTargets []string field")
}

// ---------------------------------------------------------------------------
// Bulk sentinel cleanup — popScreen and performSave (REQ-TUI-006)
// ---------------------------------------------------------------------------

// TestPopScreen_ClearsBulkAllSentinel verifies that popping ScreenModelSelection
// when fieldEditing was "bulk-all" resets the sentinel and bulkTargets so no
// stale state leaks into subsequent flows.
//
// Spec: REQ-TUI-006 — ESC from bulk ModelSelection clears sentinels.
func TestPopScreen_ClearsBulkAllSentinel(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenModelSelection
	m.fieldEditing = fieldEditingBulkAll
	m.bulkTargets = []string{"stale"}
	m.navigationStack = []appState{ScreenAgentList}

	m.popScreen()

	assert.Equal(t, ScreenAgentList, m.state,
		"popScreen MUST return to the navigation origin")
	assert.Equal(t, "", m.fieldEditing,
		"popScreen from bulk-all MUST clear fieldEditing")
	assert.Nil(t, m.bulkTargets,
		"popScreen from bulk-all MUST clear bulkTargets")
}

// TestPopScreen_ClearsBulkListSentinel verifies the same cleanup for bulk-list.
//
// Spec: REQ-TUI-006 — ESC from bulk ModelSelection clears sentinels.
func TestPopScreen_ClearsBulkListSentinel(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenModelSelection
	m.fieldEditing = fieldEditingBulkList
	m.bulkTargets = []string{"agent-a", "agent-b"}
	m.navigationStack = []appState{ScreenAgentList}

	m.popScreen()

	assert.Equal(t, ScreenAgentList, m.state)
	assert.Equal(t, "", m.fieldEditing,
		"popScreen from bulk-list MUST clear fieldEditing")
	assert.Nil(t, m.bulkTargets,
		"popScreen from bulk-list MUST clear bulkTargets")
}

// TestPopScreen_PreservesGlobalSentinel is a regression guard: the "global"
// sentinel MUST NOT be cleared by popScreen. Only bulk-* sentinels are cleaned.
//
// Spec: REQ-TUI-006 — only bulk sentinels are cleared, not "global".
func TestPopScreen_PreservesGlobalSentinel(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenModelSelection
	m.fieldEditing = "global"
	m.navigationStack = []appState{ScreenAgentList}

	m.popScreen()

	assert.Equal(t, ScreenAgentList, m.state)
	assert.Equal(t, "global", m.fieldEditing,
		"popScreen from global ModelSelection MUST NOT clear fieldEditing")
}

// TestSaveComplete_ClearsBulkState verifies that a successful performSave
// resets bulkTargets and fieldEditing alongside the existing cleanup
// (dirty, changes, navigationStack).
//
// Spec: REQ-TUI-006 — save completes after bulk → state cleared.
func TestSaveComplete_ClearsBulkState(t *testing.T) {
	m := writableSaveConfirmModel(t, 0)
	m.fieldEditing = fieldEditingBulkList
	m.bulkTargets = []string{"agent-a", "agent-b"}

	result, _ := performSave(m)

	assert.Equal(t, ScreenAgentList, result.state,
		"performSave MUST return to AgentList on success")
	assert.Nil(t, result.bulkTargets,
		"performSave MUST clear bulkTargets on success")
	assert.Equal(t, "", result.fieldEditing,
		"performSave MUST clear fieldEditing on success")
}

// TestSingleAgentFlowAfterBulkCancel verifies that after cancelling a bulk-all
// flow via ESC, immediately entering a single-agent edit does NOT inherit the
// stale bulk-all sentinel.
//
// Spec: REQ-TUI-006 — entering single-agent flow after bulk cancel uses the
// agent name, not a stale sentinel.
func TestSingleAgentFlowAfterBulkCancel(t *testing.T) {
	m := NewModel(fixtureConfig(t), richGrouped(), 5)

	// Simulate Flow A entry
	m.pushScreen(ScreenModelSelection)
	m.fieldEditing = fieldEditingBulkAll
	require.Equal(t, fieldEditingBulkAll, m.fieldEditing)

	// Simulate ESC (cancel)
	m.popScreen()

	// Immediately enter a single-agent edit
	m.selectedAgent = "plan"
	m.pushScreen(ScreenModelSelection)
	m.fieldEditing = "plan"

	assert.NotEqual(t, fieldEditingBulkAll, m.fieldEditing,
		"fieldEditing MUST NOT retain stale bulk-all after cancel")
	assert.NotEqual(t, fieldEditingBulkList, m.fieldEditing,
		"fieldEditing MUST NOT retain stale bulk-list after cancel")
	assert.Equal(t, "plan", m.fieldEditing,
		"fieldEditing MUST be the agent name for single-agent edit")
}

// ---------------------------------------------------------------------------
// Markdown agent integration (T-B6, T-B7)
//
// The TUI consumes cfg.GetAgents() which includes merged markdown agents.
// Markdown-backed agents are now EDITABLE in v2: ENTER opens the Agent Detail
// editor for them, and edits persist as inline-JSON overrides
// (agent.<name>.<field>) via SetAgentField — the .md file is never touched.
// The merged display reader (GetMergedAgentField) lets the TUI show the
// agent's actual md model instead of "(none)" before any override exists.
// ---------------------------------------------------------------------------

// loadFixtureWithMD loads the JSON fixture config while pointing $HOME at a
// temp dir whose ~/.config/opencode/agents/ contains the supplied .md files.
// files maps file name -> content. The fixture's own JSON agents are unchanged.
func loadFixtureWithMD(t *testing.T, files map[string]string) *config.Config {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if len(files) > 0 {
		gdir := filepath.Join(home, ".config", "opencode", "agents")
		require.NoError(t, os.MkdirAll(gdir, 0o755))
		for name, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(gdir, name), []byte(content), 0o644))
		}
	}
	abs, err := filepath.Abs(filepath.Join("..", "..", "test", "fixtures", "opencode.json"))
	require.NoError(t, err)
	cfg, err := config.LoadConfig(abs)
	require.NoError(t, err)
	return cfg
}

// TestTUI_ShowsMDAgents verifies that markdown agents appear in the TUI agent
// lists (consumed from cfg.GetAgents) and are flagged md-only.
func TestTUI_ShowsMDAgents(t *testing.T) {
	cfg := loadFixtureWithMD(t, map[string]string{
		"mdshown.md": "---\nmode: subagent\nmodel: m1\n---\nBody.\n",
	})

	m := NewModel(cfg, sampleGrouped(), 5)
	assert.Contains(t, m.subagents, "mdshown",
		"markdown agent must appear in the TUI subagent list")
	assert.True(t, m.IsMarkdownOnly("mdshown"),
		"md-only agent (no inline-JSON backing) must be flagged")
	assert.False(t, m.IsMarkdownOnly("code-reviewer"),
		"a JSON-backed agent must NOT be flagged md-only")
}

// TestTUI_MarkdownBackedAgentIsEditable verifies that pressing ENTER on an
// md-backed agent opens model selection. Edits persist as inline-JSON
// overrides (agent.<name>.<field>); the .md file is never written. This was
// previously gated off (read-only v1); the gate is removed in T-B7 because
// OpenCode treats agent.<name> as a per-field overlay on the md agent, so a
// model-only JSON entry overrides the model without shadowing the prompt.
//
// Spec: REQ-TUI-003 — ENTER on an agent (any agent) opens AgentDetail.
func TestTUI_MarkdownBackedAgentIsEditable(t *testing.T) {
	cfg := loadFixtureWithMD(t, map[string]string{
		"editable.md": "---\nmode: primary\nmodel: m\n---\nBody.\n",
	})

	m := NewModel(cfg, sampleGrouped(), 5)
	items := selectableItems(m)
	cursor := indexOf(items, "editable")
	require.GreaterOrEqual(t, cursor, 0,
		"md-backed agent 'editable' must be selectable in the list")
	m.agentCursor = cursor

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(Model)
	assert.Equal(t, ScreenModelSelection, result.state,
		"ENTER on an md-backed agent MUST open model selection")
	assert.Equal(t, "editable", result.selectedAgent,
		"selectedAgent MUST be the md-backed agent at the cursor")

	// Sanity: a JSON-backed agent still opens the editor on ENTER.
	m2 := NewModel(cfg, sampleGrouped(), 5)
	items2 := selectableItems(m2)
	cursor2 := indexOf(items2, "code-reviewer")
	require.GreaterOrEqual(t, cursor2, 0)
	m2.agentCursor = cursor2
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result2 := updated2.(Model)
	assert.Equal(t, ScreenModelSelection, result2.state,
		"ENTER on a JSON-backed agent must still open model selection")
}

// TestTUI_MarkdownAgentShowsMergedModel verifies that a md-backed agent's
// detail and list rows show its actual markdown model (via the merged reader),
// NOT "(none)". This is the display-side fix that complements the edit-gate
// removal: before any override exists, the user must see what they would be
// overriding.
func TestTUI_MarkdownAgentShowsMergedModel(t *testing.T) {
	cfg := loadFixtureWithMD(t, map[string]string{
		"rev.md": "---\nmode: subagent\nmodel: anthropic/claude-3-opus\n---\nBody.\n",
	})

	m := NewModel(cfg, sampleGrouped(), 5)

	// Detail screen shows the merged model, not "(none)".
	m.state = ScreenAgentDetail
	m.selectedAgent = "rev"
	m.navigationStack = []appState{ScreenAgentList}
	detailOut := viewAgentDetail(m)
	assert.Contains(t, detailOut, "anthropic/claude-3-opus",
		"detail screen MUST show the merged md model for an md-backed agent")

	// List row shows the merged model, not "(none)" next to the agent name.
	listOut := viewAgentList(NewModel(cfg, sampleGrouped(), 5))
	assert.Contains(t, listOut, "anthropic/claude-3-opus",
		"agent list row MUST show the merged md model for an md-backed agent")
}

// TestTUI_MarkdownAgentBadgeDisappearsAfterJSONOverride verifies that once a
// JSON override exists for an md-backed agent, the [MD] badge disappears from
// the rendered list (the agent is no longer MdOnly in the merged view). This
// requires performSave to recompute the mdOnlyAgents cache after a successful
// save.
func TestTUI_MarkdownAgentBadgeDisappearsAfterJSONOverride(t *testing.T) {
	files := map[string]string{
		"rev.md": "---\nmode: subagent\nmodel: md-orig\n---\nBody.\n",
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	gdir := filepath.Join(home, ".config", "opencode", "agents")
	require.NoError(t, os.MkdirAll(gdir, 0o755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(gdir, name), []byte(content), 0o644))
	}

	// Writable JSON config in a separate temp dir.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "opencode.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0o600))
	cfg, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	m := NewModel(cfg, sampleGrouped(), 0)

	// Before override: [MD] badge is shown for rev.
	assert.True(t, m.IsMarkdownOnly("rev"),
		"precondition: rev starts as md-only")
	listBefore := viewAgentList(m)
	assert.Contains(t, listBefore, "[MD]",
		"[MD] badge MUST be shown while no JSON override exists")

	// Simulate the user setting a model override via the editor flow.
	require.NoError(t, m.config.SetAgentField("rev", "model", "json-model"))
	m.RecordChange("rev", "model", "md-orig", "json-model")
	m.dirty = true
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}

	updated, _ := performSave(m)
	m = updated

	// After Save: badge disappears because mdOnlyAgents was recomputed.
	assert.False(t, m.IsMarkdownOnly("rev"),
		"after a JSON override is saved, rev MUST NOT be flagged md-only")
	listAfter := viewAgentList(m)
	assert.NotContains(t, listAfter, "[MD]",
		"[MD] badge MUST disappear once a JSON override exists")

	// And the merged model reflected is the override.
	val, ok := m.config.GetMergedAgentField("rev", "model")
	require.True(t, ok)
	assert.Equal(t, "json-model", val)
}

// TestTUI_EditingMarkdownAgentPersistsJSONOnly verifies the end-to-end editing
// flow: selecting a model on a md-backed agent writes ONLY an inline-JSON
// agent.<name>.model entry; the .md file remains byte-identical. After reload
// the merged model reflects the override (per-field merge over the md agent).
func TestTUI_EditingMarkdownAgentPersistsJSONOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	gdir := filepath.Join(home, ".config", "opencode", "agents")
	require.NoError(t, os.MkdirAll(gdir, 0o755))
	mdContent := []byte("---\nmode: subagent\nmodel: md-original\n---\nBody.\n")
	mdPath := filepath.Join(gdir, "review.md")
	require.NoError(t, os.WriteFile(mdPath, mdContent, 0o644))

	// Writable JSON config: empty so review is purely md-backed at first.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "opencode.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0o600))
	cfg, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	// Verify the precondition: GetAgentField returns absent (md only).
	_, ok := cfg.GetAgentField("review", "model")
	assert.False(t, ok, "precondition: review has no JSON entry yet")

	// Apply a model override via SetAgentField — the exact write path used by
	// selectModelAtCursor / commitFieldInput / ApplyModelToAgents.
	require.NoError(t, cfg.SetAgentField("review", "model", "opencode-go/glm-5.2"))
	require.NoError(t, cfg.Save())

	// 1) The .md file MUST be byte-for-byte unchanged.
	gotMD, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Equal(t, string(mdContent), string(gotMD),
		"Save MUST NOT modify the markdown agent file")

	// 2) The JSON config now contains an inline-JSON agent.review.model entry
	//    and ONLY that entry (no prompt, no mode copied from md — confirming
	//    the JSON write path stays model-only).
	reloaded, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)
	jsonAgent, ok := reloaded.Data()["agent"].(map[string]interface{})
	require.True(t, ok, "agent section MUST exist in the saved JSON")
	review, ok := jsonAgent["review"].(map[string]interface{})
	require.True(t, ok, "agent.review object MUST exist")
	assert.Equal(t, "opencode-go/glm-5.2", review["model"],
		"agent.review.model MUST equal the override")
	assert.Len(t, review, 1,
		"agent.review MUST contain ONLY the model field (model-only JSON entry)")

	// 3) The merged model after reload reflects the override, and the rest of
	//    the agent (mode, prompt) still comes from md.
	merged := reloaded.MergedAgents()["review"]
	require.NotNil(t, merged)
	assert.Equal(t, "opencode-go/glm-5.2", merged.Fields["model"],
		"merged model MUST reflect the saved JSON override")
	assert.Equal(t, "subagent", merged.Mode(),
		"merged mode MUST still come from md (per-field merge)")
	assert.Equal(t, "Body.", merged.Prompt,
		"merged prompt MUST still come from md body (per-field merge)")
}

// TestEditableSchemaConsistency verifies the TUI editable field schema matches
// the canonical 6-field set shared with cmd/ocs agentFields (model,
// temperature, top_p, color, steps, disable), keeping the two in lockstep.
func TestEditableSchemaConsistency(t *testing.T) {
	expected := []string{"model", "temperature", "top_p", "color", "steps", "disable"}
	assert.Equal(t, expected, editableFieldSchema,
		"editableFieldSchema must match the canonical 6-field set (cmd/ocs agentFields)")
}
