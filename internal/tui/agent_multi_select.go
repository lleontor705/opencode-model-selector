// Package tui — multi-select screen implementation (REQ-TUI-002, REQ-TUI-003).
//
// This file implements the Agent Multi-Select screen (Flow B): a checkbox list
// of agents where SPACE toggles selection, ENTER confirms and routes to
// ScreenModelSelection with fieldEditing="bulk-list", and ESC cancels back
// to the Agent List.
//
// State (multiSelectItems, multiSelectChecked, multiSelectCursor) lives on the
// root Model (see model.go) and is populated by initAgentMultiSelectScreen.
//
// Spec coverage:
//   - REQ-TUI-002: 'm' key on AgentList pushes ScreenAgentMultiSelect
//   - REQ-TUI-003: checkbox list render + SPACE/j/k/ENTER/ESC keys
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// initAgentMultiSelectScreen populates the multi-select item list from the
// current Model's agent data. Disabled agents are excluded from selection
// (they cannot be mutated by SetAgentField anyway). Hidden agents are INCLUDED
// per Business Rule 4. Resets cursor and checked state.
//
// Items are derived from m.primaryAgents and m.subagents (already populated
// from GetAgents in NewModel, which excludes system agents). The disabled
// filter uses m.disabledAgents.
func initAgentMultiSelectScreen(m *Model) {
	disabled := make(map[string]bool, len(m.disabledAgents))
	for _, d := range m.disabledAgents {
		disabled[d] = true
	}
	items := make([]string, 0, len(m.primaryAgents)+len(m.subagents))
	for _, name := range m.primaryAgents {
		if !disabled[name] {
			items = append(items, name)
		}
	}
	for _, name := range m.subagents {
		if !disabled[name] {
			items = append(items, name)
		}
	}
	sort.Strings(items)
	m.multiSelectItems = items
	m.multiSelectChecked = make([]bool, len(items))
	m.multiSelectCursor = 0
}

// updateAgentMultiSelect handles keys on the multi-select screen.
//
// Keys:
//   - j/Down, k/Up: move cursor
//   - SPACE:        toggle checkbox at cursor
//   - ENTER:        confirm selection, transition to ScreenModelSelection
//     with fieldEditing="bulk-list" and bulkTargets populated.
//     If zero agents are checked, this is a no-op (the screen
//     stays and the status line shows the selection count).
//   - ESC:          cancel, pop back to AgentList without changes.
func updateAgentMultiSelect(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case keyMsg.Type == tea.KeyEsc || keyMsg.Type == tea.KeyEscape:
		m.multiSelectItems = nil
		m.multiSelectChecked = nil
		m.multiSelectCursor = 0
		m.popScreen()
		return m, nil

	case keyMsg.Type == tea.KeyEnter:
		var targets []string
		for i, name := range m.multiSelectItems {
			if m.multiSelectChecked[i] {
				targets = append(targets, name)
			}
		}
		if len(targets) == 0 {
			return m, nil
		}
		m.bulkTargets = targets
		m.multiSelectItems = nil
		m.multiSelectChecked = nil
		m.multiSelectCursor = 0
		m.pushScreen(ScreenModelSelection)
		m.fieldEditing = fieldEditingBulkList
		initModelSelectionScreen(&m)
		return m, nil

	case keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == ' ':
		if m.multiSelectCursor >= 0 && m.multiSelectCursor < len(m.multiSelectChecked) {
			m.multiSelectChecked[m.multiSelectCursor] = !m.multiSelectChecked[m.multiSelectCursor]
		}
		return m, nil

	case keyMsg.Type == tea.KeyDown ||
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'j'):
		if m.multiSelectCursor < len(m.multiSelectItems)-1 {
			m.multiSelectCursor++
		}
		return m, nil

	case keyMsg.Type == tea.KeyUp ||
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'k'):
		if m.multiSelectCursor > 0 {
			m.multiSelectCursor--
		}
		return m, nil

	default:
		return m, nil
	}
}

// viewAgentMultiSelect renders the multi-select screen. Layout:
//
//	[header with title "Select Agents"]
//
//	▶ [x] agent-foo
//	  [ ] agent-bar
//	  [x] agent-baz
//
//	[help: Space Toggle · Enter Confirm · Esc Cancel]
//	[status bar with selected count]
func viewAgentMultiSelect(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}
	header := renderHeader(m, "Select Agents")
	help := renderResponsiveHelp(m.width,
		"Space Toggle · Enter Confirm · Esc Cancel",
		"Space · Enter · Esc",
	)

	var b strings.Builder
	if len(m.multiSelectItems) == 0 {
		b.WriteString(HelpStyle.Render("No selectable agents"))
	} else {
		for i, name := range m.multiSelectItems {
			checkbox := "[ ]"
			if m.multiSelectChecked[i] {
				checkbox = "[x]"
			}
			cursor := "  "
			if i == m.multiSelectCursor {
				cursor = SelectedPrefix.Render("▶ ") + " "
			}
			line := fmt.Sprintf("%s%s %s", cursor, checkbox, name)
			if i == m.multiSelectCursor {
				b.WriteString(SelectedStyle.Render(line))
			} else {
				b.WriteString(AgentNormal.Render(line))
			}
			b.WriteByte('\n')
		}
	}

	selectedCount := 0
	for _, c := range m.multiSelectChecked {
		if c {
			selectedCount++
		}
	}
	status := renderStatusBar(m, "Select Agents", selectedCount)

	return strings.Join([]string{header, b.String(), help, status}, "\n")
}
