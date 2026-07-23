package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

func TestViewAgentList_RespectsTerminalHeightAndKeepsSelectionVisible(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = len(selectableItems(m)) - 1
	m = resizeModel(t, m, 80, 24)

	out := m.View()
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

	out := m.View()
	t.Logf("60-model picker rendered %d lines at 80x24", renderedLineCount(out))
	assert.LessOrEqual(t, renderedLineCount(out), 24)
	assert.Contains(t, out, m.filteredModels[m.modelCursor].ID)
}

func TestWindowSizeMsg_ResizesListAndViewportComponents(t *testing.T) {
	m := NewModel(fixtureConfig(t), sixtyModels(), 5)
	m = resizeModel(t, m, 92, 31)

	assert.Equal(t, 92, m.agentViewport.Width)
	assert.Positive(t, m.agentViewport.Height)
	assert.Equal(t, 92, m.modelViewport.Width)
	assert.Positive(t, m.modelViewport.Height)
}

func TestViewAgentList_CompactRowShowsConfiguredValuesAndOmitsUnsetFields(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentField("plan", "model", "openai/gpt-5"))
	require.NoError(t, cfg.SetAgentField("plan", "top_p", 0.9))
	require.NoError(t, cfg.SetAgentField("plan", "color", "#FF5733"))
	require.NoError(t, cfg.SetAgentField("plan", "steps", 10.0))
	m := NewModel(cfg, sampleGrouped(), 5)
	m.agentCursor = indexOf(selectableItems(m), "plan")
	m = resizeModel(t, m, 80, 24)

	out := m.View()
	assert.Contains(t, out, "plan")
	assert.Contains(t, out, "openai/gpt-5")
	assert.Contains(t, out, "temp .4 · top_p .9 · color #FF5733 · steps 10")
	assert.NotContains(t, out, "temperature:")
	assert.NotContains(t, out, "disable:")
}

func TestView_FooterRemainsVisibleAtMinimumSupportedHeight(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m = resizeModel(t, m, minTerminalWidth, minTerminalHeight)

	out := m.View()
	assert.LessOrEqual(t, renderedLineCount(out), minTerminalHeight)
	assert.Contains(t, out, "Review & Save")
	assert.Contains(t, out, agentListScreenLabel)
}

func TestAgentList_LongModelValueTruncatesToTerminalWidth(t *testing.T) {
	cfg := fixtureConfig(t)
	require.NoError(t, cfg.SetAgentField("plan", "model", "provider/"+strings.Repeat("very-long-model-", 12)))
	m := NewModel(cfg, sampleGrouped(), 5)
	m.agentCursor = indexOf(selectableItems(m), "plan")
	m = resizeModel(t, m, 60, 18)

	assertRenderedWidthAtMost(t, m.View(), 60)
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

	assertRenderedWidthAtMost(t, m.View(), 60)
}

func TestView_BelowMinimumTerminalSizeShowsCompactWarning(t *testing.T) {
	m := resizeModel(t, NewModel(fixtureConfig(t), sampleGrouped(), 5), minTerminalWidth-1, minTerminalHeight-1)
	out := m.View()

	assert.Contains(t, out, "Terminal too small")
	assert.LessOrEqual(t, renderedLineCount(out), minTerminalHeight-1)
	assertRenderedWidthAtMost(t, out, minTerminalWidth-1)
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
