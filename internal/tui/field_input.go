// Package tui — field input screen implementation (REQ-TUI-006).
//
// This file implements the Field Input screen: rendering and key handling.
// It captures free-form text for non-model fields (temperature, top_p, color,
// steps) with per-field validation, and commits valid values back to the
// config via SetAgentField.
//
// Validation rules (REQ-TUI-006):
//   - temperature: float in [0.0, 1.0] inclusive → float64
//   - top_p:       float in [0.0, 1.0] inclusive → float64
//   - color:       #RRGGBB hex (case-insensitive) OR theme name → string
//   - steps:       positive integer (> 0) → int
//
// Spec coverage:
//   - REQ-TUI-006: Field input rendering (header, current value, new value,
//     range hint, error, help)
//   - REQ-TUI-006: Validation per field type
//   - REQ-TUI-006: Interaction (typing, ENTER commit, ESC cancel, backspace)
package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// hexColorRegex matches a valid #RRGGBB hex color (case-insensitive).
var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// themeNames is the set of accepted theme name aliases for the color field.
// These allow users to reference semantic colors without knowing exact hex
// values.
var themeNames = map[string]bool{
	"primary":   true,
	"secondary": true,
	"accent":    true,
	"success":   true,
	"warning":   true,
	"error":     true,
	"info":      true,
}

// validateFieldInput validates the input string against the rules for the given
// field type. On success it returns the parsed Go value (float64 for
// temperature/top_p, string for color, int for steps) and nil. On failure it
// returns nil and a descriptive error.
//
// Spec: REQ-TUI-006 — per-field-type validation.
func validateFieldInput(fieldType, value string) (interface{}, error) {
	switch fieldType {
	case "temperature", "top_p":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number")
		}
		if f < 0.0 || f > 1.0 {
			return nil, fmt.Errorf("%s must be between 0.0 and 1.0", fieldType)
		}
		return f, nil

	case "color":
		if hexColorRegex.MatchString(value) {
			return value, nil
		}
		if themeNames[value] {
			return value, nil
		}
		return nil, fmt.Errorf("color must be #RRGGBB hex or theme name")

	case "steps":
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("steps must be a positive integer")
		}
		if n <= 0 {
			return nil, fmt.Errorf("steps must be a positive integer")
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown field type: %s", fieldType)
	}
}

// fieldHint returns a human-readable hint string for the given field type,
// shown below the input to guide the user on valid input ranges/formats.
func fieldHint(fieldType string) string {
	switch fieldType {
	case "temperature", "top_p":
		return "Range: 0.0 - 1.0"
	case "color":
		return "Format: #RRGGBB hex or theme name"
	case "steps":
		return "Range: positive integer (> 0)"
	default:
		return ""
	}
}

// fieldCurrentValue resolves the display string for the current value of the
// field being edited. Returns "(none)" when the field is unset.
func fieldCurrentValue(m Model) string {
	if m.config == nil {
		return fieldNone
	}
	val, ok := m.config.GetAgentField(m.selectedAgent, m.fieldEditing)
	if !ok {
		return fieldNone
	}
	return formatFieldValue(val)
}

// viewFieldInput renders the Field Input screen. The layout is:
//
//	[dirty] Edit: <field>
//
//	Current: <value or (none)>
//
//	New value: <input>
//	<range/format hint>
//
//	<error message if any>
//
//	ENTER: save  ESC: cancel
//
// Spec: REQ-TUI-006 — rendering.
func viewFieldInput(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	var b strings.Builder

	// --- Header ---
	header := "Edit: " + m.fieldEditing
	if m.dirty {
		b.WriteString(DirtyIndicator.Render("*") + " " + TitleStyle.Render(header))
	} else {
		b.WriteString(TitleStyle.Render(header))
	}
	b.WriteString("\n\n")

	// --- Current value ---
	b.WriteString(FieldLabel.Render("Current: "))
	b.WriteString(FieldValue.Render(fieldCurrentValue(m)))
	b.WriteString("\n\n")

	// --- New value input ---
	b.WriteString(FieldLabel.Render("New value: "))
	b.WriteString(m.fieldInput.View())
	b.WriteByte('\n')

	// --- Range / format hint ---
	if hint := fieldHint(m.fieldEditing); hint != "" {
		b.WriteString(HelpStyle.Render(hint))
		b.WriteByte('\n')
	}

	// --- Error message ---
	if m.saveError != "" {
		b.WriteByte('\n')
		b.WriteString(ErrorStyle.Render(m.saveError))
		b.WriteByte('\n')
	}

	// --- Help footer ---
	b.WriteByte('\n')
	b.WriteString(HelpStyle.Render("Enter Apply · Esc Discard"))

	return b.String()
}

// updateFieldInput handles key presses on the Field Input screen.
//
// Keys:
//   - typing:    appends characters to fieldInput, clears any stale error
//   - Backspace: deletes from fieldInput, clears error
//   - ENTER:     validates input; on success commits to config and returns to
//     its immutable origin; on failure shows error and stays on screen
//   - ESC:       cancels, returns to its immutable origin without changes
//
// Non-key messages (e.g. cursor blink) are forwarded to the textinput.
//
// Spec: REQ-TUI-006 — interaction.
func updateFieldInput(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages (cursor blink, etc.) to textinput.
		var cmd tea.Cmd
		m.fieldInput, cmd = m.fieldInput.Update(msg)
		return m, cmd
	}

	switch {
	// --- ESC: cancel, return to immutable origin ---
	case keyMsg.Type == tea.KeyEsc || keyMsg.Type == tea.KeyEscape:
		m.popScreen()
		m.saveError = ""
		return m, nil

	// --- ENTER: validate and commit ---
	case keyMsg.Type == tea.KeyEnter:
		return commitFieldInput(m), nil

	// --- Backspace: delete character ---
	case keyMsg.Type == tea.KeyBackspace:
		m.fieldInput, _ = m.fieldInput.Update(keyMsg)
		m.saveError = ""
		return m, nil

	// --- Typing: append characters ---
	case keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) > 0:
		m.fieldInput, _ = m.fieldInput.Update(keyMsg)
		m.saveError = ""
		return m, nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// commitFieldInput validates the current fieldInput value, commits it to the
// config on success, or sets saveError on failure. On success it records the
// change, marks the model dirty, and transitions back to its immutable origin.
func commitFieldInput(m Model) Model {
	value := m.fieldInput.Value()
	parsed, err := validateFieldInput(m.fieldEditing, value)
	if err != nil {
		m.saveError = err.Error()
		return m
	}

	// Capture the previous value BEFORE writing so the diff reflects the
	// actual mutation, not the new value on both sides.
	oldVal, _ := m.config.GetAgentField(m.selectedAgent, m.fieldEditing)

	// Commit to config. SetAgentField may reject if the agent is disabled,
	// but disabled agents are not navigable to this screen in normal flow.
	if err := m.config.SetAgentField(m.selectedAgent, m.fieldEditing, parsed); err != nil {
		m.saveError = err.Error()
		return m
	}

	m.RecordChange(m.selectedAgent, m.fieldEditing, oldVal, parsed)
	m.saveError = ""
	m.popScreen()
	return m
}

// initFieldInputScreen initializes the field input screen for the named field:
// creates a fresh textinput, focuses it, pre-fills with the current value if
// one exists, and clears any stale error.
//
// Spec: REQ-TUI-006 — pre-fill with current value.
func initFieldInputScreen(m *Model, fieldName string) {
	m.fieldInput = textinput.New()
	m.fieldInput.Focus()

	// Pre-fill with the current value so the user can edit rather than
	// re-type from scratch.
	if m.config != nil {
		if val, ok := m.config.GetAgentField(m.selectedAgent, fieldName); ok {
			m.fieldInput.SetValue(formatFieldValue(val))
		}
	}

	m.saveError = ""
}
