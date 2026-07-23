// Package tui tests — agent list screen (REQ-TUI-002, REQ-TUI-003).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in agent_list.go, and drive its design. Coverage focuses on:
//   - Rendering: section ordering, model display, disabled/hidden indicators,
//     system-agent exclusion, dirty indicator.
//   - Navigation: cursor movement, skipping disabled agents, ENTER transitions.
//   - Key handling: j/k, arrows, ENTER, s, q.
//
// Spec: REQ-TUI-002 (agent list rendering), REQ-TUI-003 (navigation/keys).
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Rendering — viewAgentList (REQ-TUI-002)
// ---------------------------------------------------------------------------

// TestViewAgentList_GlobalDefaultModelFirst verifies that the "Global Default
// Model" entry appears in the rendered output, and that it appears BEFORE the
// "Primary Agents" section.
//
// Spec: REQ-TUI-002 — Happy path — global model entry at top.
func TestViewAgentList_GlobalDefaultModelFirst(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)

	assert.Contains(t, out, "Global Default Model",
		"the Global Default Model entry MUST appear in the rendered output")

	globalIdx := strings.Index(out, "Global Default Model")
	primaryIdx := strings.Index(out, "Primary Agents")
	require.GreaterOrEqual(t, globalIdx, 0)
	require.GreaterOrEqual(t, primaryIdx, 0)
	assert.Less(t, globalIdx, primaryIdx,
		"Global Default Model MUST appear before the Primary Agents section")
}

// TestViewAgentList_PrimaryBeforeSubagents verifies the section ordering:
// "Primary Agents" header appears before "Subagents" header.
//
// Spec: REQ-TUI-002 — Happy path — primary then subagent sections.
func TestViewAgentList_PrimaryBeforeSubagents(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)

	primaryIdx := strings.Index(out, "Primary Agents")
	subIdx := strings.Index(out, "Subagents")
	require.GreaterOrEqual(t, primaryIdx, 0, "Primary Agents section MUST exist")
	require.GreaterOrEqual(t, subIdx, 0, "Subagents section MUST exist")
	assert.Less(t, primaryIdx, subIdx,
		"Primary Agents section MUST appear before Subagents section")
}

// TestViewAgentList_ModelValueShownForCodeReviewer verifies that when an agent
// has a model set, the model value is displayed in its row.
//
// Spec: REQ-TUI-002 — Happy path — model value shown for configured agent.
func TestViewAgentList_ModelValueShownForCodeReviewer(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.Contains(t, out, "anthropic/claude-sonnet-4-20250514",
		"code-reviewer's model value MUST appear in the rendered output")
}

// TestViewAgentList_NoneShownWhenNoModel verifies that agents without a model
// display "(none)" as the model value.
//
// Spec: REQ-TUI-002 — Happy path — model: (none) when unset.
func TestViewAgentList_NoneShownWhenNoModel(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	// Multiple agents in the fixture have no model (build, plan, debug, etc.)
	assert.Contains(t, out, "(none)",
		"(none) MUST be shown for agents without a model set")
}

// TestViewAgentList_GlobalModelValueShownWhenSet verifies that when a global
// model is configured, its value appears in the Global Default Model row.
//
// Spec: REQ-TUI-002 — Happy path — global model value displayed.
func TestViewAgentList_GlobalModelValueShownWhenSet(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.SetGlobalModel("opencode-go/glm-5.2")
	m := NewModel(cfg, sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.Contains(t, out, "opencode-go/glm-5.2",
		"global model value MUST be shown when set")
}

// TestViewAgentList_GlobalModelNoneWhenUnset verifies that "(none)" appears
// for the global default when no global model is configured.
func TestViewAgentList_GlobalModelNoneWhenUnset(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	// The fixture has no top-level "model" key.
	globalIdx := strings.Index(out, "Global Default Model")
	require.GreaterOrEqual(t, globalIdx, 0)
	afterGlobal := out[globalIdx:]
	assert.Contains(t, afterGlobal, "(none)",
		"global model MUST show (none) when unset")
}

// TestViewAgentList_DisabledAgentShownWithIndicator verifies that disabled
// agents (build) appear visually with a [DISABLED] indicator.
//
// Spec: REQ-TUI-002 — Edge case — disabled agents greyed, shown but not editable.
func TestViewAgentList_DisabledAgentShownWithIndicator(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.Contains(t, out, "build",
		"disabled agent 'build' MUST still appear visually in the list")
	assert.Contains(t, out, "[DISABLED]",
		"disabled agents MUST carry a [DISABLED] indicator")
}

// TestViewAgentList_HiddenAgentShownWithIndicator verifies that hidden agents
// (parallel-dispatch) appear with an [H] indicator.
//
// Spec: REQ-TUI-002 — Edge case — hidden agents shown with [H] and selectable.
func TestViewAgentList_HiddenAgentShownWithIndicator(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.Contains(t, out, "parallel-dispatch",
		"hidden agent 'parallel-dispatch' MUST appear in the list")
	assert.Contains(t, out, "[H]",
		"hidden agents MUST carry an [H] indicator")
}

// TestViewAgentList_SystemAgentsExcluded verifies that system agents
// (compactación, title, summary) do NOT appear anywhere in the rendered output.
//
// Spec: REQ-TUI-002 + REQ-CFG-008 — system agents filtered from display.
func TestViewAgentList_SystemAgentsExcluded(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	for _, sys := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, out, sys,
			"system agent %q MUST NOT appear in the rendered output", sys)
	}
}

// TestViewAgentList_DirtyIndicatorShown verifies that the dirty indicator
// marker appears when dirty=true.
//
// Spec: REQ-TUI-002 — Happy path — dirty indicator * on unsaved changes.
func TestViewAgentList_DirtyIndicatorShown(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	out := viewAgentList(m)
	assert.Contains(t, out, "*",
		"dirty indicator '*' MUST be shown when there are unsaved changes")
}

// TestViewAgentList_DirtyIndicatorNotShownWhenClean verifies that the dirty
// indicator does NOT appear when dirty=false.
func TestViewAgentList_DirtyIndicatorNotShownWhenClean(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = false
	out := viewAgentList(m)
	// The title line is the first line; extract it and verify no '*'.
	lines := strings.SplitN(out, "\n", 2)
	require.GreaterOrEqual(t, len(lines), 1)
	assert.NotContains(t, lines[0], "*",
		"dirty indicator '*' MUST NOT appear in the title when clean")
}

// TestViewAgentList_HelpFooterPresent verifies the keybinding help line is shown.
//
// Spec: REQ-TUI-002 — Happy path — help text at bottom.
func TestViewAgentList_HelpFooterPresent(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.Contains(t, out, "ENTER")
	assert.Contains(t, out, "save")
	assert.Contains(t, out, "quit")
}

// TestViewAgentList_ReturnsNonEmpty verifies a basic non-empty contract.
func TestViewAgentList_ReturnsNonEmpty(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	out := viewAgentList(m)
	assert.NotEmpty(t, out, "viewAgentList MUST return a non-empty string")
}

// ---------------------------------------------------------------------------
// Selectable items — selectableItems (cursor logic)
// ---------------------------------------------------------------------------

// TestSelectableItems_GlobalIsFirst verifies that "__global__" is always the
// first entry in the selectable items list.
func TestSelectableItems_GlobalIsFirst(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	require.NotEmpty(t, items)
	assert.Equal(t, "__global__", items[0],
		"__global__ MUST be the first selectable item")
}

// TestSelectableItems_DisabledExcluded verifies that disabled agents are NOT
// in the selectable list.
//
// Spec: REQ-TUI-002 — disabled agents are non-selectable.
func TestSelectableItems_DisabledExcluded(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	assert.NotContains(t, items, "build",
		"disabled agent 'build' MUST NOT be selectable")
}

// TestSelectableItems_HiddenIncluded verifies that hidden agents ARE in the
// selectable list.
//
// Spec: REQ-TUI-002 — hidden agents ARE selectable.
func TestSelectableItems_HiddenIncluded(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	assert.Contains(t, items, "parallel-dispatch",
		"hidden agent 'parallel-dispatch' MUST be selectable")
}

// TestSelectableItems_SystemExcluded verifies that system agents never appear
// in the selectable list.
func TestSelectableItems_SystemExcluded(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	for _, sys := range []string{"compactación", "title", "summary"} {
		assert.NotContains(t, items, sys,
			"system agent %q MUST NOT be selectable", sys)
	}
}

// TestSelectableItems_SortedAlphabetically verifies that primary and subagent
// groups are each sorted alphabetically within their section.
func TestSelectableItems_SortedAlphabetically(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	items := selectableItems(m)
	// items[0] is __global__, items[1:] are agents.
	// The non-disabled primary is "plan"; non-disabled subagents are
	// code-reviewer, debug, docs, explore, general, orchestrator,
	// parallel-dispatch, security-auditor, team-lead.
	// Verify that plan (the only non-disabled primary) comes before all
	// subagents, and subagents are sorted.
	planIdx := indexOf(items, "plan")
	require.GreaterOrEqual(t, planIdx, 0)

	// Every subagent must come after plan.
	for _, sub := range []string{"code-reviewer", "debug", "docs", "explore",
		"general", "orchestrator", "parallel-dispatch", "security-auditor",
		"team-lead"} {
		idx := indexOf(items, sub)
		require.GreaterOrEqual(t, idx, 0, "subagent %q must be selectable", sub)
		assert.Greater(t, idx, planIdx,
			"subagent %q must come after primary agents in selectable list", sub)
	}

	// Verify subagents are sorted among themselves.
	var subItems []string
	for _, item := range items[planIdx+1:] {
		if item != "__global__" {
			subItems = append(subItems, item)
		}
	}
	assert.True(t, isSorted(subItems),
		"subagents within the selectable list MUST be sorted alphabetically: %v", subItems)
}

// ---------------------------------------------------------------------------
// Navigation — updateAgentList (REQ-TUI-003)
// ---------------------------------------------------------------------------

// TestUpdateAgentList_J_MovesCursorDown verifies that pressing 'j' increments
// the cursor.
//
// Spec: REQ-TUI-003 — Happy path — j navigates down.
func TestUpdateAgentList_J_MovesCursorDown(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	assert.Equal(t, 0, m.cursor, "precondition: cursor starts at 0 (global)")

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, newM.cursor, "cursor MUST be 1 after pressing 'j'")
}

// TestUpdateAgentList_J_SkipsDisabledAgents verifies that moving down from the
// global entry lands on the first non-disabled primary agent ("plan"), NOT on
// the disabled agent "build".
//
// Spec: REQ-TUI-003 — Edge case — j skips disabled agents.
func TestUpdateAgentList_J_SkipsDisabledAgents(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	items := selectableItems(newM)
	require.True(t, newM.cursor < len(items), "cursor must be in bounds")
	assert.Equal(t, "plan", items[newM.cursor],
		"after 'j' from global, cursor MUST be on 'plan' (first non-disabled primary), NOT 'build' (disabled)")
}

// TestUpdateAgentList_K_MovesCursorUp verifies that pressing 'k' decrements the
// cursor.
//
// Spec: REQ-TUI-003 — Happy path — k navigates up.
func TestUpdateAgentList_K_MovesCursorUp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.cursor = 2

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, newM.cursor, "cursor MUST be 1 after pressing 'k' from 2")
}

// TestUpdateAgentList_K_AtTopStaysAtZero verifies that pressing 'k' at cursor 0
// keeps the cursor at 0 (no negative wrap).
func TestUpdateAgentList_K_AtTopStaysAtZero(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.cursor = 0

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, newM.cursor,
		"cursor MUST stay at 0 when pressing 'k' at the top")
}

// TestUpdateAgentList_DownArrow_MovesDown verifies that the Down arrow key
// works the same as 'j'.
//
// Spec: REQ-TUI-003 — Down arrow navigates down.
func TestUpdateAgentList_DownArrow_MovesDown(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, newM.cursor, "Down arrow MUST move cursor down")
}

// TestUpdateAgentList_UpArrow_MovesUp verifies that the Up arrow key works the
// same as 'k'.
//
// Spec: REQ-TUI-003 — Up arrow navigates up.
func TestUpdateAgentList_UpArrow_MovesUp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.cursor = 2
	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, newM.cursor, "Up arrow MUST move cursor up")
}

// TestUpdateAgentList_EnterOnGlobal_TransitionsToModelSelection verifies that
// pressing ENTER on the global default model entry transitions to the Model
// Selection screen.
//
// Spec: REQ-TUI-003 — Happy path — ENTER on global opens model selection.
func TestUpdateAgentList_EnterOnGlobal_TransitionsToModelSelection(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.cursor = 0 // __global__

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state,
		"ENTER on global MUST transition to ScreenModelSelection")
}

// TestUpdateAgentList_EnterOnAgent_TransitionsToAgentDetail verifies that
// pressing ENTER on an agent transitions to the Agent Detail screen and sets
// selectedAgent.
//
// Spec: REQ-TUI-003 — Happy path — ENTER on agent opens detail.
func TestUpdateAgentList_EnterOnAgent_TransitionsToAgentDetail(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	// cursor 1 = "plan" (first non-disabled primary agent)
	m.cursor = 1

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, newM.state,
		"ENTER on an agent MUST transition to ScreenAgentDetail")
	assert.Equal(t, "plan", newM.selectedAgent,
		"selectedAgent MUST be set to the agent at the cursor position")
}

// TestUpdateAgentList_S_WhenDirty_TransitionsToSaveConfirm verifies that
// pressing 's' when dirty transitions to the Save Confirm screen.
//
// Spec: REQ-TUI-007 — Happy path — save when dirty.
func TestUpdateAgentList_S_WhenDirty_TransitionsToSaveConfirm(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, ScreenSaveConfirm, newM.state,
		"'s' when dirty MUST transition to ScreenSaveConfirm")
}

// TestUpdateAgentList_S_WhenNotDirty_StaysOnScreen verifies that pressing 's'
// when not dirty does NOT transition.
//
// Spec: REQ-TUI-007 — Edge case — save with no changes.
func TestUpdateAgentList_S_WhenNotDirty_StaysOnScreen(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	require.False(t, m.dirty)

	newM, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, ScreenAgentList, newM.state,
		"'s' when NOT dirty MUST stay on ScreenAgentList")
}

// TestUpdateAgentList_Q_Quits verifies that pressing 'q' produces a tea.Quit
// command.
//
// Spec: REQ-TUI-003 — Happy path — q quits.
func TestUpdateAgentList_Q_Quits(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	_, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd, "'q' MUST produce a non-nil command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'q' MUST produce a tea.QuitMsg")
}

// TestUpdateAgentList_OtherKey_NoOp verifies that unmapped keys do not change
// state or cursor.
func TestUpdateAgentList_OtherKey_NoOp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, ScreenAgentList, newM.state, "unmapped key MUST not change state")
	assert.Equal(t, 0, newM.cursor, "unmapped key MUST not move cursor")
	assert.Nil(t, cmd, "unmapped key MUST produce a nil command")
}

// ---------------------------------------------------------------------------
// Quit confirmation — dirty state guard (REQ-TUI-003)
// ---------------------------------------------------------------------------
//
// When dirty == true, pressing 'q' or Ctrl+C does NOT quit immediately.
// Instead, it sets quitConfirm to show an overlay: "You have unsaved changes.
// Quit anyway? (y/n)". Only y/Y/ENTER confirm; n/N/ESC cancel; all other keys
// are ignored.
//
// When dirty == false, 'q' and Ctrl+C quit immediately (existing behavior).

// TestUpdateAgentList_Q_NotDirty_QuitsImmediately verifies that pressing 'q'
// when there are no unsaved changes quits immediately without showing a
// confirmation prompt, and quitConfirm stays false.
func TestUpdateAgentList_Q_NotDirty_QuitsImmediately(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	require.False(t, m.dirty, "precondition: dirty must be false")

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd, "'q' with dirty=false MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'q' with dirty=false MUST produce a tea.QuitMsg")
	assert.False(t, newM.quitConfirm, "quitConfirm MUST stay false when not dirty")
}

// TestUpdateAgentList_Q_Dirty_ShowsConfirmation verifies that pressing 'q'
// when there are unsaved changes does NOT quit immediately but instead sets
// the quitConfirm flag to show the confirmation prompt.
//
// Spec: REQ-TUI-003 — dirty guard prevents accidental data loss.
func TestUpdateAgentList_Q_Dirty_ShowsConfirmation(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.True(t, newM.quitConfirm, "'q' with dirty=true MUST set quitConfirm")
	assert.Nil(t, cmd, "'q' with dirty=true MUST NOT produce a quit command")
}

// TestUpdateAgentList_CtrlC_Dirty_ShowsConfirmation verifies that pressing
// Ctrl+C when dirty behaves the same as 'q' — it triggers the confirmation
// instead of quitting immediately.
//
// Spec: REQ-TUI-003 — Ctrl+C respects the dirty guard on AgentList.
func TestUpdateAgentList_CtrlC_Dirty_ShowsConfirmation(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.True(t, newM.quitConfirm, "Ctrl+C with dirty=true MUST set quitConfirm")
	assert.Nil(t, cmd, "Ctrl+C with dirty=true MUST NOT quit immediately")
}

// --- Quit confirmation: confirm with y / Y / ENTER ---

// TestUpdateAgentList_QuitConfirm_Y_ConfirmsQuit verifies that pressing 'y'
// while the quit confirmation is showing confirms the quit.
func TestUpdateAgentList_QuitConfirm_Y_ConfirmsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	_, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd, "'y' in quitConfirm MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'y' in quitConfirm MUST produce tea.QuitMsg")
}

// TestUpdateAgentList_QuitConfirm_UpperY_ConfirmsQuit verifies that pressing
// uppercase 'Y' also confirms the quit.
func TestUpdateAgentList_QuitConfirm_UpperY_ConfirmsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	_, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	require.NotNil(t, cmd, "'Y' in quitConfirm MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'Y' in quitConfirm MUST produce tea.QuitMsg")
}

// TestUpdateAgentList_QuitConfirm_Enter_ConfirmsQuit verifies that pressing
// ENTER while the quit confirmation is showing confirms the quit.
func TestUpdateAgentList_QuitConfirm_Enter_ConfirmsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	_, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "ENTER in quitConfirm MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "ENTER in quitConfirm MUST produce tea.QuitMsg")
}

// --- Quit confirmation: cancel with n / N / ESC ---

// TestUpdateAgentList_QuitConfirm_N_CancelsQuit verifies that pressing 'n'
// while the quit confirmation is showing cancels the quit and clears
// quitConfirm, keeping the user on the Agent List screen.
func TestUpdateAgentList_QuitConfirm_N_CancelsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.False(t, newM.quitConfirm, "'n' MUST clear quitConfirm")
	assert.Nil(t, cmd, "'n' MUST NOT produce a quit command")
	assert.Equal(t, ScreenAgentList, newM.state, "'n' MUST stay on ScreenAgentList")
}

// TestUpdateAgentList_QuitConfirm_UpperN_CancelsQuit verifies that pressing
// uppercase 'N' also cancels the quit.
func TestUpdateAgentList_QuitConfirm_UpperN_CancelsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.False(t, newM.quitConfirm, "'N' MUST clear quitConfirm")
	assert.Nil(t, cmd, "'N' MUST NOT produce a quit command")
}

// TestUpdateAgentList_QuitConfirm_Esc_CancelsQuit verifies that pressing ESC
// while the quit confirmation is showing cancels the quit.
func TestUpdateAgentList_QuitConfirm_Esc_CancelsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, newM.quitConfirm, "ESC MUST clear quitConfirm")
	assert.Nil(t, cmd, "ESC MUST NOT produce a quit command")
	assert.Equal(t, ScreenAgentList, newM.state, "ESC in quitConfirm MUST stay on AgentList")
}

// --- Quit confirmation: other keys ignored ---

// TestUpdateAgentList_QuitConfirm_J_Ignored verifies that pressing 'j' while
// the quit confirmation is showing does not move the cursor.
func TestUpdateAgentList_QuitConfirm_J_Ignored(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true
	m.cursor = 0

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.True(t, newM.quitConfirm, "'j' MUST NOT clear quitConfirm")
	assert.Equal(t, 0, newM.cursor, "'j' MUST NOT move cursor during quitConfirm")
	assert.Nil(t, cmd, "'j' MUST NOT produce a command during quitConfirm")
}

// TestUpdateAgentList_QuitConfirm_K_Ignored verifies that pressing 'k' while
// the quit confirmation is showing does not move the cursor.
func TestUpdateAgentList_QuitConfirm_K_Ignored(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true
	m.cursor = 2

	newM, cmd := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.True(t, newM.quitConfirm, "'k' MUST NOT clear quitConfirm")
	assert.Equal(t, 2, newM.cursor, "'k' MUST NOT move cursor during quitConfirm")
	assert.Nil(t, cmd, "'k' MUST NOT produce a command during quitConfirm")
}

// --- Quit confirmation: rendering (View) ---

// TestViewAgentList_QuitConfirm_ShowsPrompt verifies that the confirmation
// prompt text appears in the rendered output when quitConfirm is true.
func TestViewAgentList_QuitConfirm_ShowsPrompt(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	out := viewAgentList(m)
	assert.Contains(t, out, "unsaved changes",
		"view MUST contain 'unsaved changes' when quitConfirm is active")
	assert.Contains(t, out, "Quit anyway",
		"view MUST contain 'Quit anyway' when quitConfirm is active")
}

// TestViewAgentList_QuitConfirm_ShowsYNOptions verifies that the confirmation
// prompt shows the (y/n) option hint.
func TestViewAgentList_QuitConfirm_ShowsYNOptions(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	out := viewAgentList(m)
	assert.Contains(t, out, "(y/n)",
		"view MUST contain '(y/n)' when quitConfirm is active")
}

// --- Quit confirmation: state cleanup on transition ---

// TestUpdateAgentList_CancelThenEnter_TransitionsCleanly verifies that after
// canceling the quit confirmation, normal navigation resumes and quitConfirm
// is properly reset to false when transitioning away from AgentList.
func TestUpdateAgentList_CancelThenEnter_TransitionsCleanly(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.cursor = 1 // plan

	// Step 1: 'q' with dirty → quitConfirm=true
	m1, _ := updateAgentList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.True(t, m1.quitConfirm, "step 1: quitConfirm must be set")

	// Step 2: 'n' → cancel, quitConfirm=false
	m2, _ := updateAgentList(m1, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.False(t, m2.quitConfirm, "step 2: quitConfirm must be cleared")

	// Step 3: ENTER → transitions to AgentDetail, quitConfirm still false
	m3, _ := updateAgentList(m2, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentDetail, m3.state,
		"ENTER after canceling quit MUST transition normally")
	assert.False(t, m3.quitConfirm,
		"quitConfirm MUST be false after transitioning to AgentDetail")
}

// ---------------------------------------------------------------------------
// Helpers for test assertions
// ---------------------------------------------------------------------------

// indexOf returns the first index of target in s, or -1 if not found.
func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

// isSorted returns true if the slice is in ascending lexicographic order.
func isSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}
