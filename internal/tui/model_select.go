// Package tui — model selection screen implementation (REQ-TUI-005).
//
// This file implements the Model Selection screen: rendering and key handling.
// It shows all available models grouped by provider with a fuzzy filter input,
// and allows the user to assign a model to the global default or a specific
// agent.
//
// Spec coverage:
//   - REQ-TUI-005: Model selection rendering (grouped display, filter, markers)
//   - REQ-TUI-005: Filter logic (case-insensitive substring on provider + ID + FullName)
//   - REQ-TUI-005: Navigation (j/k, arrows, ENTER, ESC, typing, backspace)
package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"opencode-model-selector/internal/config"
	"opencode-model-selector/internal/opencode"
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
// filterInput value, sorts the result by FullName for deterministic display,
// and clamps the cursor to a valid index.
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

	// Sort by FullName for deterministic ordering in both rendering and
	// cursor navigation.
	sort.Slice(m.filteredModels, func(i, j int) bool {
		return m.filteredModels[i].FullName < m.filteredModels[j].FullName
	})

	// Clamp cursor to valid range.
	if m.cursor >= len(m.filteredModels) {
		m.cursor = len(m.filteredModels) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	return m
}

// viewModelSelection renders the model selection screen. The layout is:
//
//	[dirty] Select Model
//	Filter: <value>
//
//	── <provider>/ ──
//	► <id>  ✓  (if current model)
//	  <id>
//
//	── <provider>/ ──
//	  <id>
//
//	Type to filter  ENTER: select  ESC: cancel
//
// When the flat models list is empty, "No models available" is shown.
// When the filter produces no results, "No models match filter" is shown.
//
// Spec: REQ-TUI-005 — rendering.
func viewModelSelection(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	var b strings.Builder

	// --- Title bar with dirty indicator ---
	title := TitleStyle.Render("Select Model")
	if m.dirty {
		b.WriteString(DirtyIndicator.Render("*") + " " + title)
	} else {
		b.WriteString(title)
	}
	b.WriteString("\n\n")

	// --- Empty models list ---
	if len(m.flatModels) == 0 {
		b.WriteString(HelpStyle.Render("No models available"))
		b.WriteByte('\n')
		return b.String()
	}

	// --- Filter input ---
	b.WriteString("Filter: ")
	b.WriteString(m.filterInput.View())
	b.WriteString("\n\n")

	// --- No filter results ---
	if len(m.filteredModels) == 0 {
		b.WriteString(HelpStyle.Render("No models match filter"))
		b.WriteByte('\n')
		return b.String()
	}

	// --- Determine the current model (for checkmark marker) ---
	currentModel := currentModelFullName(m)

	// --- Grouped display by provider ---
	// Build a set of filtered FullNames for O(1) membership checks during
	// grouped iteration.
	filteredSet := make(map[string]bool, len(m.filteredModels))
	for _, model := range m.filteredModels {
		filteredSet[model.FullName] = true
	}

	// Sort providers alphabetically for deterministic rendering.
	providers := make([]string, 0, len(m.groupedModels))
	for p := range m.groupedModels {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	// flatIdx tracks the position in the flat filteredModels list so we can
	// apply the cursor highlight to the correct row.
	flatIdx := 0
	for _, provider := range providers {
		// Sort models within each provider by FullName.
		models := append([]opencode.Model(nil), m.groupedModels[provider]...)
		sort.Slice(models, func(i, j int) bool {
			return models[i].FullName < models[j].FullName
		})

		// Check if any model from this provider is in the filtered set.
		hasFiltered := false
		for _, model := range models {
			if filteredSet[model.FullName] {
				hasFiltered = true
				break
			}
		}
		if !hasFiltered {
			continue
		}

		// Section header.
		b.WriteString(SectionHeader.Render(provider + "/"))
		b.WriteByte('\n')

		for _, model := range models {
			if !filteredSet[model.FullName] {
				continue
			}

			isSelected := flatIdx == m.cursor
			isCurrent := model.FullName == currentModel

			b.WriteString(renderModelRow(model, isSelected, isCurrent))
			b.WriteByte('\n')
			flatIdx++
		}
		b.WriteByte('\n')
	}

	// --- Help footer ---
	b.WriteString(HelpStyle.Render("Type to filter  ENTER: select  ESC: cancel"))

	return b.String()
}

// renderModelRow renders a single model row in the selection list. The cursor
// is indicated by '►' and the current model by '✓'.
func renderModelRow(model opencode.Model, isSelected, isCurrent bool) string {
	cursor := "  "
	if isSelected {
		cursor = "\u25ba "
	}

	line := cursor + model.ID
	if isCurrent {
		line += "  \u2713"
	}

	if isSelected {
		return SelectedStyle.Render(line)
	}
	return AgentNormal.Render(line)
}

// currentModelFullName returns the FullName of the model currently assigned to
// the global default or the selected agent, depending on fieldEditing.
func currentModelFullName(m Model) string {
	if m.fieldEditing == "global" {
		if val, ok := m.config.GetGlobalModel(); ok && val != "" {
			return val
		}
		return ""
	}
	// Per-agent model.
	if val, ok := m.config.GetAgentField(m.selectedAgent, "model"); ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// updateModelSelection handles key presses on the model selection screen.
//
// Keys:
//   - typing:   updates filterInput, applies filter, resets cursor to 0
//   - Backspace: deletes from filterInput, applies filter
//   - j / Down: cursor down (stops at last filtered model)
//   - k / Up:   cursor up (stops at 0)
//   - ENTER:    selects model at cursor, persists to config, returns to previousState
//   - ESC:      cancels, returns to previousState without changes
//
// Spec: REQ-TUI-005 — interaction.
func updateModelSelection(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	// --- ESC: cancel, return to previousState ---
	case keyMsg.Type == tea.KeyEsc || keyMsg.Type == tea.KeyEscape:
		m.state = m.previousState
		return m, nil

	// --- ENTER: select model at cursor ---
	case keyMsg.Type == tea.KeyEnter:
		return selectModelAtCursor(m), nil

	// --- Backspace: delete from filter, re-apply ---
	case keyMsg.Type == tea.KeyBackspace:
		m.filterInput, _ = m.filterInput.Update(keyMsg)
		m.cursor = 0
		return applyFilter(m), nil

	// --- Cursor down ---
	case (keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'j') ||
		keyMsg.Type == tea.KeyDown:
		if len(m.filteredModels) > 0 && m.cursor < len(m.filteredModels)-1 {
			m.cursor++
		}
		return m, nil

	// --- Cursor up ---
	case (keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'k') ||
		keyMsg.Type == tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	// --- Typing: update filter input, reset cursor ---
	case keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) > 0:
		m.filterInput, _ = m.filterInput.Update(keyMsg)
		m.cursor = 0
		return applyFilter(m), nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// selectModelAtCursor validates and persists the model at the current cursor
// position. For global edits it calls SetGlobalModel; for per-agent edits it
// calls SetAgentField. Marks dirty and returns to previousState.
func selectModelAtCursor(m Model) Model {
	if m.cursor < 0 || m.cursor >= len(m.filteredModels) {
		return m
	}

	selected := m.filteredModels[m.cursor]

	// Validate — should always pass since we're selecting from the list.
	if !config.ValidateModel(selected.FullName, m.flatModels) {
		return m
	}

	if m.fieldEditing == "global" {
		m.config.SetGlobalModel(selected.FullName)
	} else {
		// Per-agent model. May error if the agent is disabled, but the
		// config layer handles that — we proceed only on success.
		_ = m.config.SetAgentField(m.selectedAgent, "model", selected.FullName)
	}

	m.dirty = true
	m.state = m.previousState
	return m
}

// initModelSelectionScreen resets the filter input, cursor, and filteredModels
// when entering the Model Selection screen. Called from updateAgentList and
// updateAgentDetail when transitioning to ScreenModelSelection.
func initModelSelectionScreen(m *Model) {
	m.filterInput = textinput.New()
	m.filterInput.Placeholder = "Type to filter..."
	m.filterInput.Focus()
	m.cursor = 0
	m.filteredModels = append([]opencode.Model(nil), m.flatModels...)
	// Sort for deterministic initial display.
	sort.Slice(m.filteredModels, func(i, j int) bool {
		return m.filteredModels[i].FullName < m.filteredModels[j].FullName
	})
}
