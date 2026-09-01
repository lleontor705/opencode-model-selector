// Package tui — agent list screen implementation (REQ-TUI-002, REQ-TUI-003).
//
// This file implements the main Agent List screen: rendering and key handling.
// It is the entry screen of the TUI and shows all user-facing agents grouped
// into Primary Agents and Subagents, plus the Global Default Model entry.
//
// Visual layout (top → bottom):
//
//	╭─ ocs ─────────────────────────────────────────╮
//	│  Interactive model selector for OpenCode...   │
//	╰───────────────────────────────────────────────╯
//
//	[Global Default Model]
//	  model: <value or (none)>
//
//	── Primary Agents ──
//	  ▶ agent-name [DISABLED] · model <value or (none)>
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//	    ...same compact configured-value summary...
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
//
// Spec coverage:
//   - REQ-TUI-002: Agent list rendering (sections, model display, indicators)
//   - REQ-TUI-003: Navigation (j/k, arrows, ENTER, s, q)
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lleontor705/opencode-model-selector/internal/config"
)

// mergedAgentsForRender is the render-side chokepoint for every
// markdown-dependent config read on the agent list. Config.MergedAgents
// re-discovers and re-parses the global/project markdown agent layers on
// every call (filesystem I/O), so the render MUST call it exactly once and
// share the result across all rows. It is a package-level indirection (the
// same pattern as config.mdFileParse) so tests can count discovery passes
// per render — the per-row resolution regression guard.
var mergedAgentsForRender = (*config.Config).MergedAgents

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
	items := make([]string, 0, len(m.primaryAgents)+len(m.subagents)+len(m.allAgents)+1)
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
	for _, name := range sortedCopy(m.allAgents) {
		if !disabled[name] {
			items = append(items, name)
		}
	}

	return items
}

// viewAgentList renders the agent list screen. The layout is:
//
//	[dirty] ocs
//
//	[Global Default Model]
//	  model: <value or (none)>
//
//	── Primary Agents ──
//	  ▶ agent-name [DISABLED] · model <value or (none)>
//
//	── Subagents ──
//	  ▶ agent-name  [H]
//
//	[ <Agents> · 11 agents · ● unsaved ]                  ? for help
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
	if m.width <= 0 || m.height <= 0 {
		content, _, _ := renderAgentListContent(m)
		parts := []string{renderHeader(m, "Agents")}
		if warning := catalogWarning(m); warning != "" {
			parts = append(parts, renderCatalogWarningOverlay(m.width, warning))
		}
		if m.saveSuccess {
			parts = append(parts, renderSaveSuccessOverlay(m.width))
		}
		parts = append(parts, content)
		if m.quitConfirm {
			parts = append(parts, renderQuitConfirmOverlay(m.width))
		}
		parts = append(parts, agentListHelp(m.width), renderStatusBar(m, agentListScreenLabel, agentCount(m)))
		return strings.Join(parts, "\n")
	}

	syncAgentViewport(&m)
	parts := []string{renderHeader(m, "Agents")}
	if warning := catalogWarning(m); warning != "" {
		parts = append(parts, renderCatalogWarningOverlay(m.width, warning))
	}
	if m.saveSuccess {
		parts = append(parts, renderSaveSuccessOverlay(m.width))
	}
	parts = append(parts, m.agentViewport.View())
	if m.quitConfirm {
		parts = append(parts, renderQuitConfirmOverlay(m.width))
	}
	parts = append(parts, agentListHelp(m.width))
	parts = append(parts, renderStatusBar(m, agentListScreenLabel, agentCount(m)))
	return strings.Join(parts, "\n")
}

func agentListHelp(width int) string {
	return renderResponsiveHelp(width,
		"Enter Model · A Apply-all · M Multi · S Review & Save · Q Quit",
		"Enter Model · A · M · S Save · Q Quit",
	)
}

func agentListViewportHeight(m Model) int {
	if m.height <= 0 {
		return 0
	}
	fixed := lipgloss.Height(renderHeader(m, "Agents")) + lipgloss.Height(agentListHelp(m.width)) +
		lipgloss.Height(renderStatusBar(m, agentListScreenLabel, agentCount(m)))
	components := 4 // header, viewport, help, status
	if warning := catalogWarning(m); warning != "" {
		fixed += lipgloss.Height(renderCatalogWarningOverlay(m.width, warning))
		components++
	}
	if m.saveSuccess {
		fixed += lipgloss.Height(renderSaveSuccessOverlay(m.width))
		components++
	}
	if m.quitConfirm {
		fixed += lipgloss.Height(renderQuitConfirmOverlay(m.width))
		components++
	}
	return max(1, m.height-fixed-(components-1))
}

func renderOverlay(width int, content string) string {
	boxWidth := headerBoxWidth
	if width > 0 {
		boxWidth = min(boxWidth, max(1, width-OverlayBoxStyle.GetHorizontalFrameSize()))
		return clipLines(OverlayBoxStyle.Width(boxWidth).Render(content), width)
	}
	return OverlayBoxStyle.Render(content)
}

func renderCatalogWarningOverlay(width int, warning string) string {
	return renderOverlay(width, WarningStyle.Render("⚠ "+warning))
}

func renderSaveSuccessOverlay(width int) string {
	return renderOverlay(width, SuccessStyle.Render("✓ Saved successfully"))
}

func renderQuitConfirmOverlay(width int) string {
	return renderOverlay(width, WarningStyle.Render("⚠ You have unsaved changes. Quit anyway? (y/n)"))
}

func syncAgentViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.agentViewport.SetWidth(max(1, m.width))
	m.agentViewport.SetHeight(agentListViewportHeight(*m))
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

	// ONE markdown-discovery + merge pass per render, shared by every row
	// (per-row resolution re-globs and re-parses the agent .md layers once
	// per agent — see mergedAgentsForRender).
	snap := buildRenderSnapshot(m)

	disabled := make(map[string]bool, len(m.disabledAgents))
	for _, d := range m.disabledAgents {
		disabled[d] = true
	}
	selectableIdx := 0
	globalModelVal := "(none)"
	if snap.globalModel != "" {
		globalModelVal = snap.globalModel
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
		appendBlock(renderAgentRow(m, snap, name, isDisabled, isSelected), isSelected)
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
		appendBlock(renderAgentRow(m, snap, name, isDisabled, isSelected), isSelected)
	}
	appendBlock(SectionHeader.Render("◆ All Agents"), false)
	for _, name := range sortedCopy(m.allAgents) {
		isDisabled := disabled[name]
		isSelected := false
		if !isDisabled {
			isSelected = selectableIdx == m.agentCursor
			selectableIdx++
		}
		appendBlock(renderAgentRow(m, snap, name, isDisabled, isSelected), isSelected)
	}
	return strings.Join(blocks, "\n"), selectedStart, selectedEnd
}

func agentCount(m Model) int { return len(m.primaryAgents) + len(m.subagents) + len(m.allAgents) }

func catalogWarning(m Model) string {
	if !m.agentCatalog.Degraded {
		return ""
	}
	if len(m.agentCatalog.Diagnostics) > 0 && m.agentCatalog.Diagnostics[0].Message != "" {
		return m.agentCatalog.Diagnostics[0].Message
	}
	return "runtime agent discovery unavailable; using static catalog"
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

// renderSnapshot bundles the markdown-dependent config state for ONE agent
// list render. It is built exactly once per renderAgentListContent call (via
// buildRenderSnapshot) and shared by every row, so the markdown discovery +
// parse cost behind Config.MergedAgents is paid once per frame instead of
// once per row.
//
// It stays LIVE per render: the picker commits inline-JSON overrides directly
// to the in-memory config (selectModelAtCursor → SetAgentModelOverride /
// SetGlobalModel), so each new render rebuilds the snapshot and reflects
// pending selections immediately.
type renderSnapshot struct {
	// merged is the JSON > project md > global md merge of all agent layers
	// (single discovery pass). Never nil.
	merged map[string]*config.MergedAgent
	// globalModel is the non-empty top-level "model" fallback, if any.
	globalModel string
}

// buildRenderSnapshot computes the per-render shared config view. Nil config
// yields an empty snapshot (viewAgentList already guards that path).
func buildRenderSnapshot(m Model) renderSnapshot {
	snap := renderSnapshot{merged: map[string]*config.MergedAgent{}}
	if m.config == nil {
		return snap
	}
	if val, ok := m.config.GetGlobalModel(); ok && val != "" {
		snap.globalModel = val
	}
	snap.merged = mergedAgentsForRender(m.config)
	return snap
}

// effectiveModel resolves the LIVE effective model for name from the
// per-render snapshot with the exact Config.ResolveEffectiveModel precedence:
// inline JSON > project markdown > global markdown > global top-level. The
// equivalence holds because MergedAgent.Fields["model"] is already the
// JSON > project md > global md merge (empty values treated as absent, so an
// explicit JSON "" does not shadow an md value — the same rule ResolveModel
// applies), and globalModel is the same final fallback. No filesystem I/O:
// discovery happened once, in buildRenderSnapshot.
//
// Deletion/revert semantics are preserved: clearing an inline override makes
// Fields["model"] fall back to the md value on the NEXT render's snapshot,
// and removing every agent-specific layer resurfaces the global model.
func (s renderSnapshot) effectiveModel(name string) (string, bool) {
	if ma, ok := s.merged[name]; ok {
		if model, ok := ma.Fields["model"].(string); ok && model != "" {
			return model, true
		}
	}
	if s.globalModel != "" {
		return s.globalModel, true
	}
	return "", false
}

// mergedField mirrors Config.GetMergedAgentField on the shared snapshot: the
// resolved (JSON > project md > global md) field value, with absent, nil, and
// empty-string values treated as missing.
func (s renderSnapshot) mergedField(name, field string) (interface{}, bool) {
	ma, ok := s.merged[name]
	if !ok {
		return nil, false
	}
	v, present := ma.Fields[field]
	if !present || v == nil {
		return nil, false
	}
	if str, isStr := v.(string); isStr && str == "" {
		return nil, false
	}
	return v, true
}

// compactFieldValue resolves a single field for an agent and renders it as a
// short string for the agent list. Returns "(none)" if the field is missing
// or its value is nil. Empty strings also render as "" (caller decides) —
// for the agent list, we never want empty strings to look like a real value,
// so we collapse them to "(none)" too.
//
// For the "model" field the LIVE effective model wins:
// snap.effectiveModel resolves inline JSON > project md > global md > global
// top-level fallback from the once-per-render snapshot, reflecting pending
// picker selections immediately without per-row filesystem I/O (display only;
// model writes remain JSON-only). The discovery-time catalog snapshot is kept
// only as a degraded fallback for agents whose model no longer resolves from
// any config layer — consulting it first would keep showing a stale model
// after the picker writes a new selection.
func compactFieldValue(m Model, snap renderSnapshot, name, field string) string {
	if field == "model" {
		if model, ok := snap.effectiveModel(name); ok {
			return model
		}
		if record, ok := m.catalogByName[name]; ok && record.Model != "" {
			return record.Model
		}
	}
	val, ok := snap.mergedField(name, field)
	if !ok || val == nil {
		return "(none)"
	}
	s := fmt.Sprintf("%v", val)
	if s == "" {
		return "(none)"
	}
	return s
}

// renderAgentRow renders a compact agent summary with identity, badges, and
// model. All markdown-dependent reads (effective model, merged fields) come
// from the once-per-render snap — no per-row filesystem I/O.
//
// Layout:
//
//	[cursor] name [H] | [DISABLED] · model <value or (none)>
//
// Spec: REQ-TUI-002 — agent list rendering summarizes configured values.
func renderAgentRow(m Model, snap renderSnapshot, name string, isDisabled, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = SelectedPrefix.Render("▶ ") + " "
	}

	nameLine := prefix + name

	// Append the [MD] badge while the agent is still markdown-only (no JSON
	// override yet). Once any inline-JSON override exists the merge layer
	// unsets MdOnly and the badge disappears — performSave recomputes the
	// mdOnlyAgents cache so this stays live after a save.
	if m.IsMarkdownOnly(name) {
		nameLine += " " + HelpStyle.Render("[MD]")
	}
	// Append indicator for hidden agents.
	if isAgentHidden(m, name) {
		nameLine += " " + HelpStyle.Render("[H]")
	}
	// Append indicator for disabled agents.
	if isDisabled {
		nameLine += " " + ErrorStyle.Render("[DISABLED]")
	}
	nameLine += " · model " + FieldValue.Render(compactFieldValue(m, snap, name, "model"))

	content := nameLine
	switch {
	case isDisabled:
		return AgentDisabled.Render(content)
	case isSelected:
		return SelectedStyle.Render(content)
	case isAgentHidden(m, name):
		return AgentHidden.Render(content)
	default:
		return AgentNormal.Render(content)
	}
}

func isAgentHidden(m Model, name string) bool {
	if record, ok := m.catalogByName[name]; ok && record.Hidden {
		return true
	}
	return m.config != nil && m.config.IsAgentHidden(name)
}

// updateAgentList handles key presses on the agent list screen.
//
// Keys:
//   - j / Down: cursor down (skips disabled agents)
//   - k / Up:   cursor up (skips disabled agents)
//   - ENTER:    on global or agent → ScreenModelSelection
//   - s:        transition to ScreenSaveConfirm (only if dirty)
//   - q / Ctrl+C: quit — when dirty, shows confirmation overlay first
//
// When m.quitConfirm is true, a sub-state takes over: only y/Y/ENTER confirm
// the quit, n/N/ESC cancel it, and all other keys are ignored.
//
// Spec: REQ-TUI-003.
func updateAgentList(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// --- Quit confirmation sub-state ---
	// When active, intercept ALL keys. Only y/Y/ENTER confirm; n/N/ESC cancel;
	// everything else is ignored (including j/k navigation).
	if m.quitConfirm {
		switch {
		// Confirm quit: y, Y, or ENTER
		case msg.Text == "y" || msg.Text == "Y":
			return m, tea.Quit
		case msg.Code == tea.KeyEnter:
			return m, tea.Quit

		// Cancel quit: n, N, or ESC
		case msg.Text == "n" || msg.Text == "N":
			m.quitConfirm = false
			return m, nil
		case msg.Code == tea.KeyEsc:
			m.quitConfirm = false
			return m, nil

		// Any other key (including j, k, Ctrl+C, s) → ignored
		default:
			return m, nil
		}
	}

	switch {
	// --- 'a': Flow A — apply model to ALL non-system, non-disabled agents ---
	case msg.Text == "a":
		m.pushScreen(ScreenModelSelection)
		m.fieldEditing = fieldEditingBulkAll
		m.bulkTargets = nil
		m.quitConfirm = false
		initModelSelectionScreen(&m)
		return m, nil

	// --- 'm': Flow B — pick agents first, then model ---
	case msg.Text == "m":
		initAgentMultiSelectScreen(&m)
		m.multiSelectItems = selectableCatalogNames(m)
		m.multiSelectChecked = make([]bool, len(m.multiSelectItems))
		m.pushScreen(ScreenAgentMultiSelect)
		m.quitConfirm = false
		return m, nil

	// --- Quit (q or Ctrl+C) ---
	// When dirty, show the confirmation overlay instead of quitting.
	// When clean, quit immediately.
	case msg.Text == "q" || (msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl)):
		if m.dirty {
			m.quitConfirm = true
			return m, nil
		}
		return m, tea.Quit

	// --- ESC: quit from the root, guarded when there are unsaved changes ---
	case msg.Code == tea.KeyEsc:
		if m.dirty {
			m.quitConfirm = true
			return m, nil
		}
		return m, tea.Quit

	// --- Save ---
	case msg.Text == "s":
		if m.dirty {
			m.pushScreen(ScreenSaveConfirm)
		}
		return m, nil

	// --- Cursor down ---
	case msg.Text == "j" || msg.Code == tea.KeyDown:
		items := selectableItems(m)
		m.agentCursor = clampAgentCursor(m.agentCursor, len(items))
		if len(items) > 0 && m.agentCursor < len(items)-1 {
			m.agentCursor++
		}
		syncAgentViewport(&m)
		return m, nil

	// --- Cursor up ---
	case msg.Text == "k" || msg.Code == tea.KeyUp:
		m.agentCursor = clampAgentCursor(m.agentCursor, len(selectableItems(m)))
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		syncAgentViewport(&m)
		return m, nil

	// --- ENTER: transition ---
	case msg.Code == tea.KeyEnter:
		items := selectableItems(m)
		m.agentCursor = clampAgentCursor(m.agentCursor, len(items))
		if m.agentCursor >= 0 && m.agentCursor < len(items) {
			item := items[m.agentCursor]
			if item == globalItemKey {
				m.pushScreen(ScreenModelSelection)
				m.fieldEditing = "global"
				m.quitConfirm = false
				initModelSelectionScreen(&m)
			} else {
				m.selectedAgent = item
				m.pushScreen(ScreenModelSelection)
				m.fieldEditing = item
				m.quitConfirm = false
				initModelSelectionScreen(&m)
			}
		}
		return m, nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

func clampAgentCursor(cursor, itemCount int) int {
	if itemCount <= 0 || cursor < 0 {
		return 0
	}
	if cursor >= itemCount {
		return itemCount - 1
	}
	return cursor
}

// sortedCopy returns a sorted copy of the input slice. The original slice is
// not modified.
func sortedCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}
