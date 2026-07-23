// Package tui — agent detail screen implementation (REQ-TUI-004).
//
// This file implements the Agent Detail screen: rendering and key handling.
// It shows the 6 editable fields for a single agent (model, temperature,
// top_p, color, steps, disable) and routes ENTER/SPACE to the appropriate
// sub-screen or toggle action.
//
// Visual layout:
//
//	╭─ opencode-model-selector ─────────────────────╮
//	│  Interactive model selector for OpenCode...   │
//	╰───────────────────────────────────────────────╯
//
//	Agent: code-reviewer (subagent)
//	Description: Code review subagent
//
//	── Editable Fields ──
//	  ▶ model       anthropic/claude-sonnet-4-20250514
//	    temperature (none)              (range 0.0–1.0)
//	    top_p       0.9
//	    color       #FF5733
//	    steps       10                  (positive integer)
//	    disable     ✗ disabled
//
//	[ <code-reviewer> ]                                      ? for help
//
// Spec coverage:
//   - REQ-TUI-004: Agent detail rendering (header, fields, cursor, warning)
//   - REQ-TUI-004: Navigation (j/k, arrows, ENTER, SPACE, ESC)
package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// fieldNone is the placeholder shown when a field has no value set.
const fieldNone = "(none)"

// viewAgentDetail renders the Agent Detail screen for the agent stored in
// m.selectedAgent.
func viewAgentDetail(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	var b strings.Builder

	agent := m.selectedAgent
	mode := m.config.GetAgentMode(agent)
	isDisabled := m.config.IsAgentDisabled(agent)

	// --- Header banner ---
	b.WriteString(renderHeader(m, "Agent: "+agent+" ("+mode+")"))
	b.WriteString("\n\n")

	// --- Description (only if present) ---
	if val, ok := m.config.GetAgentField(agent, "description"); ok {
		if desc, ok := val.(string); ok && desc != "" {
			b.WriteString(FieldLabel.Render("Description: ") + FieldValue.Render(desc))
			b.WriteString("\n\n")
		}
	}

	// --- Disabled warning ---
	if isDisabled {
		b.WriteString(ErrorStyle.Render("⚠ WARNING: agent is disabled — fields cannot be edited"))
		b.WriteString("\n\n")
	}

	// --- Editable Fields ---
	b.WriteString(SectionHeader.Render("◆ Editable Fields"))
	b.WriteByte('\n')

	for i, field := range m.editableFields {
		value := fieldDisplayValue(m, agent, field)
		hint := fieldContextHint(field)

		prefix := "  "
		if i == m.selectedField {
			prefix = SelectedPrefix.Render("▶ ") + " "
		}

		// Field row: "model: <value>" with optional hint.
		var valueStr string
		if field == "disable" {
			// Render booleans with ✓/✗ icons for at-a-glance recognition.
			if isDisabled {
				valueStr = BoolDisabled.Render("✗ disabled")
			} else {
				valueStr = BoolEnabled.Render("✓ enabled")
			}
		} else {
			valueStr = FieldValue.Render(value)
		}

		line := prefix +
			FieldLabel.Render(fmt.Sprintf("%-13s", field+":")) + " " +
			valueStr
		if hint != "" && field != "disable" {
			line += "  " + FieldHint.Render(hint)
		}

		if i == m.selectedField {
			line = SelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// --- Help footer ---
	b.WriteString("\n")
	b.WriteString(helpLine([]helpItem{
		{"j/k", "navigate"},
		{"ENTER", "edit"},
		{"SPACE", "toggle"},
		{"ESC", "back"},
	}))

	// --- Status bar ---
	b.WriteString("\n")
	b.WriteString(renderStatusBar(m, "Agent: "+agent, 0))

	return b.String()
}

// fieldContextHint returns a short parenthetical hint shown next to a field
// to guide the user on valid input. Returns "" when no hint is needed (e.g.
// for "model" where the picker UI does the guidance).
func fieldContextHint(field string) string {
	switch field {
	case "temperature", "top_p":
		return "(default: 0.7)"
	case "color":
		return "(hex or theme)"
	case "steps":
		return "(positive integer)"
	}
	return ""
}

// fieldDisplayValue resolves the display string for a single editable field of
// an agent. For the "disable" field it always reads IsAgentDisabled so the
// rendered value is authoritative. All other fields fall back to "(none)"
// when absent or empty.
func fieldDisplayValue(m Model, agent, field string) string {
	if field == "disable" {
		if m.config.IsAgentDisabled(agent) {
			return "true"
		}
		return "false"
	}

	val, ok := m.config.GetAgentField(agent, field)
	if !ok {
		return fieldNone
	}
	return formatFieldValue(val)
}

// formatFieldValue converts a raw config value (interface{}) into a display
// string. Strings are shown verbatim (empty → "(none)"). float64 values are
// rendered without unnecessary trailing zeros. All other types fall back to
// fmt's default formatting.
func formatFieldValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		if v == "" {
			return fieldNone
		}
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		s := fmt.Sprintf("%v", v)
		if s == "" {
			return fieldNone
		}
		return s
	}
}

// updateAgentDetail handles key presses on the Agent Detail screen.
//
// Keys:
//   - j / Down: selectedField down (stops at last field)
//   - k / Up:   selectedField up (stops at 0)
//   - ENTER:    model → ScreenModelSelection; temperature/top_p/color/steps →
//     ScreenFieldInput; disable → toggle (same as SPACE)
//   - SPACE:    disable → toggle; other fields → no-op
//   - ESC:      pop to previousState
//
// Spec: REQ-TUI-004 — interaction.
func updateAgentDetail(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	// --- ESC: pop navigation stack ---
	case msg.Type == tea.KeyEsc || msg.Type == tea.KeyEscape:
		m.state = m.previousState
		return m, nil

	// --- Cursor down ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j') ||
		msg.Type == tea.KeyDown:
		if m.selectedField < len(m.editableFields)-1 {
			m.selectedField++
		}
		return m, nil

	// --- Cursor up ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k') ||
		msg.Type == tea.KeyUp:
		if m.selectedField > 0 {
			m.selectedField--
		}
		return m, nil

	// --- ENTER: route by field type ---
	case msg.Type == tea.KeyEnter:
		return handleEnterOnField(m), nil

	// --- SPACE: toggle disable only ---
	case msg.Type == tea.KeySpace:
		field := currentField(m)
		if field == "disable" {
			toggleDisable(&m)
		}
		// Non-disable fields: no-op.
		return m, nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// handleEnterOnField routes an ENTER key press to the correct sub-screen based
// on the currently selected field. "model" opens the model picker; numeric/text
// fields open the field input; "disable" toggles.
func handleEnterOnField(m Model) Model {
	field := currentField(m)

	switch field {
	case "model":
		m.previousState = ScreenAgentDetail
		m.state = ScreenModelSelection
		m.fieldEditing = "model"
		initModelSelectionScreen(&m)

	case "temperature", "top_p", "color", "steps":
		m.previousState = ScreenAgentDetail
		m.state = ScreenFieldInput
		m.fieldEditing = field
		initFieldInputScreen(&m, field)

	case "disable":
		toggleDisable(&m)
	}

	return m
}

// currentField returns the field name at the current selectedField index, or an
// empty string if the index is out of bounds.
func currentField(m Model) string {
	if m.selectedField < 0 || m.selectedField >= len(m.editableFields) {
		return ""
	}
	return m.editableFields[m.selectedField]
}

// toggleDisable flips the agent's disable flag and records the change. If the
// agent is currently disabled, the config layer rejects the mutation (disabled
// agents are immutable per REQ-CFG-005); in that case the toggle is a no-op
// because the agent should not have been navigable in the first place.
func toggleDisable(m *Model) {
	agent := m.selectedAgent
	current := m.config.IsAgentDisabled(agent)
	if err := m.config.SetAgentField(agent, "disable", !current); err != nil {
		// Disabled agents are immutable at the config layer. Since disabled
		// agents are not navigable from the list, this path is defensive only.
		return
	}
	m.RecordChange(agent, "disable", current, !current)
}
