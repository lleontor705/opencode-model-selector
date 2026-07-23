// Package tui tests — field input screen (REQ-TUI-006).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in field_input.go, and drive its design. Coverage focuses on:
//   - validateFieldInput: per-field-type validation (temperature, top_p, color,
//     steps) including range checks, hex pattern, theme names, positive int.
//   - updateFieldInput: ENTER commits valid values to config, rejects invalid
//     values with an error message, ESC cancels without changes, typing updates
//     the textinput.
//   - viewFieldInput: rendering shows field name, current value, new value,
//     range hint, and help footer.
//   - initFieldInputScreen: pre-fills the textinput with the current value.
//
// Spec: REQ-TUI-006 (field input rendering and interaction).
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFieldInputModel constructs a Model positioned on the Field Input screen
// for the named agent and field. Centralizes boilerplate for every test in
// this file.
func newFieldInputModel(t *testing.T, agentName, fieldName string) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenFieldInput
	m.navigationStack = []appState{ScreenAgentDetail}
	m.selectedAgent = agentName
	m.fieldEditing = fieldName
	m.fieldInput.Focus()
	return m
}

// ---------------------------------------------------------------------------
// validateFieldInput — pure validation function (REQ-TUI-006)
// ---------------------------------------------------------------------------

// --- temperature ---

func TestValidateFieldInput_TemperatureValid(t *testing.T) {
	val, err := validateFieldInput("temperature", "0.5")
	require.NoError(t, err)
	assert.Equal(t, float64(0.5), val, "temperature 0.5 MUST parse as float64(0.5)")
}

func TestValidateFieldInput_TemperatureBoundaryZero(t *testing.T) {
	val, err := validateFieldInput("temperature", "0.0")
	require.NoError(t, err)
	assert.Equal(t, float64(0.0), val, "temperature 0.0 MUST be valid (inclusive lower bound)")
}

func TestValidateFieldInput_TemperatureBoundaryOne(t *testing.T) {
	val, err := validateFieldInput("temperature", "1.0")
	require.NoError(t, err)
	assert.Equal(t, float64(1.0), val, "temperature 1.0 MUST be valid (inclusive upper bound)")
}

func TestValidateFieldInput_TemperatureOutOfRangeHigh(t *testing.T) {
	_, err := validateFieldInput("temperature", "1.5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 0.0 and 1.0",
		"out-of-range temperature MUST mention the valid range")
}

func TestValidateFieldInput_TemperatureOutOfRangeNegative(t *testing.T) {
	_, err := validateFieldInput("temperature", "-0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 0.0 and 1.0")
}

func TestValidateFieldInput_TemperatureInvalidNumber(t *testing.T) {
	_, err := validateFieldInput("temperature", "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number",
		"non-numeric input MUST be rejected as invalid number")
}

// --- top_p ---

func TestValidateFieldInput_TopPValid(t *testing.T) {
	val, err := validateFieldInput("top_p", "0.9")
	require.NoError(t, err)
	assert.Equal(t, float64(0.9), val, "top_p 0.9 MUST parse as float64(0.9)")
}

func TestValidateFieldInput_TopPOutOfRange(t *testing.T) {
	_, err := validateFieldInput("top_p", "1.5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 0.0 and 1.0")
}

// --- color ---

func TestValidateFieldInput_ColorHexUppercase(t *testing.T) {
	val, err := validateFieldInput("color", "#FF5733")
	require.NoError(t, err)
	assert.Equal(t, "#FF5733", val, "valid hex color MUST return as string")
}

func TestValidateFieldInput_ColorHexLowercase(t *testing.T) {
	val, err := validateFieldInput("color", "#ff5733")
	require.NoError(t, err)
	assert.Equal(t, "#ff5733", val, "lowercase hex MUST be valid (case-insensitive)")
}

func TestValidateFieldInput_ColorThemeName(t *testing.T) {
	val, err := validateFieldInput("color", "primary")
	require.NoError(t, err)
	assert.Equal(t, "primary", val, "theme name MUST be valid and returned as string")
}

func TestValidateFieldInput_ColorThemeAccent(t *testing.T) {
	_, err := validateFieldInput("color", "accent")
	require.NoError(t, err, "'accent' is a valid theme name")
}

func TestValidateFieldInput_ColorInvalidString(t *testing.T) {
	_, err := validateFieldInput("color", "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color must be",
		"invalid color MUST be rejected with a descriptive message")
}

func TestValidateFieldInput_ColorInvalidHex(t *testing.T) {
	_, err := validateFieldInput("color", "#GGG")
	require.Error(t, err, "#GGG is not valid hex (G is not a hex digit)")
	assert.Contains(t, err.Error(), "color must be")
}

// --- steps ---

func TestValidateFieldInput_StepsValid(t *testing.T) {
	val, err := validateFieldInput("steps", "10")
	require.NoError(t, err)
	assert.Equal(t, 10, val, "steps 10 MUST parse as int(10)")
}

func TestValidateFieldInput_StepsZero(t *testing.T) {
	_, err := validateFieldInput("steps", "0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive integer",
		"steps 0 MUST be rejected (must be positive)")
}

func TestValidateFieldInput_StepsNegative(t *testing.T) {
	_, err := validateFieldInput("steps", "-5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive integer")
}

func TestValidateFieldInput_StepsNonNumeric(t *testing.T) {
	_, err := validateFieldInput("steps", "abc")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// updateFieldInput — ENTER commit (REQ-TUI-006)
// ---------------------------------------------------------------------------

func TestUpdateFieldInput_EnterTemperatureValid_CommitsAndReturns(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("0.5")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state,
		"ENTER on valid temperature MUST return to previousState")
	assert.True(t, newM.dirty, "dirty MUST be true after committing a change")

	// Verify the value was committed to config as float64.
	val, ok := newM.config.GetAgentField("code-reviewer", "temperature")
	require.True(t, ok, "temperature MUST be set on the agent after commit")
	assert.Equal(t, float64(0.5), val, "committed temperature MUST be float64(0.5)")
}

func TestUpdateFieldInput_EnterTemperatureOutOfRange_StaysOnScreen(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("1.5")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state,
		"invalid temperature MUST keep the user on the field input screen")
	assert.False(t, newM.dirty, "dirty MUST NOT be set on rejected input")
	assert.Contains(t, newM.saveError, "between 0.0 and 1.0",
		"saveError MUST contain the range message")
}

func TestUpdateFieldInput_EnterTemperatureNegative_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("-0.1")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state, "negative temperature MUST be rejected")
	assert.NotEmpty(t, newM.saveError)
}

func TestUpdateFieldInput_EnterTemperatureInvalid_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("abc")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state, "non-numeric temperature MUST be rejected")
	assert.Contains(t, newM.saveError, "invalid number")
}

func TestUpdateFieldInput_EnterTopPValid_Commits(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "top_p")
	m.fieldInput.SetValue("0.9")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state)
	assert.True(t, newM.dirty)

	val, ok := newM.config.GetAgentField("code-reviewer", "top_p")
	require.True(t, ok)
	assert.Equal(t, float64(0.9), val)
}

func TestUpdateFieldInput_EnterColorHex_Commits(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")
	m.fieldInput.SetValue("#FF5733")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state)

	val, ok := newM.config.GetAgentField("code-reviewer", "color")
	require.True(t, ok)
	assert.Equal(t, "#FF5733", val)
}

func TestUpdateFieldInput_EnterColorLowercaseHex_Commits(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")
	m.fieldInput.SetValue("#ff5733")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state,
		"lowercase hex MUST be accepted (case-insensitive)")

	val, ok := newM.config.GetAgentField("code-reviewer", "color")
	require.True(t, ok)
	assert.Equal(t, "#ff5733", val)
}

func TestUpdateFieldInput_EnterColorThemeName_Commits(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")
	m.fieldInput.SetValue("primary")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state)

	val, ok := newM.config.GetAgentField("code-reviewer", "color")
	require.True(t, ok)
	assert.Equal(t, "primary", val)
}

func TestUpdateFieldInput_EnterColorInvalid_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")
	m.fieldInput.SetValue("invalid")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state)
	assert.NotEmpty(t, newM.saveError)
}

func TestUpdateFieldInput_EnterColorInvalidHex_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "color")
	m.fieldInput.SetValue("#GGG")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state)
	assert.NotEmpty(t, newM.saveError)
}

func TestUpdateFieldInput_EnterStepsValid_Commits(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "steps")
	m.fieldInput.SetValue("10")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state)

	val, ok := newM.config.GetAgentField("code-reviewer", "steps")
	require.True(t, ok)
	assert.Equal(t, 10, val)
}

func TestUpdateFieldInput_EnterStepsZero_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "steps")
	m.fieldInput.SetValue("0")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state, "steps=0 MUST be rejected")
	assert.NotEmpty(t, newM.saveError)
}

func TestUpdateFieldInput_EnterStepsNegative_Rejected(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "steps")
	m.fieldInput.SetValue("-5")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenFieldInput, newM.state, "steps=-5 MUST be rejected")
	assert.NotEmpty(t, newM.saveError)
}

// ---------------------------------------------------------------------------
// updateFieldInput — ESC cancel (REQ-TUI-006)
// ---------------------------------------------------------------------------

func TestUpdateFieldInput_Esc_ReturnsWithoutChanges(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("0.5")
	require.False(t, m.dirty, "precondition: not dirty")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ScreenAgentDetail, newM.state,
		"ESC MUST return to previousState")
	assert.False(t, newM.dirty, "ESC MUST NOT set dirty")

	// No value should have been committed.
	_, ok := newM.config.GetAgentField("code-reviewer", "temperature")
	assert.False(t, ok, "ESC MUST NOT commit any value to config")
}

// ---------------------------------------------------------------------------
// updateFieldInput — typing and backspace (REQ-TUI-006)
// ---------------------------------------------------------------------------

func TestUpdateFieldInput_TypingAppendsCharacter(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	require.Empty(t, m.fieldInput.Value())

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Contains(t, newM.fieldInput.Value(), "x",
		"typing a character MUST append it to the field input value")
}

func TestUpdateFieldInput_TypingClearsError(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.saveError = "previous error"

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Empty(t, newM.saveError,
		"typing MUST clear any previous validation error")
}

func TestUpdateFieldInput_BackspaceDeletesCharacter(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("0.5")

	newM, _ := updateFieldInput(m, tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "0.", newM.fieldInput.Value(),
		"backspace MUST delete the last character")
}

// ---------------------------------------------------------------------------
// viewFieldInput — rendering (REQ-TUI-006)
// ---------------------------------------------------------------------------

func TestViewFieldInput_ShowsFieldName(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	out := viewFieldInput(m)
	assert.Contains(t, out, "temperature",
		"the rendered screen MUST show the field being edited")
}

func TestViewFieldInput_ShowsCurrentValueNone(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	out := viewFieldInput(m)
	assert.Contains(t, out, "(none)",
		"current value MUST show (none) when the field is unset")
}

func TestViewFieldInput_ShowsCurrentValueSet(t *testing.T) {
	m := newFieldInputModel(t, "plan", "temperature")
	out := viewFieldInput(m)
	assert.Contains(t, out, "0.4",
		"current value MUST show the existing value when set (plan has temperature 0.4)")
}

func TestViewFieldInput_ShowsHelpFooter(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	out := viewFieldInput(m)
	assert.Contains(t, out, "ENTER")
	assert.Contains(t, out, "ESC")
}

func TestViewFieldInput_ShowsRangeHint(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	out := viewFieldInput(m)
	assert.Contains(t, out, "0.0",
		"range hint MUST be shown for numeric fields")
}

func TestViewFieldInput_ShowsErrorWhenPresent(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.saveError = "temperature must be between 0.0 and 1.0"
	out := viewFieldInput(m)
	assert.Contains(t, out, "between 0.0 and 1.0",
		"validation error MUST be rendered when saveError is set")
}

func TestViewFieldInput_NilConfigDoesNotPanic(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.config = nil
	require.NotPanics(t, func() {
		out := viewFieldInput(m)
		assert.NotEmpty(t, out)
	})
}

// ---------------------------------------------------------------------------
// initFieldInputScreen — pre-fill (REQ-TUI-006)
// ---------------------------------------------------------------------------

func TestInitFieldInputScreen_PreFillsCurrentValue(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.selectedAgent = "plan"
	m.fieldEditing = "temperature"

	initFieldInputScreen(&m, "temperature")
	assert.Equal(t, "0.4", m.fieldInput.Value(),
		"initFieldInputScreen MUST pre-fill with the current value (plan has temperature 0.4)")
}

func TestInitFieldInputScreen_NoValueLeavesEmpty(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.selectedAgent = "code-reviewer"
	m.fieldEditing = "temperature"

	initFieldInputScreen(&m, "temperature")
	assert.Empty(t, m.fieldInput.Value(),
		"initFieldInputScreen MUST leave the input empty when no current value exists")
}

func TestInitFieldInputScreen_ClearsError(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.selectedAgent = "code-reviewer"
	m.fieldEditing = "temperature"
	m.saveError = "stale error"

	initFieldInputScreen(&m, "temperature")
	assert.Empty(t, m.saveError,
		"initFieldInputScreen MUST clear any stale saveError")
}

// ---------------------------------------------------------------------------
// Dispatcher integration — Update() routes to updateFieldInput (REQ-TUI-008)
// ---------------------------------------------------------------------------

func TestUpdate_DispatchesToFieldInput(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	m.fieldInput.SetValue("0.5")

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result, ok := newM.(Model)
	require.True(t, ok)
	assert.Equal(t, ScreenAgentDetail, result.state,
		"global Update() MUST dispatch ScreenFieldInput to updateFieldInput")
}

func TestView_DispatchesToFieldInput(t *testing.T) {
	m := newFieldInputModel(t, "code-reviewer", "temperature")
	out := m.View()
	assert.Contains(t, out, "temperature",
		"global View() MUST dispatch ScreenFieldInput to viewFieldInput")
}
