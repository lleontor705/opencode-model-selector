package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

func TestViewAgentList_RespectsTerminalHeightAndKeepsSelectionVisible(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = len(selectableItems(m)) - 1
	m = resizeModel(t, m, 80, 24)

	out := m.View().Content
	t.Logf("agent list rendered %d lines at 80x24", renderedLineCount(out))
	assert.LessOrEqual(t, renderedLineCount(out), 24)
	assert.Contains(t, out, selectableItems(m)[m.agentCursor])
}

func TestViewModelSelection_RespectsTerminalHeightAndKeepsSelectionVisible(t *testing.T) {
	m := NewModel(fixtureConfig(t), sixtyModels(), 5)
	m.state = ScreenModelSelection
	m.fieldEditing = "global"
	initModelSelectionScreen(&m)
	m.modelCursor = len(m.filteredModels) - 1
	m = resizeModel(t, m, 80, 24)

	out := m.View().Content
	t.Logf("60-model picker rendered %d lines at 80x24", renderedLineCount(out))
	assert.LessOrEqual(t, renderedLineCount(out), 24)
	assert.Contains(t, out, m.filteredModels[m.modelCursor].ID)
}

func TestWindowSizeMsg_ResizesListAndViewportComponents(t *testing.T) {
	m := NewModel(fixtureConfig(t), sixtyModels(), 5)
	m = resizeModel(t, m, 92, 31)

	assert.Equal(t, 92, m.agentViewport.Width())
	assert.Positive(t, m.agentViewport.Height())
	assert.Equal(t, 92, m.modelViewport.Width())
	assert.Positive(t, m.modelViewport.Height())
}

func TestViewAgentList_CompactRowShowsModelOnly(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentModelOverride("plan", "openai/gpt-5"))
	m := NewModel(cfg, sampleGrouped(), 5)
	m.agentCursor = indexOf(selectableItems(m), "plan")
	m = resizeModel(t, m, 80, 24)

	out := m.View().Content
	assert.Contains(t, out, "plan")
	assert.Contains(t, out, "openai/gpt-5")
}

func TestView_FooterRemainsVisibleAtMinimumSupportedHeight(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m = resizeModel(t, m, minTerminalWidth, minTerminalHeight)

	out := m.View().Content
	assert.LessOrEqual(t, renderedLineCount(out), minTerminalHeight)
	assert.Contains(t, out, "Enter Model · A · M · S Save · Q Quit")
	assert.Contains(t, out, agentListScreenLabel)
}

func TestAgentList_LongModelValueTruncatesToTerminalWidth(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentModelOverride("plan", "provider/"+strings.Repeat("very-long-model-", 12)))
	m := NewModel(cfg, sampleGrouped(), 5)
	m.agentCursor = indexOf(selectableItems(m), "plan")
	m = resizeModel(t, m, 60, 18)

	assertRenderedWidthAtMost(t, m.View().Content, 60)
}

func TestModelSelection_LongModelValueTruncatesToTerminalWidth(t *testing.T) {
	longID := "model-" + strings.Repeat("extremely-long-", 10)
	grouped := map[string][]opencode.Model{
		"provider": {{Provider: "provider", ID: longID, FullName: "provider/" + longID}},
	}
	m := NewModel(fixtureConfig(t), grouped, 5)
	m.state = ScreenModelSelection
	m.fieldEditing = "global"
	initModelSelectionScreen(&m)
	m = resizeModel(t, m, 60, 18)

	assertRenderedWidthAtMost(t, m.View().Content, 60)
}

func TestView_BelowMinimumTerminalSizeShowsCompactWarning(t *testing.T) {
	m := resizeModel(t, NewModel(fixtureConfig(t), sampleGrouped(), 5), minTerminalWidth-1, minTerminalHeight-1)
	out := m.View().Content

	assert.Contains(t, out, "Terminal too small")
	assert.LessOrEqual(t, renderedLineCount(out), minTerminalHeight-1)
	assertRenderedWidthAtMost(t, out, minTerminalWidth-1)
}

func TestView_HelpFootersFitSupportedWidthsAndKeepCriticalActions(t *testing.T) {
	tests := []struct {
		name        string
		model       func(t *testing.T) Model
		compactHelp string
		fullHelp    string
	}{
		{
			name:        "agent list",
			model:       func(t *testing.T) Model { return NewModel(fixtureConfig(t), sampleGrouped(), 5) },
			compactHelp: "Enter Model · A · M · S Save · Q Quit",
			fullHelp:    "Enter Model · A Apply-all · M Multi · S Review & Save · Q Quit",
		},
		{
			name: "model picker",
			model: func(t *testing.T) Model {
				m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
				m.state = ScreenModelSelection
				m.fieldEditing = "global"
				initModelSelectionScreen(&m)
				return m
			},
			compactHelp: "Enter Apply · Esc Cancel",
			fullHelp:    "Enter Apply model · Esc Cancel",
		},
		{
			name: "save review",
			model: func(t *testing.T) Model {
				return newSaveConfirmModel(t, true)
			},
			compactHelp: "Y Save · N/Esc Back",
			fullHelp:    "Enter/Y Save to disk · Esc/N Back",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, width := range []int{40, 60, 80} {
				t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
					m := resizeModel(t, tt.model(t), width, 30)
					expected := tt.compactHelp
					if width >= 70 {
						expected = tt.fullHelp
					}

					footer := renderedLineContaining(t, m.View().Content, expected)
					assert.LessOrEqual(t, lipgloss.Width(footer), m.width)
				})
			}
		})
	}
}

func TestResponsiveHelp_SelectsCompactWordingBelowFullHelpThreshold(t *testing.T) {
	full := "Enter Apply model · Esc Cancel"
	compact := "Enter Apply · Esc Cancel"

	for _, width := range []int{40, fullHelpMinWidth - 1} {
		assert.Equal(t, compact, ansi.Strip(renderResponsiveHelp(width, full, compact)),
			"width %d must render the compact wording", width)
	}
	for _, width := range []int{fullHelpMinWidth, 80} {
		assert.Equal(t, full, ansi.Strip(renderResponsiveHelp(width, full, compact)),
			"width %d must render the full wording", width)
	}
}

func TestResponsiveChrome_HeaderAndStatusBarFitNarrowWidths(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	for _, width := range []int{40, 60, 80} {
		m.width = width
		assertRenderedWidthAtMost(t, renderHeader(m, "Agents"), width)
		assertRenderedWidthAtMost(t, renderStatusBar(m, "Agents", 7), width)
	}
}

func resizeModel(t *testing.T, m Model, width, height int) Model {
	t.Helper()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	result, ok := updated.(Model)
	require.True(t, ok)
	return result
}

func sixtyModels() map[string][]opencode.Model {
	grouped := make(map[string][]opencode.Model)
	for i := 0; i < 60; i++ {
		provider := fmt.Sprintf("provider-%02d", i/10)
		id := fmt.Sprintf("model-%02d", i)
		grouped[provider] = append(grouped[provider], opencode.Model{
			Provider: provider,
			ID:       id,
			FullName: provider + "/" + id,
		})
	}
	return grouped
}

func renderedLineCount(s string) int {
	return lipgloss.Height(strings.TrimSuffix(s, "\n"))
}

func assertRenderedWidthAtMost(t *testing.T, output string, width int) {
	t.Helper()
	for i, line := range strings.Split(output, "\n") {
		assert.LessOrEqualf(t, lipgloss.Width(line), width, "rendered line %d exceeds terminal width", i+1)
	}
}

func renderedLineContaining(t *testing.T, output, expected string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, expected) {
			return line
		}
	}
	require.FailNowf(t, "footer not found", "expected rendered footer containing %q", expected)
	return ""
}
