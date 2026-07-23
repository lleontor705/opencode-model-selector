// Package tui — agent list screen implementation (REQ-TUI-002, REQ-TUI-003).
//
// This file implements the main Agent List screen: rendering and key handling.
// It is the entry screen of the TUI and shows all user-facing agents grouped
// into Primary Agents and Subagents, plus the Global Default Model entry.
//
// Spec coverage:
//   - REQ-TUI-002: Agent list rendering (sections, model display, indicators)
//   - REQ-TUI-003: Navigation (j/k, arrows, ENTER, s, q)
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// globalItemKey is the sentinel value in the selectable items list that
// represents the Global Default Model entry. It is always the first selectable
// item. Rendering checks for this key to apply the "Global Default Model"
// label; ENTER checks for it to route to ScreenModelSelection.
const globalItemKey = "__global__"

// selectableItems returns the flat list of selectable items on the agent list
// screen. Disabled agents are excluded from selection; hidden agents are
// included.
//
// Ordering:
//  1. "__global__" (always first)
//  2. Non-disabled primary agents, sorted alphabetically
//  3. Non-disabled subagents, sorted alphabetically
//
// Disabled agents appear in the VISUAL list (via viewAgentList) but are NOT
// in this selectable list, so the cursor naturally skips them.
//
// Spec: REQ-TUI-002 — disabled non-selectable, hidden selectable.
func selectableItems(m Model) []string {
	items := make([]string, 0, len(m.primaryAgents)+len(m.subagents)+1)
	items = append(items, globalItemKey)

	disabled := make(map[string]bool, len(m.disabledAgents))
	for _, d := range m.disabledAgents {
		disabled[d] = true
	}

	for _, name := range sortedCopy(m.primaryAgents) {
		if !disabled[name] {
			items = append(items, name)
		}
	}
	for _, name := range sortedCopy(m.subagents) {
		if !disabled[name] {
			items = append(items, name)
		}
	}

	return items
}

// viewAgentList renders the agent list screen. The layout is:
//
//	[dirty] opencode-model-selector
//
//	[Global Default Model]
//	  model: <value or (none)>
//
//	── Primary Agents ──
//	  [cursor] agent-name  [DISABLED]
//	    model: <value or (none)>
//
//	── Subagents ──
//	  [cursor] agent-name  [H]
//	    model: <value or (none)>
//
//	j/k: navigate  ENTER: edit  s: save  q: quit
//
// Disabled agents appear visually (greyed) but are NOT selectable.
// Hidden agents appear with [H] and ARE selectable.
// System agents are already filtered out by GetAgents (never in primaryAgents
// or subagents).
//
// Spec: REQ-TUI-002.
func viewAgentList(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	var b strings.Builder

	// --- Title bar with dirty indicator ---
	title := TitleStyle.Render("opencode-model-selector")
	if m.dirty {
		b.WriteString(DirtyIndicator.Render("*") + " " + title)
	} else {
		b.WriteString(title)
	}
	b.WriteString("\n\n")

	// Build a disabled set for O(1) lookups during rendering.
	disabled := make(map[string]bool, len(m.disabledAgents))
	for _, d := range m.disabledAgents {
		disabled[d] = true
	}

	// selectableIdx tracks the current position in the selectable list so we
	// can apply the selection highlight to the correct visual row.
	selectableIdx := 0

	// --- Global Default Model entry ---
	globalModelVal := "(none)"
	if val, ok := m.config.GetGlobalModel(); ok && val != "" {
		globalModelVal = val
	}
	isGlobalSelected := selectableIdx == m.cursor
	b.WriteString(renderGlobalRow(globalModelVal, isGlobalSelected))
	b.WriteByte('\n')
	selectableIdx++

	// --- Primary Agents section ---
	b.WriteString(SectionHeader.Render("Primary Agents"))
	b.WriteByte('\n')
	for _, name := range sortedCopy(m.primaryAgents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.cursor {
				isSelected = true
			}
			selectableIdx++
		}
		b.WriteString(renderAgentRow(m, name, isDisabled, isSelected))
		b.WriteByte('\n')
	}

	// --- Subagents section ---
	b.WriteString(SectionHeader.Render("Subagents"))
	b.WriteByte('\n')
	for _, name := range sortedCopy(m.subagents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.cursor {
				isSelected = true
			}
			selectableIdx++
		}
		b.WriteString(renderAgentRow(m, name, isDisabled, isSelected))
		b.WriteByte('\n')
	}

	// --- Help footer ---
	b.WriteByte('\n')
	b.WriteString(HelpStyle.Render("j/k: navigate  ENTER: edit  s: save  q: quit"))

	// --- Quit confirmation overlay ---
	// When m.quitConfirm is true, append a confirmation prompt below the help
	// footer. The overlay renders on top of the existing agent list so the
	// user can still see what they would lose.
	if m.quitConfirm {
		b.WriteByte('\n')
		b.WriteString(ErrorStyle.Render("⚠ You have unsaved changes. Quit anyway? (y/n)"))
	}

	return b.String()
}

// renderGlobalRow renders the Global Default Model entry row.
func renderGlobalRow(modelVal string, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}
	content := prefix + "[Global Default Model]\n" +
		"    model: " + modelVal
	if isSelected {
		return SelectedStyle.Render(content)
	}
	return AgentNormal.Render(content)
}

// renderAgentRow renders a single agent row (name + model value). The style
// depends on whether the agent is disabled, hidden, or currently selected.
func renderAgentRow(m Model, name string, isDisabled, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	modelVal := "(none)"
	if val, ok := m.config.GetAgentField(name, "model"); ok {
		if s, ok := val.(string); ok && s != "" {
			modelVal = s
		}
	}

	content := prefix + name

	// Append indicator for hidden agents.
	if m.config.IsAgentHidden(name) {
		content += " [H]"
	}
	// Append indicator for disabled agents.
	if isDisabled {
		content += " [DISABLED]"
	}

	content += "\n    model: " + modelVal

	switch {
	case isDisabled:
		return AgentDisabled.Render(content)
	case isSelected:
		return SelectedStyle.Render(content)
	case m.config.IsAgentHidden(name):
		return AgentHidden.Render(content)
	default:
		return AgentNormal.Render(content)
	}
}

// updateAgentList handles key presses on the agent list screen.
//
// Keys:
//   - j / Down: cursor down (skips disabled agents)
//   - k / Up:   cursor up (skips disabled agents)
//   - ENTER:    on global → ScreenModelSelection; on agent → ScreenAgentDetail
//   - s:        transition to ScreenSaveConfirm (only if dirty)
//   - q / Ctrl+C: quit — when dirty, shows confirmation overlay first
//
// When m.quitConfirm is true, a sub-state takes over: only y/Y/ENTER confirm
// the quit, n/N/ESC cancel it, and all other keys are ignored.
//
// Spec: REQ-TUI-003.
func updateAgentList(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	// --- Quit confirmation sub-state ---
	// When active, intercept ALL keys. Only y/Y/ENTER confirm; n/N/ESC cancel;
	// everything else is ignored (including j/k navigation).
	if m.quitConfirm {
		switch {
		// Confirm quit: y, Y, or ENTER
		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 &&
			(msg.Runes[0] == 'y' || msg.Runes[0] == 'Y'):
			return m, tea.Quit
		case msg.Type == tea.KeyEnter:
			return m, tea.Quit

		// Cancel quit: n, N, or ESC
		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 &&
			(msg.Runes[0] == 'n' || msg.Runes[0] == 'N'):
			m.quitConfirm = false
			return m, nil
		case msg.Type == tea.KeyEsc || msg.Type == tea.KeyEscape:
			m.quitConfirm = false
			return m, nil

		// Any other key (including j, k, Ctrl+C, s) → ignored
		default:
			return m, nil
		}
	}

	switch {
	// --- Quit (q or Ctrl+C) ---
	// When dirty, show the confirmation overlay instead of quitting.
	// When clean, quit immediately.
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q') ||
		msg.Type == tea.KeyCtrlC:
		if m.dirty {
			m.quitConfirm = true
			return m, nil
		}
		return m, tea.Quit

	// --- ESC: pop to previousState (no-op on root since previousState defaults
	// to ScreenAgentList) ---
	case msg.Type == tea.KeyEsc || msg.Type == tea.KeyEscape:
		m.state = m.previousState
		return m, nil

	// --- Save ---
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's':
		if m.dirty {
			m.previousState = m.state
			m.state = ScreenSaveConfirm
		}
		return m, nil

	// --- Cursor down ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j') ||
		msg.Type == tea.KeyDown:
		items := selectableItems(m)
		if len(items) > 0 && m.cursor < len(items)-1 {
			m.cursor++
		}
		return m, nil

	// --- Cursor up ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k') ||
		msg.Type == tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	// --- ENTER: transition ---
	case msg.Type == tea.KeyEnter:
		items := selectableItems(m)
		if m.cursor >= 0 && m.cursor < len(items) {
			item := items[m.cursor]
			if item == globalItemKey {
				m.previousState = m.state
				m.state = ScreenModelSelection
				m.fieldEditing = "global"
				m.quitConfirm = false
				initModelSelectionScreen(&m)
			} else {
				m.selectedAgent = item
				m.previousState = m.state
				m.state = ScreenAgentDetail
				m.quitConfirm = false
			}
		}
		return m, nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// sortedCopy returns a sorted copy of the input slice. The original slice is
// not modified.
func sortedCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}
