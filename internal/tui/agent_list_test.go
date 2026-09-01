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

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/agentcatalog"
	"github.com/lleontor705/opencode-model-selector/internal/config"
)

func runtimeCatalogForTUI() agentcatalog.Catalog {
	return agentcatalog.Catalog{
		Buckets: agentcatalog.Buckets{
			Primary: []agentcatalog.AgentRecord{
				{Name: "z-primary", Role: agentcatalog.RolePrimary, Native: true, Model: "runtime/z"},
				{Name: "a-primary", Role: agentcatalog.RolePrimary, Native: true},
			},
			Subagent: []agentcatalog.AgentRecord{
				{Name: "custom-hidden", Role: agentcatalog.RoleSubagent, Hidden: true, Source: agentcatalog.SourcePlugin},
				{Name: "a-primary", Role: agentcatalog.RoleSubagent},
			},
			All: []agentcatalog.AgentRecord{
				{Name: "all-role", Role: agentcatalog.RoleAll},
				{Name: "native-hidden", Role: agentcatalog.RoleAll, Native: true, Hidden: true},
			},
		},
	}
}

func TestViewAgentList_CatalogSectionsAreExclusiveDeterministicAndComplete(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, runtimeCatalogForTUI())
	out := viewAgentList(m)

	primary := strings.Index(out, "Primary Agents")
	subagent := strings.Index(out, "Subagents")
	all := strings.Index(out, "All Agents")
	require.GreaterOrEqual(t, primary, 0)
	require.Greater(t, subagent, primary)
	require.Greater(t, all, subagent)
	assert.Less(t, strings.Index(out, "a-primary"), strings.Index(out, "z-primary"))
	assert.Equal(t, 1, strings.Count(out, "a-primary"), "a catalog identity must render in exactly one role section")
	assert.Equal(t, 1, strings.Count(out, "all-role"))
	assert.NotContains(t, out, "native-hidden", "native hidden identities are already catalog-filtered")
}

func TestViewAgentList_CustomHiddenCatalogAgentHasMarker(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, runtimeCatalogForTUI())
	out := viewAgentList(m)
	hidden := strings.Index(out, "custom-hidden")
	require.GreaterOrEqual(t, hidden, 0)
	assert.Contains(t, out[hidden:], "[H]")
}

func TestViewAgentList_DegradedCatalogWarnsWithoutBlockingNavigation(t *testing.T) {
	catalog := runtimeCatalogForTUI()
	catalog.Degraded = true
	catalog.Diagnostics = []agentcatalog.Diagnostic{{Message: "runtime unavailable; using fallback"}}
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, catalog)

	assert.Contains(t, viewAgentList(m), "runtime unavailable; using fallback")
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "all-role")
	require.GreaterOrEqual(t, m.agentCursor, 0)
	updated, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, updated.state)
	assert.Equal(t, "all-role", updated.selectedAgent)
}

func TestUpdateAgentList_CatalogNavigationClampsStaleCursor(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, agentcatalog.Catalog{})
	m.agentCursor = 99

	updated, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 0, updated.agentCursor)
	require.NotPanics(t, func() {
		_, _ = updateAgentList(updated, tea.KeyPressMsg{Code: tea.KeyEnter})
	})
}

func TestT10AgentList_AllModelFlowsReachableWithoutGenericEditor(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), richGrouped(), 5, runtimeCatalogForTUI())

	for _, tc := range []struct {
		name      string
		item      string
		key       tea.KeyPressMsg
		wantState appState
		wantEdit  string
	}{
		{name: "global", item: globalItemKey, key: tea.KeyPressMsg{Code: tea.KeyEnter}, wantState: ScreenModelSelection, wantEdit: "global"},
		{name: "runtime primary", item: "z-primary", key: tea.KeyPressMsg{Code: tea.KeyEnter}, wantState: ScreenModelSelection, wantEdit: "z-primary"},
		{name: "all role", item: "all-role", key: tea.KeyPressMsg{Code: tea.KeyEnter}, wantState: ScreenModelSelection, wantEdit: "all-role"},
		{name: "bulk all", key: tea.KeyPressMsg{Text: "a"}, wantState: ScreenModelSelection, wantEdit: fieldEditingBulkAll},
		{name: "multi select", key: tea.KeyPressMsg{Text: "m"}, wantState: ScreenAgentMultiSelect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := m
			if tc.item != "" {
				candidate.agentCursor = indexOf(selectableItems(candidate), tc.item)
				require.GreaterOrEqual(t, candidate.agentCursor, 0)
			}
			updated, _ := updateAgentList(candidate, tc.key)
			assert.Equal(t, tc.wantState, updated.state)
			if tc.wantEdit != "" {
				assert.Equal(t, tc.wantEdit, updated.fieldEditing)
			}
		})
	}
}

func TestT10MultiSelectUsesUniqueCatalogNamesIncludingAllRole(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), richGrouped(), 5, runtimeCatalogForTUI())
	updated, _ := updateAgentList(m, tea.KeyPressMsg{Text: "m"})

	assert.Contains(t, updated.multiSelectItems, "z-primary")
	assert.Contains(t, updated.multiSelectItems, "custom-hidden")
	assert.Contains(t, updated.multiSelectItems, "all-role")
	assert.Equal(t, 1, strings.Count(strings.Join(updated.multiSelectItems, "\n"), "a-primary"))
}

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
	// Lipgloss v2 emits per-rune SGR sequences for faint+strikethrough rows,
	// so the disabled-row contract is asserted on the ANSI-stripped render
	// (the design contract: markers survive ANSI stripping).
	out := testANSISequence.ReplaceAllString(viewAgentList(m), "")
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
	assert.Contains(t, out, "S Review & Save · Q Quit")
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
	assert.Equal(t, 0, m.agentCursor, "precondition: cursor starts at 0 (global)")

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "j"})
	assert.Equal(t, 1, newM.agentCursor, "cursor MUST be 1 after pressing 'j'")
}

// TestUpdateAgentList_J_SkipsDisabledAgents verifies that moving down from the
// global entry lands on the first non-disabled primary agent ("plan"), NOT on
// the disabled agent "build".
//
// Spec: REQ-TUI-003 — Edge case — j skips disabled agents.
func TestUpdateAgentList_J_SkipsDisabledAgents(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "j"})

	items := selectableItems(newM)
	require.True(t, newM.agentCursor < len(items), "cursor must be in bounds")
	assert.Equal(t, "plan", items[newM.agentCursor],
		"after 'j' from global, cursor MUST be on 'plan' (first non-disabled primary), NOT 'build' (disabled)")
}

// TestUpdateAgentList_K_MovesCursorUp verifies that pressing 'k' decrements the
// cursor.
//
// Spec: REQ-TUI-003 — Happy path — k navigates up.
func TestUpdateAgentList_K_MovesCursorUp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = 2

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "k"})
	assert.Equal(t, 1, newM.agentCursor, "cursor MUST be 1 after pressing 'k' from 2")
}

// TestUpdateAgentList_K_AtTopStaysAtZero verifies that pressing 'k' at cursor 0
// keeps the cursor at 0 (no negative wrap).
func TestUpdateAgentList_K_AtTopStaysAtZero(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = 0

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "k"})
	assert.Equal(t, 0, newM.agentCursor,
		"cursor MUST stay at 0 when pressing 'k' at the top")
}

// TestUpdateAgentList_DownArrow_MovesDown verifies that the Down arrow key
// works the same as 'j'.
//
// Spec: REQ-TUI-003 — Down arrow navigates down.
func TestUpdateAgentList_DownArrow_MovesDown(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, newM.agentCursor, "Down arrow MUST move cursor down")
}

// TestUpdateAgentList_UpArrow_MovesUp verifies that the Up arrow key works the
// same as 'k'.
//
// Spec: REQ-TUI-003 — Up arrow navigates up.
func TestUpdateAgentList_UpArrow_MovesUp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = 2
	newM, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 1, newM.agentCursor, "Up arrow MUST move cursor up")
}

// TestUpdateAgentList_EnterOnGlobal_TransitionsToModelSelection verifies that
// pressing ENTER on the global default model entry transitions to the Model
// Selection screen.
//
// Spec: REQ-TUI-003 — Happy path — ENTER on global opens model selection.
func TestUpdateAgentList_EnterOnGlobal_TransitionsToModelSelection(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.agentCursor = 0 // __global__

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state,
		"ENTER on global MUST transition to ScreenModelSelection")
}

// TestUpdateAgentList_EnterOnAgent_TransitionsToModelSelection verifies that
// pressing ENTER on an agent transitions directly to model selection and sets
// selectedAgent.
//
// Spec: REQ-TUI-003 — Happy path — ENTER on agent opens detail.
func TestUpdateAgentList_EnterOnAgent_TransitionsToModelSelection(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	// cursor 1 = "plan" (first non-disabled primary agent)
	m.agentCursor = 1

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, newM.state,
		"ENTER on an agent MUST transition directly to ScreenModelSelection")
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

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "s"})
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

	newM, _ := updateAgentList(m, tea.KeyPressMsg{Text: "s"})
	assert.Equal(t, ScreenAgentList, newM.state,
		"'s' when NOT dirty MUST stay on ScreenAgentList")
}

// TestUpdateAgentList_Q_Quits verifies that pressing 'q' produces a tea.Quit
// command.
//
// Spec: REQ-TUI-003 — Happy path — q quits.
func TestUpdateAgentList_Q_Quits(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	_, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "q"})
	require.NotNil(t, cmd, "'q' MUST produce a non-nil command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'q' MUST produce a tea.QuitMsg")
}

// TestUpdateAgentList_OtherKey_NoOp verifies that unmapped keys do not change
// state or cursor.
func TestUpdateAgentList_OtherKey_NoOp(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "x"})
	assert.Equal(t, ScreenAgentList, newM.state, "unmapped key MUST not change state")
	assert.Equal(t, 0, newM.agentCursor, "unmapped key MUST not move cursor")
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

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "q"})
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

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "q"})
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

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
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

	_, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "y"})
	require.NotNil(t, cmd, "'y' in quitConfirm MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'y' in quitConfirm MUST produce tea.QuitMsg")
}

// TestUpdateAgentList_QuitConfirm_UpperY_ConfirmsQuit verifies that pressing
// uppercase 'Y' also confirms the quit.
func TestUpdateAgentList_QuitConfirm_UpperY_ConfirmsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	_, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "Y"})
	require.NotNil(t, cmd, "'Y' in quitConfirm MUST produce a quit command")
	assert.IsType(t, tea.QuitMsg{}, cmd(), "'Y' in quitConfirm MUST produce tea.QuitMsg")
}

// TestUpdateAgentList_QuitConfirm_Enter_ConfirmsQuit verifies that pressing
// ENTER while the quit confirmation is showing confirms the quit.
func TestUpdateAgentList_QuitConfirm_Enter_ConfirmsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	_, cmd := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "n"})
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

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "N"})
	assert.False(t, newM.quitConfirm, "'N' MUST clear quitConfirm")
	assert.Nil(t, cmd, "'N' MUST NOT produce a quit command")
}

// TestUpdateAgentList_QuitConfirm_Esc_CancelsQuit verifies that pressing ESC
// while the quit confirmation is showing cancels the quit.
func TestUpdateAgentList_QuitConfirm_Esc_CancelsQuit(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEsc})
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
	m.agentCursor = 0

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "j"})
	assert.True(t, newM.quitConfirm, "'j' MUST NOT clear quitConfirm")
	assert.Equal(t, 0, newM.agentCursor, "'j' MUST NOT move cursor during quitConfirm")
	assert.Nil(t, cmd, "'j' MUST NOT produce a command during quitConfirm")
}

// TestUpdateAgentList_QuitConfirm_K_Ignored verifies that pressing 'k' while
// the quit confirmation is showing does not move the cursor.
func TestUpdateAgentList_QuitConfirm_K_Ignored(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true
	m.agentCursor = 2

	newM, cmd := updateAgentList(m, tea.KeyPressMsg{Text: "k"})
	assert.True(t, newM.quitConfirm, "'k' MUST NOT clear quitConfirm")
	assert.Equal(t, 2, newM.agentCursor, "'k' MUST NOT move cursor during quitConfirm")
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
	m.agentCursor = 1 // plan

	// Step 1: 'q' with dirty → quitConfirm=true
	m1, _ := updateAgentList(m, tea.KeyPressMsg{Text: "q"})
	require.True(t, m1.quitConfirm, "step 1: quitConfirm must be set")

	// Step 2: 'n' → cancel, quitConfirm=false
	m2, _ := updateAgentList(m1, tea.KeyPressMsg{Text: "n"})
	require.False(t, m2.quitConfirm, "step 2: quitConfirm must be cleared")

	// Step 3: ENTER → transitions directly to model selection.
	m3, _ := updateAgentList(m2, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ScreenModelSelection, m3.state,
		"ENTER after canceling quit MUST transition normally")
	assert.False(t, m3.quitConfirm,
		"quitConfirm MUST be false after transitioning to model selection")
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

func TestViewAgentList_ShowsSuccessBannerAfterSave(t *testing.T) {
	cfg := fixtureConfig(t)
	grouped := sampleGrouped()
	m := NewModel(cfg, grouped, 5)
	m.saveSuccess = true
	out := viewAgentList(m)
	assert.Contains(t, out, "Saved", "should show 'Saved' in banner")
	assert.True(t, containsAny(out, "✓", "✔"), "should have a checkmark")
}

func TestViewAgentList_NoSuccessBannerWhenNotSaved(t *testing.T) {
	cfg := fixtureConfig(t)
	grouped := sampleGrouped()
	m := NewModel(cfg, grouped, 5)
	m.saveSuccess = false
	out := viewAgentList(m)
	assert.NotContains(t, out, "Saved successfully", "should not show banner when not saved")
}

// TestViewAgentList_ShowsEffectiveModelAfterPickerRoundTrip is the round-trip
// regression for the stale-model bug: compactFieldValue used to prioritize the
// discovery-time catalog snapshot (catalogByName[name].Model), so after
// selecting a NEW model in the picker and returning to the agent list, the row
// still showed the frozen pre-selection model instead of the live effective
// model.
//
// Round-trip reproduced at the unit level with the real production handlers:
//  1. ENTER on a catalog agent whose row shows its resolved snapshot model.
//  2. selectModelAtCursor commits a different model (inline-JSON override,
//     dirty, pending save) and pops back to ScreenAgentList.
//  3. viewAgentList must render the newly selected effective model.
//
// The pre-selection assertion also pins the fallback contract: an agent with
// no JSON/md/global model still displays its catalog-resolved value, so the
// fix must preserve the global/md fallbacks — not drop catalog-only models.
func TestViewAgentList_ShowsEffectiveModelAfterPickerRoundTrip(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), richGrouped(), 5, runtimeCatalogForTUI())

	// Precondition: z-primary currently resolves to its catalog snapshot.
	pre := viewAgentList(m)
	assert.Contains(t, pre, "runtime/z",
		"pre-selection: the catalog-resolved model must still be displayed")

	// Step 1: ENTER on z-primary → model selection screen.
	items := selectableItems(m)
	m.agentCursor = indexOf(items, "z-primary")
	require.GreaterOrEqual(t, m.agentCursor, 0)
	picker, _ := updateAgentList(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, ScreenModelSelection, picker.state)
	require.Equal(t, "z-primary", picker.selectedAgent)

	// Step 2: commit a different model at the picker cursor.
	target := -1
	for i, mdl := range picker.filteredModels {
		if mdl.FullName == "opencode-go/glm-5.2" {
			target = i
			break
		}
	}
	require.GreaterOrEqual(t, target, 0, "picker must offer opencode-go/glm-5.2")
	picker.modelCursor = target
	after := selectModelAtCursor(picker)

	// Step 3: back on the agent list — the effective model must be live.
	require.Equal(t, ScreenAgentList, after.state,
		"selectModelAtCursor must pop back to the agent list")
	require.True(t, after.dirty,
		"an unsaved model selection must mark the model dirty")
	out := viewAgentList(after)
	assert.Contains(t, out, "opencode-go/glm-5.2",
		"agent list MUST show the newly selected model immediately after returning from the picker")
	assert.NotContains(t, out, "runtime/z",
		"the stale catalog snapshot MUST NOT be displayed once a live override exists")
}

// TestViewAgentList_PerformsSingleMarkdownPassPerRender pins the render-cost
// contract behind the live-model fix: every markdown-dependent config read
// during an agent-list render MUST route through mergedAgentsForRender, and
// the render MUST trigger it exactly ONCE — not once per rendered row.
//
// Why: Config.MergedAgents (and ResolveEffectiveModel / GetMergedAgentField,
// which call the same discovery internally) re-glob and re-parse the global
// and project markdown agent directories on EVERY invocation. Resolving the
// effective model per row therefore multiplies filesystem I/O by the agent
// count on every frame. The count is asserted per viewAgentList call in both
// layout modes (unsized fallback path and the viewport path), so a regression
// to per-row resolution fails on either side: N passes (> 1) when rows
// resolve through discovery again, or 0 passes when the render bypasses the
// shared snapshot entirely.
func TestViewAgentList_PerformsSingleMarkdownPassPerRender(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), richGrouped(), 5, runtimeCatalogForTUI())
	rows := len(m.primaryAgents) + len(m.subagents) + len(m.allAgents)
	require.Greater(t, rows, 1, "fixture must render multiple agent rows for this oracle to be meaningful")

	calls := 0
	orig := mergedAgentsForRender
	mergedAgentsForRender = func(c *config.Config) map[string]*config.MergedAgent {
		calls++
		return orig(c)
	}
	t.Cleanup(func() { mergedAgentsForRender = orig })

	// Render 1: unsized fallback path (width/height == 0).
	assert.NotEmpty(t, viewAgentList(m))
	assert.Equal(t, 1, calls,
		"one agent-list render MUST perform exactly one markdown-discovery pass; got %d for %d rows (per-row resolution rediscovers agents once per row)",
		calls, rows)

	// Render 2: sized viewport path — same contract, fresh count baseline.
	calls = 0
	m.width, m.height = 80, 24
	assert.NotEmpty(t, viewAgentList(m))
	assert.Equal(t, 1, calls,
		"the viewport render path MUST also perform exactly one markdown-discovery pass; got %d for %d rows",
		calls, rows)
}

func TestViewAgentList_CatalogWarningUsesBorderedOverlayBox(t *testing.T) {
	catalog := runtimeCatalogForTUI()
	catalog.Degraded = true
	catalog.Diagnostics = []agentcatalog.Diagnostic{{Message: "runtime unavailable; using fallback"}}
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, catalog)
	out := viewAgentList(m)

	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "⚠ runtime unavailable; using fallback")
	assert.Contains(t, stripped, "╭", "catalog warning must be enclosed in a bordered overlay box")
	assert.Contains(t, stripped, "╰")
}

func TestViewAgentList_SaveSuccessUsesBorderedOverlayBox(t *testing.T) {
	cfg := fixtureConfig(t)
	m := NewModel(cfg, sampleGrouped(), 5)
	m.saveSuccess = true
	out := viewAgentList(m)

	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "✓ Saved successfully")
	assert.Contains(t, stripped, "╭", "save success must be enclosed in a bordered overlay box")
	assert.Contains(t, stripped, "╰")
}

func TestViewAgentList_QuitConfirmUsesBorderedOverlayBox(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.dirty = true
	m.quitConfirm = true
	out := viewAgentList(m)

	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "⚠ You have unsaved changes. Quit anyway? (y/n)")
	assert.Contains(t, stripped, "╭", "quit confirmation must be enclosed in a bordered overlay box")
	assert.Contains(t, stripped, "╰")
}

func TestViewAgentList_OverlaysBoundedAtNarrowTerminalWidth(t *testing.T) {
	cfg := fixtureConfig(t)
	catalog := runtimeCatalogForTUI()
	catalog.Degraded = true
	catalog.Diagnostics = []agentcatalog.Diagnostic{{Message: "runtime unavailable; using fallback"}}
	m := NewModelWithCatalog(cfg, sampleGrouped(), 5, catalog)
	m.saveSuccess = true
	m = resizeModel(t, m, 40, 18)

	out := m.View().Content
	assertRenderedWidthAtMost(t, out, 40)
	assert.LessOrEqual(t, renderedLineCount(out), 18)
}

func TestViewAgentList_AgentRowMarkersAndBadgesSurviveAnsiStrip(t *testing.T) {
	cfg := fixtureConfig(t)
	m := NewModelWithCatalog(cfg, richGrouped(), 5, runtimeCatalogForTUI())
	m.agentCursor = 0 // __global__
	out := viewAgentList(m)

	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "▶", "selected global row must show cursor prefix")
	assert.Contains(t, stripped, "[Global Default Model]")
	assert.Contains(t, stripped, "◆ Primary Agents", "primary agents section header must include diamond marker")
	assert.Contains(t, stripped, "◆ Subagents", "subagents section header must include diamond marker")
	assert.Contains(t, stripped, "◆ All Agents", "all agents section header must include diamond marker")
	assert.Contains(t, stripped, "[H]", "hidden agent must display [H] badge")
	assert.Contains(t, stripped, "· model", "agent row must include model separator")
}

func TestAgentListViewportHeight_DeductsOverlayHeights(t *testing.T) {
	m := NewModelWithCatalog(fixtureConfig(t), sampleGrouped(), 5, runtimeCatalogForTUI())
	m.width = 80
	m.height = 24

	baseHeight := agentListViewportHeight(m)
	require.Positive(t, baseHeight)

	// Save success overlay
	m.saveSuccess = true
	saveHeight := agentListViewportHeight(m)
	assert.Less(t, saveHeight, baseHeight, "save success overlay must reduce viewport height")

	// Quit confirm overlay
	m.quitConfirm = true
	quitHeight := agentListViewportHeight(m)
	assert.Less(t, quitHeight, saveHeight, "quit confirmation overlay must further reduce viewport height")

	// Degraded catalog warning
	m.agentCatalog.Degraded = true
	m.agentCatalog.Diagnostics = []agentcatalog.Diagnostic{{Message: "warning"}}
	degradedHeight := agentListViewportHeight(m)
	assert.Less(t, degradedHeight, quitHeight, "catalog warning overlay must further reduce viewport height")
}
