// Package tui — variant selection screen implementation.
//
// This file implements the Variant Selection screen: rendering and key handling.
// When a user selects a model that has named variants (from provider configuration),
// this screen allows them to choose an independent variant for the target agent.
//
// Spec coverage:
//   - Requirement: independent compatible selection
//   - Scenario: model then variant
//   - Scenario: model-only fallback
//   - Scenario: cancel does not mutate
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// initVariantSelectionScreen initializes the variant selection state for the
// selected model and focuses the initial cursor position.
func initVariantSelectionScreen(m *Model, model opencode.Model) {
	m.pendingSelectedModel = model
	m.availableVariants = append([]opencode.VariantDescriptor(nil), model.Variants...)
	opencode.SortVariants(m.availableVariants)
	m.variantCursor = 0

	// If the agent already has an effective variant matching one of the available
	// variants for this model, position the cursor on it.
	if m.config != nil && m.selectedAgent != "" {
		currentVar, _, _ := m.config.ResolveEffectiveVariant(m.selectedAgent)
		for i, v := range m.availableVariants {
			if v.Name == currentVar {
				m.variantCursor = i
				break
			}
		}
	}
	syncVariantViewport(m)
}

// viewVariantSelection renders the variant selection screen.
func viewVariantSelection(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}
	header := renderHeader(m, "Select Variant")
	sub := renderVariantSubheader(m)
	help := variantSelectionHelp(m.width)
	status := renderStatusBar(m, "Select Variant", len(m.availableVariants))

	if m.width <= 0 || m.height <= 0 {
		content, _, _ := renderVariantSelectionContent(m)
		return strings.Join([]string{header, sub, content, help, status}, "\n")
	}
	syncVariantViewport(&m)
	return strings.Join([]string{header, sub, m.variantViewport.View(), help, status}, "\n")
}

func renderVariantSubheader(m Model) string {
	modelName := m.pendingSelectedModel.FullName
	if modelName == "" {
		modelName = "(none)"
	}
	return clipLines(FieldLabel.Render("Model: ")+FieldValue.Render(modelName), m.width)
}

func variantSelectionHelp(width int) string {
	return renderResponsiveHelp(width,
		"Enter Apply variant · Esc Cancel",
		"Enter Apply · Esc Cancel",
	)
}

func variantSelectionViewportHeight(m Model) int {
	if m.height <= 0 {
		return 0
	}
	sub := renderVariantSubheader(m)
	fixed := lipgloss.Height(renderHeader(m, "Select Variant")) + lipgloss.Height(sub) +
		lipgloss.Height(variantSelectionHelp(m.width)) + lipgloss.Height(renderStatusBar(m, "Select Variant", len(m.availableVariants)))
	return max(1, m.height-fixed-4)
}

func syncVariantViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.variantViewport.SetWidth(max(1, m.width))
	m.variantViewport.SetHeight(variantSelectionViewportHeight(*m))
	content, selectedStart, selectedEnd := renderVariantSelectionContent(*m)
	m.variantViewport.SetContent(content)
	ensureViewportRange(&m.variantViewport, selectedStart, selectedEnd)
}

func renderVariantSelectionContent(m Model) (string, int, int) {
	if len(m.availableVariants) == 0 {
		return HelpStyle.Render("No variants available"), 0, 0
	}

	currentVar := currentVariantName(m)
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

	for index, variant := range m.availableVariants {
		isSelected := index == m.variantCursor
		isCurrent := variant.Name == currentVar
		appendBlock(renderVariantRow(variant, isSelected, isCurrent, m.width), isSelected)
	}

	return strings.Join(blocks, "\n"), selectedStart, selectedEnd
}

func currentVariantName(m Model) string {
	if m.config == nil || m.selectedAgent == "" {
		return ""
	}
	currentMod, _, _ := m.config.ResolveEffectiveModel(m.selectedAgent)
	if currentMod != m.pendingSelectedModel.FullName {
		return ""
	}
	val, _, _ := m.config.ResolveEffectiveVariant(m.selectedAgent)
	return val
}

func formatVariantOptionsSummary(options map[string]interface{}) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	if effort, ok := options["reasoningEffort"].(string); ok && effort != "" {
		parts = append(parts, "effort: "+effort)
	}
	if thinking, ok := options["thinking"].(map[string]interface{}); ok {
		if budget, ok := thinking["budgetTokens"]; ok {
			parts = append(parts, fmt.Sprintf("budget: %v", budget))
		}
	} else if budget, ok := options["budgetTokens"]; ok {
		parts = append(parts, fmt.Sprintf("budget: %v", budget))
	}
	if verbosity, ok := options["textVerbosity"].(string); ok && verbosity != "" {
		parts = append(parts, "verbosity: "+verbosity)
	}
	if disabled, ok := options["disabled"].(bool); ok && disabled {
		parts = append(parts, "disabled")
	}

	if len(parts) == 0 {
		keys := make([]string, 0, len(options))
		for k := range options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %v", k, options[k]))
		}
	}

	return strings.Join(parts, ", ")
}

func renderVariantRow(variant opencode.VariantDescriptor, isSelected, isCurrent bool, width int) string {
	cursor := "  "
	if isSelected {
		cursor = SelectedPrefix.Render("▶ ") + " "
	}

	variantName := variant.Name
	optionsSummary := formatVariantOptionsSummary(variant.Options)
	desc := ""
	if optionsSummary != "" {
		desc = "  " + HelpStyle.Render("("+optionsSummary+")")
	}

	badge := ""
	if isCurrent {
		badge = " " + CurrentBadge.Render("★ current")
	}

	if width > 0 {
		available := max(1, width-lipgloss.Width(cursor)-lipgloss.Width(badge)-lipgloss.Width(desc))
		variantName = clipLines(variantName, available)
	}
	line := cursor + variantName + desc + badge

	if isSelected {
		return SelectedStyle.Render(line)
	}
	return AgentNormal.Render(line)
}

// updateVariantSelection handles key presses on the variant selection screen.
func updateVariantSelection(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case keyMsg.Code == tea.KeyEsc:
		m.popScreen()
		m.pendingSelectedModel = opencode.Model{}
		m.availableVariants = nil
		m.variantCursor = 0
		return m, nil

	case keyMsg.Code == tea.KeyEnter:
		return selectVariantAtCursor(m), nil

	case keyMsg.Code == tea.KeyDown || keyMsg.Text == "j" || (keyMsg.Code == 'n' && keyMsg.Mod.Contains(tea.ModCtrl)):
		if len(m.availableVariants) > 0 && m.variantCursor < len(m.availableVariants)-1 {
			m.variantCursor++
		}
		syncVariantViewport(&m)
		return m, nil

	case keyMsg.Code == tea.KeyUp || keyMsg.Text == "k" || (keyMsg.Code == 'p' && keyMsg.Mod.Contains(tea.ModCtrl)):
		if m.variantCursor > 0 {
			m.variantCursor--
		}
		syncVariantViewport(&m)
		return m, nil

	default:
		return m, nil
	}
}

// selectVariantAtCursor applies the selected model and variant to the agent.
func selectVariantAtCursor(m Model) Model {
	if m.variantCursor < 0 || m.variantCursor >= len(m.availableVariants) {
		return m
	}

	selectedVariant := m.availableVariants[m.variantCursor]
	selectedModel := m.pendingSelectedModel

	switch m.fieldEditing {
	case fieldEditingBulkAll:
		for _, name := range selectableCatalogNames(m) {
			oldModel, _, _ := m.config.ResolveEffectiveModel(name)
			oldVariant, _, _ := m.config.ResolveEffectiveVariant(name)
			if oldModel == selectedModel.FullName && oldVariant == selectedVariant.Name {
				continue
			}
			if err := m.config.SetAgentModelOverride(name, selectedModel.FullName); err != nil {
				continue
			}
			if err := m.config.SetAgentVariantOverride(name, selectedVariant.Name); err != nil {
				continue
			}
			m.RecordChange(name, oldModel, selectedModel.FullName, oldVariant, selectedVariant.Name)
		}
		m.bulkTargets = nil
		m.fieldEditing = ""

	case fieldEditingBulkList:
		seen := make(map[string]struct{}, len(m.bulkTargets))
		for _, name := range m.bulkTargets {
			if _, duplicate := seen[name]; duplicate {
				continue
			}
			seen[name] = struct{}{}
			if m.config.IsAgentDisabled(name) {
				continue
			}
			oldModel, _, _ := m.config.ResolveEffectiveModel(name)
			oldVariant, _, _ := m.config.ResolveEffectiveVariant(name)
			if oldModel == selectedModel.FullName && oldVariant == selectedVariant.Name {
				continue
			}
			if err := m.config.SetAgentModelOverride(name, selectedModel.FullName); err != nil {
				continue
			}
			if err := m.config.SetAgentVariantOverride(name, selectedVariant.Name); err != nil {
				continue
			}
			m.RecordChange(name, oldModel, selectedModel.FullName, oldVariant, selectedVariant.Name)
		}
		m.bulkTargets = nil
		m.fieldEditing = ""

	default:
		target := m.selectedAgent
		if target == "" {
			target = m.fieldEditing
		}
		if target != "" && target != "global" {
			oldModel, _, _ := m.config.ResolveEffectiveModel(target)
			oldVariant, _, _ := m.config.ResolveEffectiveVariant(target)
			if oldModel == selectedModel.FullName && oldVariant == selectedVariant.Name {
				// No change
			} else {
				if err := m.config.SetAgentModelOverride(target, selectedModel.FullName); err != nil {
					return m
				}
				if err := m.config.SetAgentVariantOverride(target, selectedVariant.Name); err != nil {
					return m
				}
				m.RecordChange(target, oldModel, selectedModel.FullName, oldVariant, selectedVariant.Name)
			}
		}
		m.selectedAgent = ""
		m.fieldEditing = ""
	}

	m.state = ScreenAgentList
	m.navigationStack = nil
	m.pendingSelectedModel = opencode.Model{}
	m.availableVariants = nil
	m.variantCursor = 0
	return m
}
