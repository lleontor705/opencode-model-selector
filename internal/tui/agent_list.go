// Package tui — agent list screen implementation (REQ-TUI-002, REQ-TUI-003).
//
// This file implements the main Agent List screen: rendering and key handling.
// It is the entry screen of the TUI and shows all user-facing agents grouped
// into Primary Agents and Subagents, plus the Global Default Model entry.
//
// Visual layout (top → bottom):
//
//	╭─ opencode-model-selector ─────────────────────╮
//	│  Interactive model selector for OpenCode...   │
//	╰───────────────────────────────────────────────╯
//
//	[Global Default Model]
//	  model: <value or (none)>
//
//	── Primary Agents ──
//	  ▶ agent-name  [DISABLED]
//	    model:       <value or (none)>
//	    temperature: <value or (none)>
//	    top_p:       <value or (none)>
//	    color:       <value or (none)>
//	    steps:       <value or (none)>
//	    disable:     <value or (none)>
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//	    ...same six fields...
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
//
// All six configurable fields (model, temperature, top_p, color, steps,
// disable) are shown per agent so users can audit their full config without
// drilling into each agent. (REQ-TUI-002)
//
// Spec coverage:
//   - REQ-TUI-002: Agent list rendering (sections, model display, indicators)
//   - REQ-TUI-003: Navigation (j/k, arrows, ENTER, s, q)
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// globalItemKey is the sentinel value in the selectable items list that
// represents the Global Default Model entry. It is always the first selectable
// item. Rendering checks for this key to apply the "Global Default Model"
// label; ENTER checks for it to route to ScreenModelSelection.
const globalItemKey = "__global__"

// agentListScreenLabel is the human-readable screen name shown in the status
// bar. Centralized here so the status bar text stays consistent if the screen
// is renamed.
const agentListScreenLabel = "Agents"

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
//	  ▶ agent-name  [DISABLED]
//	    model:       <value or (none)>
//	    temperature: <value or (none)>
//	    top_p:       <value or (none)>
//	    color:       <value or (none)>
//	    steps:       <value or (none)>
//	    disable:     <value or (none)>
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//	    ...same six fields...
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
//
// Disabled agents appear visually (greyed) but are NOT selectable.
// Hidden agents appear with [H] and ARE selectable.
// System agents are already filtered out by GetAgents (never in primaryAgents
// or subagents).
//
// All six fields per agent are shown so users can verify their full config
// from the list without drilling into each agent. (REQ-TUI-002)
//
// Spec: REQ-TUI-002.
func viewAgentList(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	var b strings.Builder

	// --- Header banner ---
	b.WriteString(renderHeader(m, "Agents"))

	// --- Success banner (if just saved) ---
	if m.saveSuccess {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render("✓ Saved successfully!"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

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
	isGlobalSelected := selectableIdx == m.agentCursor
	b.WriteString(renderGlobalRow(globalModelVal, isGlobalSelected))
	b.WriteByte('\n')
	selectableIdx++

	// --- Primary Agents section ---
	b.WriteString(SectionHeader.Render("◆ Primary Agents"))
	b.WriteByte('\n')
	for _, name := range sortedCopy(m.primaryAgents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.agentCursor {
				isSelected = true
			}
			selectableIdx++
		}
		b.WriteString(renderAgentRow(m, name, isDisabled, isSelected))
		b.WriteByte('\n')
	}

	// --- Subagents section ---
	b.WriteString(SectionHeader.Render("◆ Subagents"))
	b.WriteByte('\n')
	for _, name := range sortedCopy(m.subagents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.agentCursor {
				isSelected = true
			}
			selectableIdx++
		}
		b.WriteString(renderAgentRow(m, name, isDisabled, isSelected))
		b.WriteByte('\n')
	}

	// --- Quit confirmation overlay (rendered above the status bar so it is
	// always visible to the user, but BELOW the section content so the user
	// still sees what they would lose). ---
	if m.quitConfirm {
		b.WriteByte('\n')
		b.WriteString(ErrorStyle.Render("⚠ You have unsaved changes. Quit anyway? (y/n)"))
		b.WriteByte('\n')
	}

	// --- Help footer (keybinding hints) ---
	b.WriteByte('\n')
	b.WriteString(helpLine([]helpItem{
		{"j/k", "navigate"},
		{"ENTER", "edit"},
		{"s", "save"},
		{"q", "quit"},
	}))

	// --- Status bar (persistent footer) ---
	b.WriteString("\n")
	b.WriteString(renderStatusBar(m, agentListScreenLabel, len(m.primaryAgents)+len(m.subagents)))

	return b.String()
}

// renderGlobalRow renders the Global Default Model entry row.
func renderGlobalRow(modelVal string, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = SelectedPrefix.Render("▶ ") + " "
	}
	content := prefix + FieldLabel.Render("[Global Default Model]") + "\n" +
		"    model: " + FieldValue.Render(modelVal)
	if isSelected {
		return SelectedStyle.Render(content)
	}
	return AgentNormal.Render(content)
}

// agentListFields is the ordered set of fields every agent row exposes in the
// agent list view. Order is fixed: model first (most important), then
// tuning knobs, then visual (color), behavior (steps), and the disable flag.
var agentListFields = []string{
	"model",
	"temperature",
	"top_p",
	"color",
	"steps",
	"disable",
}

// compactFieldValue resolves a single field for an agent and renders it as a
// short string for the agent list. Returns "(none)" if the field is missing
// or its value is nil. Empty strings also render as "" (caller decides) —
// for the agent list, we never want empty strings to look like a real value,
// so we collapse them to "(none)" too.
func compactFieldValue(m Model, name, field string) string {
	val, ok := m.config.GetAgentField(name, field)
	if !ok || val == nil {
		return "(none)"
	}
	s := fmt.Sprintf("%v", val)
	if s == "" {
		return "(none)"
	}
	return s
}

// renderAgentRow renders a single agent row — the agent name plus all six
// configurable fields (model, temperature, top_p, color, steps, disable) —
// one per line, aligned for readability. The style depends on whether the
// agent is disabled, hidden, or currently selected.
//
// Layout:
//
//	  [cursor] name  [H] | [DISABLED]
//	    model:       <value or (none)>
//	    temperature: <value or (none)>
//	    top_p:       <value or (none)>
//	    color:       <value or (none)>
//	    steps:       <value or (none)>
//	    disable:     <value or (none)>
//
// Spec: REQ-TUI-002 — agent list rendering shows full per-agent config.
func renderAgentRow(m Model, name string, isDisabled, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = SelectedPrefix.Render("▶ ") + " "
	}

	nameLine := prefix + name

	// Append indicator for hidden agents.
	if m.config.IsAgentHidden(name) {
		nameLine += " " + HelpStyle.Render("[H]")
	}
	// Append indicator for disabled agents.
	if isDisabled {
		nameLine += " " + ErrorStyle.Render("[DISABLED]")
	}

	var b strings.Builder
	b.WriteString(nameLine)
	b.WriteByte('\n')

	// Render all six fields, label-padded for visual alignment. Field labels
	// are plain; only the value is themed with FieldValue.
	for _, field := range agentListFields {
		val := compactFieldValue(m, name, field)
		fmt.Fprintf(&b, "    %-13s %s\n", field+":", FieldValue.Render(val))
	}

	content := b.String()
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

	// --- ESC: quit from the root, guarded when there are unsaved changes ---
	case msg.Type == tea.KeyEsc || msg.Type == tea.KeyEscape:
		if m.dirty {
			m.quitConfirm = true
			return m, nil
		}
		return m, tea.Quit

	// --- Save ---
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 's':
		if m.dirty {
			m.pushScreen(ScreenSaveConfirm)
		}
		return m, nil

	// --- Cursor down ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j') ||
		msg.Type == tea.KeyDown:
		items := selectableItems(m)
		if len(items) > 0 && m.agentCursor < len(items)-1 {
			m.agentCursor++
		}
		return m, nil

	// --- Cursor up ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k') ||
		msg.Type == tea.KeyUp:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		return m, nil

	// --- ENTER: transition ---
	case msg.Type == tea.KeyEnter:
		items := selectableItems(m)
		if m.agentCursor >= 0 && m.agentCursor < len(items) {
			item := items[m.agentCursor]
			if item == globalItemKey {
				m.pushScreen(ScreenModelSelection)
				m.fieldEditing = "global"
				m.quitConfirm = false
				initModelSelectionScreen(&m)
			} else {
				m.selectedAgent = item
				m.pushScreen(ScreenAgentDetail)
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
