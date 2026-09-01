// Package tui — model selection screen implementation (REQ-TUI-005).
//
// This file implements the Model Selection screen: rendering and key handling.
// It shows all available models grouped by provider with a fuzzy filter input,
// and allows the user to assign a model to the global default or a specific
// agent.
//
// Visual layout:
//
//	╭─ ocs ─────────────────────────────────────────╮
//	│  Interactive model selector for OpenCode...   │
//	╰───────────────────────────────────────────────╯
//
//	🔍 Search: <filter input>
//
//	[ opencode-go/ ]
//	  ▶ glm-5.1   [★ CURRENT]
//	    glm-5.2
//
//	[ openai/ ]
//	    gpt-5
//
//	[ <Select Model> · N models ]                            ? for help
//
// Spec coverage:
//   - REQ-TUI-005: Model selection rendering (grouped display, filter, markers)
//   - REQ-TUI-005: Filter logic (case-insensitive substring on provider + ID + FullName)
//   - REQ-TUI-005: Navigation (j/k, arrows, ENTER, ESC, typing, backspace)
package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// matchesFilter checks if a model matches the filter text. The match is a
// case-insensitive substring check against the model's Provider, ID, and
// FullName fields. An empty filter matches every model.
//
// Spec: REQ-TUI-005 — case-insensitive substring on provider + ID.
func matchesFilter(model opencode.Model, filter string) bool {
	if filter == "" {
		return true
	}
	lower := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(model.Provider), lower) ||
		strings.Contains(strings.ToLower(model.ID), lower) ||
		strings.Contains(strings.ToLower(model.FullName), lower)
}

// applyFilter rebuilds filteredModels from flatModels using the current
// filterInput value, sorts the selectable rows in display order, and clamps
// the cursor to a valid index.
//
// Spec: REQ-TUI-005 — filter rebuild on text change.
func applyFilter(m Model) Model {
	filter := m.filterInput.Value()
	m.filteredModels = make([]opencode.Model, 0, len(m.flatModels))
	for _, model := range m.flatModels {
		if matchesFilter(model, filter) {
			m.filteredModels = append(m.filteredModels, model)
		}
	}

	sortSelectableModels(m.filteredModels)

	// Clamp cursor to valid range.
	if m.modelCursor >= len(m.filteredModels) {
		m.modelCursor = len(m.filteredModels) - 1
	}
	if m.modelCursor < 0 {
		m.modelCursor = 0
	}

	return m
}

func sortSelectableModels(models []opencode.Model) {
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		if models[i].ID != models[j].ID {
			return models[i].ID < models[j].ID
		}
		return models[i].FullName < models[j].FullName
	})
}

// viewModelSelection renders the model selection screen.
func viewModelSelection(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}
	search := clipLines(SearchLabel.Render("🔍 Search: ")+m.filterInput.View(), m.width)
	help := modelSelectionHelp(m.width)
	status := renderStatusBar(m, "Select Model", len(m.filteredModels))
	if m.width <= 0 || m.height <= 0 {
		content, _, _ := renderModelSelectionContent(m)
		return strings.Join([]string{renderHeader(m, "Select Model"), search, content, help, status}, "\n")
	}
	syncModelViewport(&m)
	return strings.Join([]string{renderHeader(m, "Select Model"), search, m.modelViewport.View(), help, status}, "\n")
}

func modelSelectionHelp(width int) string {
	return renderResponsiveHelp(width,
		"Enter Apply model · Esc Cancel",
		"Enter Apply · Esc Cancel",
	)
}

func modelSelectionViewportHeight(m Model) int {
	if m.height <= 0 {
		return 0
	}
	search := SearchLabel.Render("🔍 Search: ") + m.filterInput.View()
	fixed := lipgloss.Height(renderHeader(m, "Select Model")) + lipgloss.Height(search) +
		lipgloss.Height(modelSelectionHelp(m.width)) + lipgloss.Height(renderStatusBar(m, "Select Model", len(m.filteredModels)))
	return max(1, m.height-fixed-4) // five vertical components have four separators
}

func syncModelViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.modelViewport.SetWidth(max(1, m.width))
	m.modelViewport.SetHeight(modelSelectionViewportHeight(*m))
	content, selectedStart, selectedEnd := renderModelSelectionContent(*m)
	m.modelViewport.SetContent(content)
	ensureViewportRange(&m.modelViewport, selectedStart, selectedEnd)
}

func renderModelSelectionContent(m Model) (string, int, int) {
	if len(m.flatModels) == 0 {
		return HelpStyle.Render("No models available"), 0, 0
	}
	if len(m.filteredModels) == 0 {
		return HelpStyle.Render("No models match filter"), 0, 0
	}

	currentModel := currentModelFullName(m)
	var blocks []string
	line := 0
	selectedStart, selectedEnd := 0, 0
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
	previousProvider := ""
	for index, model := range m.filteredModels {
		if model.Provider != previousProvider {
			appendBlock(renderProviderBadge(model.Provider), false)
			previousProvider = model.Provider
		}

		isSelected := index == m.modelCursor
		isCurrent := model.FullName == currentModel
		appendBlock(renderModelRow(model, isSelected, isCurrent, m.width), isSelected)
	}
	return strings.Join(blocks, "\n"), selectedStart, selectedEnd
}

// renderProviderBadge renders a provider name as a colored pill in the model
// picker. The diamond prefix keeps the visual language consistent with
// section headers elsewhere in the TUI.
func renderProviderBadge(provider string) string {
	return ProviderBadge.Render("◆ " + provider + "/")
}

// renderModelRow renders a single model row in the selection list. The cursor
// is indicated by '▶' and the current model by a '★ CURRENT' pill.
func renderModelRow(model opencode.Model, isSelected, isCurrent bool, width int) string {
	cursor := "  "
	if isSelected {
		cursor = SelectedPrefix.Render("▶ ") + " "
	}

	// Keep provider identity visible even when its group header has scrolled
	// outside the viewport.
	modelName := model.FullName
	badge := ""
	if isCurrent {
		badge = " " + CurrentBadge.Render("★ current")
		if width > 0 {
			available := max(1, width-lipgloss.Width(cursor)-lipgloss.Width(badge))
			modelName = clipLines(modelName, available)
		}
	}
	line := cursor + modelName + badge

	if isSelected {
		return SelectedStyle.Render(line)
	}
	return AgentNormal.Render(line)
}

// currentModelFullName returns the FullName of the model currently assigned to
// the global default or the selected agent, depending on fieldEditing.
//
// For per-agent edits, reads the MERGED model value so the "★ current" marker
// shows on the row matching the agent's actual current model, even when that
// model comes from a markdown frontmatter value. This is DISPLAY only; the
// commit path still writes a JSON model override.
func currentModelFullName(m Model) string {
	if m.fieldEditing == fieldEditingBulkAll || m.fieldEditing == fieldEditingBulkList {
		return ""
	}
	if m.fieldEditing == "global" {
		if val, ok := m.config.GetGlobalModel(); ok && val != "" {
			return val
		}
		return ""
	}
	// Per-agent model: merged reader so md-backed agents show their md model.
	if val, ok := m.config.GetMergedAgentField(m.selectedAgent, "model"); ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// updateModelSelection handles key presses and widget messages on the model selection screen.
//
// Keys:
//   - typing:   updates filterInput, applies filter, resets cursor to 0, propagates blink cmd
//   - Backspace: deletes from filterInput, applies filter, propagates blink cmd
//   - Down / Ctrl+N: cursor down (stops at last filtered model)
//   - Up / Ctrl+P:   cursor up (stops at 0)
//   - ENTER:         selects model at cursor, persists to config, pops origin
//   - ESC:           cancels and pops the immutable origin without changes
//   - non-key msgs:  passed to filterInput.Update to process cursor blink ticks
//
// Spec: REQ-TUI-005 — interaction, REQ-TUI-PRO-004 — blink command propagation.
func updateModelSelection(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	switch {
	// --- ESC: cancel, return to immutable origin ---
	case keyMsg.Code == tea.KeyEsc:
		m.popScreen()
		return m, nil

	// --- ENTER: select model at cursor ---
	case keyMsg.Code == tea.KeyEnter:
		return selectModelAtCursor(m), nil

	// --- Backspace: delete from filter, re-apply ---
	case keyMsg.Code == tea.KeyBackspace:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(keyMsg)
		m.modelCursor = 0
		m = applyFilter(m)
		syncModelViewport(&m)
		return m, cmd

	// --- Cursor down ---
	case keyMsg.Code == tea.KeyDown || (keyMsg.Code == 'n' && keyMsg.Mod.Contains(tea.ModCtrl)):
		if len(m.filteredModels) > 0 && m.modelCursor < len(m.filteredModels)-1 {
			m.modelCursor++
		}
		syncModelViewport(&m)
		return m, nil

	// --- Cursor up ---
	case keyMsg.Code == tea.KeyUp || (keyMsg.Code == 'p' && keyMsg.Mod.Contains(tea.ModCtrl)):
		if m.modelCursor > 0 {
			m.modelCursor--
		}
		syncModelViewport(&m)
		return m, nil

	// --- Typing: update filter input, reset cursor ---
	case keyMsg.Text != "":
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(keyMsg)
		m.modelCursor = 0
		m = applyFilter(m)
		syncModelViewport(&m)
		return m, cmd

	// --- Unmapped key: pass to filterInput in case widget handles it ---
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(keyMsg)
		return m, cmd
	}
}

// selectModelAtCursor validates and persists the model at the current cursor
// position. For global edits it calls SetGlobalModel; for per-agent edits it
// writes a model override. Records the change so the save-confirm screen can
// render a diff, marks dirty, and returns to the immutable origin.
func selectModelAtCursor(m Model) Model {
	if m.modelCursor < 0 || m.modelCursor >= len(m.filteredModels) {
		return m
	}

	selected := m.filteredModels[m.modelCursor]

	if !config.ValidateModel(selected.FullName, m.flatModels) {
		return m
	}

	switch m.fieldEditing {
	case "global":
		if len(selected.Variants) > 0 {
			m.pendingSelectedModel = selected
			m.pushScreen(ScreenVariantSelection)
			initVariantSelectionScreen(&m, selected)
			return m
		}
		oldVal, _ := m.config.GetGlobalModel()
		m.config.SetGlobalModel(selected.FullName)
		m.RecordModelChange("global", oldVal, selected.FullName)

	case fieldEditingBulkAll:
		if len(selected.Variants) > 0 {
			m.pendingSelectedModel = selected
			m.pushScreen(ScreenVariantSelection)
			initVariantSelectionScreen(&m, selected)
			return m
		}
		for _, name := range selectableCatalogNames(m) {
			oldVal, _, _ := m.config.ResolveEffectiveModel(name)
			if oldVal == selected.FullName {
				continue
			}
			if err := m.config.SetAgentModelOverride(name, selected.FullName); err != nil {
				continue
			}
			m.RecordModelChange(name, oldVal, selected.FullName)
		}
		m.bulkTargets = nil

	case fieldEditingBulkList:
		if len(selected.Variants) > 0 {
			m.pendingSelectedModel = selected
			m.pushScreen(ScreenVariantSelection)
			initVariantSelectionScreen(&m, selected)
			return m
		}
		seen := make(map[string]struct{}, len(m.bulkTargets))
		for _, name := range m.bulkTargets {
			if _, duplicate := seen[name]; duplicate {
				continue
			}
			seen[name] = struct{}{}
			if m.config.IsAgentDisabled(name) {
				continue
			}
			oldVal, _, _ := m.config.ResolveEffectiveModel(name)
			if oldVal == selected.FullName {
				continue
			}
			if err := m.config.SetAgentModelOverride(name, selected.FullName); err != nil {
				continue
			}
			m.RecordModelChange(name, oldVal, selected.FullName)
		}
		m.bulkTargets = nil

	default:
		if len(selected.Variants) > 0 {
			m.pendingSelectedModel = selected
			m.pushScreen(ScreenVariantSelection)
			initVariantSelectionScreen(&m, selected)
			return m
		}

		// Capture the previous MERGED model so the save-confirm diff reads
		// "<md-model> -> <new>" instead of "(none) -> <new>" when overriding
		// a markdown-backed agent. The write remains a JSON model override.
		oldVal, _, _ := m.config.ResolveEffectiveModel(m.selectedAgent)
		if oldVal == selected.FullName {
			m.popScreen()
			return m
		}
		if err := m.config.SetAgentModelOverride(m.selectedAgent, selected.FullName); err != nil {
			return m
		}
		m.RecordModelChange(m.selectedAgent, oldVal, selected.FullName)
	}

	m.popScreen()
	return m
}

// initModelSelectionScreen resets the filter input, cursor, and filteredModels
// when entering the Model Selection screen. It focuses the filter input and
// returns the cursor blink command.
func initModelSelectionScreen(m *Model) tea.Cmd {
	m.filterInput = textinput.New()
	m.filterInput.Placeholder = "Type to filter..."
	cmd := m.filterInput.Focus()
	// Bubbles v2 truncates the placeholder to Width()+1 runes, so a zero
	// width would hide it entirely. Seed the width with the placeholder
	// length; the WindowSizeMsg handler re-clamps it to the terminal.
	m.filterInput.SetWidth(lipgloss.Width(m.filterInput.Placeholder))
	m.modelCursor = 0
	m.filteredModels = append([]opencode.Model(nil), m.flatModels...)
	sortSelectableModels(m.filteredModels)
	syncModelViewport(m)
	return cmd
}
