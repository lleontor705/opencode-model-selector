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
//	  ▶ agent-name [DISABLED] · model <value or (none)>
//	    temp .4 · top_p .9 · color #FF5733 · steps 10
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//	    ...same compact configured-value summary...
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
//
// Rows stay compact; Agent Detail remains the complete six-field editor.
//
// Spec coverage:
//   - REQ-TUI-002: Agent list rendering (sections, model display, indicators)
//   - REQ-TUI-003: Navigation (j/k, arrows, ENTER, s, q)
package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
//	  ▶ agent-name [DISABLED] · model <value or (none)>
//	    temp .4 · top_p .9 · color #FF5733 · steps 10
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//	    ...same compact configured-value summary...
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
//
// Disabled agents appear visually (greyed) but are NOT selectable.
// Hidden agents appear with [H] and ARE selectable.
// System agents are already filtered out by GetAgents (never in primaryAgents
// or subagents).
//
// Only configured optional values appear in the list. The Agent Detail screen
// retains all six editable fields. (REQ-TUI-002)
//
// Spec: REQ-TUI-002.
func viewAgentList(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}
	if m.width <= 0 || m.height <= 0 {
		content, _, _ := renderAgentListContent(m)
		parts := []string{renderHeader(m, "Agents")}
		if m.saveSuccess {
			parts = append(parts, SuccessStyle.Render("✓ Saved successfully"))
		}
		parts = append(parts, content)
		if m.quitConfirm {
			parts = append(parts, ErrorStyle.Render("⚠ You have unsaved changes. Quit anyway? (y/n)"))
		}
		return strings.Join(append(parts, agentListHelp(m.width), renderStatusBar(m, agentListScreenLabel, len(m.primaryAgents)+len(m.subagents))), "\n")
	}

	syncAgentViewport(&m)
	parts := []string{renderHeader(m, "Agents")}
	if m.saveSuccess {
		parts = append(parts, SuccessStyle.Render("✓ Saved successfully"))
	}
	parts = append(parts, m.agentViewport.View())
	if m.quitConfirm {
		parts = append(parts, clipLines(ErrorStyle.Render("⚠ You have unsaved changes. Quit anyway? (y/n)"), m.width))
	}
	parts = append(parts, agentListHelp(m.width))
	parts = append(parts, renderStatusBar(m, agentListScreenLabel, len(m.primaryAgents)+len(m.subagents)))
	return strings.Join(parts, "\n")
}

func agentListHelp(width int) string {
	return renderResponsiveHelp(width,
		"S Review & Save · Q Quit · Esc Quit/confirm",
		"S Save · Q Quit · Esc Exit",
	)
}

func agentListViewportHeight(m Model) int {
	if m.height <= 0 {
		return 0
	}
	fixed := lipgloss.Height(renderHeader(m, "Agents")) + lipgloss.Height(agentListHelp(m.width)) +
		lipgloss.Height(renderStatusBar(m, agentListScreenLabel, len(m.primaryAgents)+len(m.subagents)))
	components := 4 // header, viewport, help, status
	if m.saveSuccess {
		fixed++
		components++
	}
	if m.quitConfirm {
		fixed++
		components++
	}
	return max(1, m.height-fixed-(components-1))
}

func syncAgentViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.agentViewport.Width = max(1, m.width)
	m.agentViewport.Height = agentListViewportHeight(*m)
	content, selectedStart, selectedEnd := renderAgentListContent(*m)
	m.agentViewport.SetContent(content)
	ensureViewportRange(&m.agentViewport, selectedStart, selectedEnd)
}

func renderAgentListContent(m Model) (string, int, int) {
	var blocks []string
	selectedStart, selectedEnd := 0, 0
	line := 0
	appendBlock := func(block string, selected bool) {
		block = clipLines(block, m.width)
		height := lipgloss.Height(block)
		if selected {
			selectedStart = line
			selectedEnd = line + height - 1
		}
		blocks = append(blocks, block)
		line += height
	}

	disabled := make(map[string]bool, len(m.disabledAgents))
	for _, d := range m.disabledAgents {
		disabled[d] = true
	}
	selectableIdx := 0
	globalModelVal := "(none)"
	if val, ok := m.config.GetGlobalModel(); ok && val != "" {
		globalModelVal = val
	}
	isGlobalSelected := selectableIdx == m.agentCursor
	appendBlock(renderGlobalRow(globalModelVal, isGlobalSelected), isGlobalSelected)
	selectableIdx++
	appendBlock(SectionHeader.Render("◆ Primary Agents"), false)
	for _, name := range sortedCopy(m.primaryAgents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.agentCursor {
				isSelected = true
			}
			selectableIdx++
		}
		appendBlock(renderAgentRow(m, name, isDisabled, isSelected), isSelected)
	}
	appendBlock(SectionHeader.Render("◆ Subagents"), false)
	for _, name := range sortedCopy(m.subagents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			if selectableIdx == m.agentCursor {
				isSelected = true
			}
			selectableIdx++
		}
		appendBlock(renderAgentRow(m, name, isDisabled, isSelected), isSelected)
	}
	return strings.Join(blocks, "\n"), selectedStart, selectedEnd
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

// agentListOptionalFields is the ordered set of configured values shown on the
// compact second line. Model stays on line one and disable is represented by a
// badge, so neither appears here.
var agentListOptionalFields = []string{
	"temperature",
	"top_p",
	"color",
	"steps",
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

// renderAgentRow renders a compact two-line agent summary. The first line has
// identity, badges, and model. The second includes configured optional values
// only. Agent Detail remains the complete six-field editor.
//
// Layout:
//
//	[cursor] name [H] | [DISABLED] · model <value or (none)>
//	  temp .4 · top_p .9 · color #FF5733 · steps 10
//
// Spec: REQ-TUI-002 — agent list rendering summarizes configured values.
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
	nameLine += " · model " + FieldValue.Render(compactFieldValue(m, name, "model"))

	values := make([]string, 0, len(agentListOptionalFields))
	for _, field := range agentListOptionalFields {
		val, ok := configuredFieldValue(m, name, field)
		if !ok {
			continue
		}
		label := field
		if field == "temperature" {
			label = "temp"
		}
		values = append(values, label+" "+val)
	}
	content := nameLine + "\n    " + strings.Join(values, " · ")
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

func configuredFieldValue(m Model, name, field string) (string, bool) {
	val, ok := m.config.GetAgentField(name, field)
	if !ok || val == nil {
		return "", false
	}
	switch value := val.(type) {
	case float64:
		s := strconv.FormatFloat(value, 'f', -1, 64)
		if strings.HasPrefix(s, "0.") {
			s = strings.TrimPrefix(s, "0")
		} else if strings.HasPrefix(s, "-0.") {
			s = "-" + strings.TrimPrefix(s, "-0")
		}
		return s, true
	default:
		s := fmt.Sprintf("%v", value)
		return s, s != ""
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
		syncAgentViewport(&m)
		return m, nil

	// --- Cursor up ---
	case (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k') ||
		msg.Type == tea.KeyUp:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		syncAgentViewport(&m)
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
