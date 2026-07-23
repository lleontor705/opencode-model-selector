// Package tui tests — agent detail screen (REQ-TUI-004).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in agent_detail.go, and drive its design. Coverage focuses on:
//   - Rendering: header (agent name + mode), description, 6 editable fields with
//     current values, cursor highlight, disabled-agent warning.
//   - Navigation: j/k and arrows move selectedField, wrap/stop at boundaries.
//   - Interaction: ENTER routes by field type, SPACE toggles disable, ESC pops.
//
// Spec: REQ-TUI-004 (agent detail rendering and interaction).
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDetailModel constructs a Model positioned on the Agent Detail screen for
// the named agent. It centralizes the boilerplate that every test in this file
// needs: state, selectedAgent, previousState, and a valid config.
func newDetailModel(t *testing.T, agentName string) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenAgentDetail
	m.selectedAgent = agentName
	m.selectedField = 0 // "model" by default
	m.previousState = ScreenAgentList
	return m
}

// ---------------------------------------------------------------------------
// Rendering — viewAgentDetail (REQ-TUI-004)
// ---------------------------------------------------------------------------

// TestViewAgentDetail_ShowsAgentNameAndMode verifies that the header shows the
// agent name and its mode.
//
// Spec: REQ-TUI-004 — Happy path — header with agent name and mode.
func TestViewAgentDetail_ShowsAgentNameAndMode(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)

	assert.Contains(t, out, "code-reviewer",
		"the agent name MUST appear in the detail screen header")
	assert.Contains(t, out, "subagent",
		"the agent mode MUST appear in the detail screen header")
}

// TestViewAgentDetail_ShowsDescription verifies that the agent description
// appears when one is configured.
//
// Spec: REQ-TUI-004 — Happy path — description shown if available.
func TestViewAgentDetail_ShowsDescription(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "Code review subagent",
		"the description MUST be shown when the agent has one")
}

// TestViewAgentDetail_ShowsAllSixEditableFields verifies that every editable
// field label appears in the rendered output.
//
// Spec: REQ-TUI-004 — Happy path — show 6 editable fields.
func TestViewAgentDetail_ShowsAllSixEditableFields(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)

	for _, field := range []string{"model", "temperature", "top_p", "color", "steps", "disable"} {
		assert.Contains(t, out, field,
			"field %q MUST appear in the editable fields list", field)
	}
}

// TestViewAgentDetail_ModelValueShown verifies that when an agent has a model
// set, its value is displayed next to the model field.
//
// Spec: REQ-TUI-004 — Happy path — model value displayed.
func TestViewAgentDetail_ModelValueShown(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "anthropic/claude-sonnet-4-20250514",
		"the model value MUST be shown when the agent has one")
}

// TestViewAgentDetail_TemperatureNoneWhenUnset verifies that "(none)" is shown
// for temperature when the agent does not have it set.
//
// Spec: REQ-TUI-004 — Happy path — (none) shown when value unset.
func TestViewAgentDetail_TemperatureNoneWhenUnset(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "(none)",
		"(none) MUST be shown for temperature when the agent has no value")
}

// TestViewAgentDetail_TemperatureValueShownForPlan verifies that when an agent
// has a numeric temperature (float), the value is rendered correctly.
func TestViewAgentDetail_TemperatureValueShownForPlan(t *testing.T) {
	m := newDetailModel(t, "plan")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "0.4",
		"the temperature value MUST be shown when set (plan has temperature 0.4)")
}

// TestViewAgentDetail_DisableFalseWhenNotDisabled verifies that the disable
// field shows a "✓ enabled" indicator for a non-disabled agent.
//
// Spec: REQ-TUI-004 — Happy path — disable value displayed.
func TestViewAgentDetail_DisableFalseWhenNotDisabled(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "✓ enabled",
		"disable field MUST show '✓ enabled' for a non-disabled agent")
}

// TestViewAgentDetail_DisabledAgentShowsWarning verifies that when the selected
// agent is disabled, a warning is displayed indicating fields cannot be edited.
//
// Spec: REQ-TUI-004 — Edge case — disabled agent warning.
func TestViewAgentDetail_DisabledAgentShowsWarning(t *testing.T) {
	m := newDetailModel(t, "build")
	out := viewAgentDetail(m)
	// "build" has disable: true — the screen MUST warn about read-only state.
	assert.Contains(t, out, "build",
		"disabled agent name MUST still appear in the detail screen")
	// The warning text should indicate the agent is disabled / not editable.
	assert.True(t,
		containsAny(out, "disabled", "DISABLED", "cannot be edited", "read-only"),
		"a warning about non-editability MUST be shown for disabled agents")
}

// TestViewAgentDetail_DisabledAgentShowsDisableTrue verifies that for a disabled
// agent, the disable field displays "✗ disabled".
func TestViewAgentDetail_DisabledAgentShowsDisableTrue(t *testing.T) {
	m := newDetailModel(t, "build")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "✗ disabled",
		"disable field MUST show '✗ disabled' for a disabled agent")
}

// TestViewAgentDetail_ReturnsNonEmpty verifies a basic non-empty contract.
func TestViewAgentDetail_ReturnsNonEmpty(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.NotEmpty(t, out, "viewAgentDetail MUST return a non-empty string")
}

// TestViewAgentDetail_HelpFooterPresent verifies that keybinding help is shown.
func TestViewAgentDetail_HelpFooterPresent(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := viewAgentDetail(m)
	assert.Contains(t, out, "ENTER",
		"help footer MUST mention ENTER for editing")
	assert.Contains(t, out, "ESC",
		"help footer MUST mention ESC for going back")
}

// ---------------------------------------------------------------------------
// Navigation — updateAgentDetail (REQ-TUI-004)
// ---------------------------------------------------------------------------

// TestUpdateAgentDetail_J_MovesFieldDown verifies that 'j' increments
// selectedField.
//
// Spec: REQ-TUI-004 — Happy path — j navigates down.
func TestUpdateAgentDetail_J_MovesFieldDown(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	require.Equal(t, 0, m.selectedField, "precondition: selectedField starts at 0")

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, newM.selectedField, "selectedField MUST be 1 after pressing 'j'")
}

// TestUpdateAgentDetail_J_StopsAtLastField verifies that pressing 'j' at the
// last field index (5) does not exceed the field count.
//
// Spec: REQ-TUI-004 — Edge case — no wrap past last field.
func TestUpdateAgentDetail_J_StopsAtLastField(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 5 // "disable" — last field

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 5, newM.selectedField,
		"selectedField MUST NOT exceed the last index (5) when pressing 'j'")
}

// TestUpdateAgentDetail_K_MovesFieldUp verifies that 'k' decrements
// selectedField.
//
// Spec: REQ-TUI-004 — Happy path — k navigates up.
func TestUpdateAgentDetail_K_MovesFieldUp(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 2

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, newM.selectedField, "selectedField MUST be 1 after pressing 'k' from 2")
}

// TestUpdateAgentDetail_K_AtTopStaysAtZero verifies that pressing 'k' at
// selectedField 0 keeps it at 0.
func TestUpdateAgentDetail_K_AtTopStaysAtZero(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 0

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, newM.selectedField,
		"selectedField MUST stay at 0 when pressing 'k' at the top")
}

// TestUpdateAgentDetail_DownArrow_MovesDown verifies the Down arrow works like
// 'j'.
func TestUpdateAgentDetail_DownArrow_MovesDown(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, newM.selectedField, "Down arrow MUST move selectedField down")
}

// TestUpdateAgentDetail_UpArrow_MovesUp verifies the Up arrow works like 'k'.
func TestUpdateAgentDetail_UpArrow_MovesUp(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 3
	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 2, newM.selectedField, "Up arrow MUST move selectedField up")
}

// ---------------------------------------------------------------------------
// ENTER — field-type-specific transitions (REQ-TUI-004)
// ---------------------------------------------------------------------------

// TestUpdateAgentDetail_EnterOnModel_TransitionsToModelSelection verifies that
// ENTER on the "model" field transitions to ScreenModelSelection.
//
// Spec: REQ-TUI-004 — Happy path — ENTER on model opens model picker.
func TestUpdateAgentDetail_EnterOnModel_TransitionsToModelSelection(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 0 // "model"

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state,
		"ENTER on 'model' MUST transition to ScreenModelSelection")
	assert.Equal(t, "model", newM.fieldEditing,
		"fieldEditing MUST be 'model' after ENTER on model field")
	assert.Equal(t, ScreenAgentDetail, newM.previousState,
		"previousState MUST be ScreenAgentDetail so ESC returns here")
}

// TestUpdateAgentDetail_EnterOnTemperature_TransitionsToFieldInput verifies that
// ENTER on the "temperature" field transitions to ScreenFieldInput.
//
// Spec: REQ-TUI-004 — Happy path — ENTER on numeric field opens field input.
func TestUpdateAgentDetail_EnterOnTemperature_TransitionsToFieldInput(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 1 // "temperature"

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state,
		"ENTER on 'temperature' MUST transition to ScreenFieldInput")
	assert.Equal(t, "temperature", newM.fieldEditing,
		"fieldEditing MUST be 'temperature'")
}

// TestUpdateAgentDetail_EnterOnColor_TransitionsToFieldInput verifies that ENTER
// on the "color" field transitions to ScreenFieldInput.
func TestUpdateAgentDetail_EnterOnColor_TransitionsToFieldInput(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 3 // "color"

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state,
		"ENTER on 'color' MUST transition to ScreenFieldInput")
	assert.Equal(t, "color", newM.fieldEditing,
		"fieldEditing MUST be 'color'")
}

// TestUpdateAgentDetail_EnterOnSteps_TransitionsToFieldInput verifies that ENTER
// on the "steps" field transitions to ScreenFieldInput.
func TestUpdateAgentDetail_EnterOnSteps_TransitionsToFieldInput(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 4 // "steps"

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state,
		"ENTER on 'steps' MUST transition to ScreenFieldInput")
	assert.Equal(t, "steps", newM.fieldEditing,
		"fieldEditing MUST be 'steps'")
}

// TestUpdateAgentDetail_EnterOnDisable_Toggles verifies that ENTER on the
// "disable" field toggles the disable value and marks dirty.
//
// Spec: REQ-TUI-004 — Happy path — ENTER on disable toggles.
func TestUpdateAgentDetail_EnterOnDisable_Toggles(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 5 // "disable"
	require.False(t, m.config.IsAgentDisabled("code-reviewer"),
		"precondition: code-reviewer must NOT be disabled before toggle")

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, newM.config.IsAgentDisabled("code-reviewer"),
		"ENTER on disable MUST toggle the agent to disabled")
	assert.True(t, newM.dirty,
		"dirty MUST be true after toggling disable")
}

// ---------------------------------------------------------------------------
// SPACE — disable toggle only (REQ-TUI-004)
// ---------------------------------------------------------------------------

// TestUpdateAgentDetail_SpaceOnDisable_Toggles verifies that SPACE on the
// "disable" field toggles the value and marks dirty.
//
// Spec: REQ-TUI-004 — Happy path — SPACE on disable toggles.
func TestUpdateAgentDetail_SpaceOnDisable_Toggles(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 5 // "disable"
	require.False(t, m.config.IsAgentDisabled("code-reviewer"),
		"precondition: code-reviewer must NOT be disabled before toggle")

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.True(t, newM.config.IsAgentDisabled("code-reviewer"),
		"SPACE on disable MUST toggle the agent to disabled")
	assert.True(t, newM.dirty,
		"dirty MUST be true after toggling disable via SPACE")
}

// TestUpdateAgentDetail_SpaceOnModel_NoOp verifies that SPACE on a non-disable
// field (model) is a no-op.
//
// Spec: REQ-TUI-004 — Edge case — SPACE only works for disable.
func TestUpdateAgentDetail_SpaceOnModel_NoOp(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 0 // "model"
	require.False(t, m.dirty, "precondition: model must not be dirty")

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, ScreenAgentDetail, newM.state,
		"SPACE on model MUST NOT change state")
	assert.False(t, newM.dirty,
		"SPACE on model MUST NOT set dirty")
	assert.Equal(t, 0, newM.selectedField,
		"SPACE on model MUST NOT move selectedField")
}

// ---------------------------------------------------------------------------
// ESC — navigation pop (REQ-TUI-004, REQ-TUI-008)
// ---------------------------------------------------------------------------

// TestUpdateAgentDetail_Esc_ReturnsToAgentList verifies that ESC pops back to
// the previous screen (ScreenAgentList).
//
// Spec: REQ-TUI-004 — Happy path — ESC returns to agent list.
func TestUpdateAgentDetail_Esc_ReturnsToAgentList(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.previousState = ScreenAgentList

	newM, _ := updateAgentDetail(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ScreenAgentList, newM.state,
		"ESC MUST return to previousState (ScreenAgentList)")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsAny returns true if any of the substrings appear in s.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Dispatcher integration — Update() routes to updateAgentDetail (REQ-TUI-008)
// ---------------------------------------------------------------------------

// TestUpdate_DispatchesToAgentDetail verifies that the global Update() function
// routes ScreenAgentDetail key presses to updateAgentDetail by checking that 'j'
// moves the field cursor.
func TestUpdate_DispatchesToAgentDetail(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	m.selectedField = 0

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	result, ok := newM.(Model)
	require.True(t, ok, "Update must return the same Model type")
	assert.Equal(t, 1, result.selectedField,
		"global Update() MUST dispatch ScreenAgentDetail to updateAgentDetail")
}

// TestView_DispatchesToAgentDetail verifies that the global View() function
// routes ScreenAgentDetail to viewAgentDetail.
func TestView_DispatchesToAgentDetail(t *testing.T) {
	m := newDetailModel(t, "code-reviewer")
	out := m.View()
	assert.Contains(t, out, "code-reviewer",
		"global View() MUST dispatch ScreenAgentDetail to viewAgentDetail")
}
