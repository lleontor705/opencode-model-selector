// Package tui tests — save confirmation screen (REQ-TUI-007).
//
// These tests follow strict TDD: they were written BEFORE the production code
// in save_confirm.go, and drive its design. Coverage focuses on:
//   - performSave: dirty=false no-op, dirty=true full save flow (backup → write
//     → cleanup), backup-skip when backupCount=0, failure handling.
//   - updateSaveConfirm: ENTER/y confirm, ESC/n cancel.
//   - viewSaveConfirm: rendering shows config path, backup retention, help.
//
// Spec: REQ-TUI-007 (save confirmation and atomic save flow).
package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lleontor705/opencode-model-selector/internal/config"
)

// newSaveConfirmModel constructs a Model positioned on the Save Confirm screen
// with the read-only fixture config. Suitable for interaction tests that do
// not perform actual filesystem writes.
func newSaveConfirmModel(t *testing.T, dirty bool) Model {
	t.Helper()
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	m.dirty = dirty
	return m
}

// writableConfig copies the fixture opencode.json into a temp directory and
// loads it. The temp path is writable, so Save() and CreateBackup() succeed.
func writableConfig(t *testing.T) *config.Config {
	t.Helper()
	srcPath := filepath.Join("..", "..", "test", "fixtures", "opencode.json")
	abs, err := filepath.Abs(srcPath)
	require.NoError(t, err)
	content, err := os.ReadFile(abs)
	require.NoError(t, err)

	dir := t.TempDir()
	dstPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(dstPath, content, 0o600))

	cfg, err := config.LoadConfig(dstPath)
	require.NoError(t, err)
	return cfg
}

// writableSaveConfirmModel creates a Model on the Save Confirm screen backed by
// a writable temp config. An optional unsaved change is applied so there is
// something to persist.
func writableSaveConfirmModel(t *testing.T, backupCount int) Model {
	t.Helper()
	cfg := writableConfig(t)
	m := NewModel(cfg, sampleGrouped(), backupCount)
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	// Make an actual change to the config so Save() has content to write.
	require.NoError(t, m.config.SetAgentField("code-reviewer", "temperature", 0.7))
	m.dirty = true
	return m
}

// countBackupsInDir counts files matching the backup glob in a directory.
func countBackupsInDir(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "opencode.json.backup.") {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — ENTER on dirty=false (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_NotDirty_Enter_ReturnsImmediately(t *testing.T) {
	m := newSaveConfirmModel(t, false)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, newM.state,
		"ENTER with no changes MUST return to previousState immediately")
	assert.False(t, newM.dirty, "dirty MUST remain false")
}

func TestUpdateSaveConfirm_NotDirty_NoWriteNoBackup(t *testing.T) {
	// Use a writable config to verify no files are written.
	cfg := writableConfig(t)
	dir := filepath.Dir(cfg.Path())
	before := countBackupsInDir(t, dir)

	m := NewModel(cfg, sampleGrouped(), 5)
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	m.dirty = false

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	after := countBackupsInDir(t, dir)

	assert.Equal(t, before, after,
		"no backup MUST be created when dirty=false")
	assert.False(t, newM.dirty)
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — ENTER on dirty=true (full save flow) (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_Dirty_Enter_SavesAndReturns(t *testing.T) {
	m := writableSaveConfirmModel(t, 5)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenAgentList, newM.state,
		"after successful save MUST return to ScreenAgentList")
	assert.False(t, newM.dirty, "dirty MUST be false after successful save")
	assert.True(t, newM.saveSuccess, "saveSuccess MUST be set after save")
}

func TestUpdateSaveConfirm_Dirty_Enter_CreatesBackup(t *testing.T) {
	m := writableSaveConfirmModel(t, 5)
	dir := filepath.Dir(m.config.Path())

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	backups := countBackupsInDir(t, dir)
	assert.GreaterOrEqual(t, backups, 1,
		"at least one backup MUST be created when backupCount > 0")

	_ = newM
}

func TestUpdateSaveConfirm_Dirty_Enter_PersistsConfigChange(t *testing.T) {
	m := writableSaveConfirmModel(t, 5)

	updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})

	// Reload from disk to verify the change was persisted.
	savedCfg, err := config.LoadConfig(m.config.Path())
	require.NoError(t, err)
	val, ok := savedCfg.GetAgentField("code-reviewer", "temperature")
	require.True(t, ok, "temperature MUST be persisted to disk")
	assert.Equal(t, 0.7, val)
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — ESC cancel (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_Dirty_Esc_ReturnsWithoutSaving(t *testing.T) {
	m := newSaveConfirmModel(t, true)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ScreenAgentList, newM.state,
		"ESC MUST return to previousState")
	assert.True(t, newM.dirty, "dirty MUST remain true when canceling save")
}

func TestUpdateSaveConfirm_Dirty_N_ReturnsWithoutSaving(t *testing.T) {
	m := newSaveConfirmModel(t, true)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, ScreenAgentList, newM.state)
	assert.True(t, newM.dirty, "'n' MUST cancel without saving")
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — 'y' confirm (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_Y_ConfirmsSave(t *testing.T) {
	m := writableSaveConfirmModel(t, 5)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, ScreenAgentList, newM.state,
		"'y' MUST trigger the save flow")
	assert.False(t, newM.dirty)
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — backupCount=0 skips backup (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_BackupCountZero_SkipsBackupStillSaves(t *testing.T) {
	m := writableSaveConfirmModel(t, 0)
	dir := filepath.Dir(m.config.Path())
	before := countBackupsInDir(t, dir)

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	after := countBackupsInDir(t, dir)

	assert.Equal(t, before, after,
		"backupCount=0 MUST NOT create any backup files")
	assert.False(t, newM.dirty, "config MUST still be saved successfully")

	// Verify the config was actually written to disk.
	savedCfg, err := config.LoadConfig(m.config.Path())
	require.NoError(t, err)
	val, ok := savedCfg.GetAgentField("code-reviewer", "temperature")
	require.True(t, ok, "config change MUST be persisted even with backupCount=0")
	assert.Equal(t, 0.7, val)
}

// ---------------------------------------------------------------------------
// updateSaveConfirm — save failure (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestUpdateSaveConfirm_SaveFails_ShowsErrorStaysDirty(t *testing.T) {
	// Load a writable config, then delete the directory so Save() fails.
	cfg := writableConfig(t)
	dir := filepath.Dir(cfg.Path())
	require.NoError(t, os.RemoveAll(dir),
		"precondition: remove temp dir to make path unwritable")

	m := NewModel(cfg, sampleGrouped(), 0) // backupCount=0 to test Save failure directly
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	require.NoError(t, m.config.SetAgentField("code-reviewer", "temperature", 0.7))
	m.dirty = true

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenSaveConfirm, newM.state,
		"on save failure MUST stay on the save confirm screen")
	assert.True(t, newM.dirty, "dirty MUST remain true on save failure")
	assert.NotEmpty(t, newM.saveError, "saveError MUST be set on failure")
}

func TestUpdateSaveConfirm_BackupFails_ShowsErrorStaysDirty(t *testing.T) {
	// Load a writable config, then delete the directory so CreateBackup fails.
	cfg := writableConfig(t)
	require.NoError(t, os.RemoveAll(filepath.Dir(cfg.Path())))

	m := NewModel(cfg, sampleGrouped(), 5) // backupCount>0 to trigger backup
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	m.dirty = true

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ScreenSaveConfirm, newM.state,
		"on backup failure MUST stay on the save confirm screen")
	assert.True(t, newM.dirty, "dirty MUST remain true on backup failure")
	assert.NotEmpty(t, newM.saveError)
}

// ---------------------------------------------------------------------------
// viewSaveConfirm — rendering (REQ-TUI-007)
// ---------------------------------------------------------------------------

func TestViewSaveConfirm_ShowsTitle(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := viewSaveConfirm(m)
	assert.Contains(t, out, "Save",
		"the save confirm screen MUST have a title mentioning save")
}

func TestViewSaveConfirm_ShowsConfigPath(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := viewSaveConfirm(m)
	assert.Contains(t, out, m.config.Path(),
		"the config file path MUST be displayed")
}

func TestViewSaveConfirm_ShowsBackupRetention(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := viewSaveConfirm(m)
	assert.Contains(t, out, "5",
		"the backup retention count MUST be displayed")
}

func TestViewSaveConfirm_ShowsHelpFooter(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := viewSaveConfirm(m)
	assert.Contains(t, out, "Enter/Y Save to disk")
	assert.Contains(t, out, "Esc/N Back")
}

func TestViewSaveConfirm_ShowsErrorWhenPresent(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.saveError = "Save failed: disk full"
	out := viewSaveConfirm(m)
	assert.Contains(t, out, "Save failed",
		"saveError MUST be rendered when set")
}

func TestViewSaveConfirm_NilConfigDoesNotPanic(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.config = nil
	require.NotPanics(t, func() {
		out := viewSaveConfirm(m)
		assert.NotEmpty(t, out)
	})
}

// ---------------------------------------------------------------------------
// Dispatcher integration (REQ-TUI-008)
// ---------------------------------------------------------------------------

func TestUpdate_DispatchesToSaveConfirm(t *testing.T) {
	m := newSaveConfirmModel(t, false)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result, ok := newM.(Model)
	require.True(t, ok)
	assert.Equal(t, ScreenAgentList, result.state,
		"global Update() MUST dispatch ScreenSaveConfirm to updateSaveConfirm")
}

func TestView_DispatchesToSaveConfirm(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := m.View()
	assert.Contains(t, out, "Save",
		"global View() MUST dispatch ScreenSaveConfirm to viewSaveConfirm")
}

// ---------------------------------------------------------------------------
// Diff preview (REQ-TUI-007 — show pending changes before save)
// ---------------------------------------------------------------------------

func TestViewSaveConfirm_ShowsChanges(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.changes = []Change{
		{Target: "code-reviewer", Field: "model", OldVal: "anthropic/claude-sonnet-4-20250514", NewVal: "opencode-go/glm-5.2"},
		{Target: "plan", Field: "temperature", OldVal: 0.4, NewVal: 0.7},
	}
	out := viewSaveConfirm(m)
	assert.Contains(t, out, "code-reviewer.model",
		"each change MUST render as Target.Field")
	assert.Contains(t, out, "plan.temperature",
		"each change MUST render as Target.Field")
	assert.Contains(t, out, "2 net changes",
		"the header MUST show the total change count")
	assert.Contains(t, out, "opencode-go/glm-5.2",
		"the new value MUST appear in the diff line")
	assert.Contains(t, out, "anthropic/claude-sonnet-4-20250514",
		"the old value MUST appear in the diff line")
}

func TestViewSaveConfirm_NoChanges_OmitsDiff(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.changes = nil
	out := viewSaveConfirm(m)
	assert.NotContains(t, out, "Saving",
		"diff section MUST be omitted when there are no pending changes")
}

func TestSelectModelAtCursor_RecordsChange(t *testing.T) {
	m := newModelSelectModel(t, "global", "")
	m.modelCursor = 0

	before, _ := m.config.GetGlobalModel()
	newM := selectModelAtCursor(m)

	require.Len(t, newM.changes, 1,
		"selecting a model MUST record exactly one change")
	assert.Equal(t, "global", newM.changes[0].Target,
		"the change target MUST be 'global' for global edits")
	assert.Equal(t, "model", newM.changes[0].Field,
		"the change field MUST be 'model'")
	assert.Equal(t, before, newM.changes[0].OldVal,
		"the change MUST capture the previous global model as OldVal")
	assert.NotEqual(t, before, newM.changes[0].NewVal,
		"the change MUST capture the newly-selected model as NewVal")
	assert.True(t, newM.dirty,
		"the model MUST be dirty after a mutation")
}

func TestSelectModelAtCursor_PerAgent_RecordsChange(t *testing.T) {
	m := newModelSelectModel(t, "model", "code-reviewer")
	m.modelCursor = 0

	before, _ := m.config.GetAgentField("code-reviewer", "model")
	newM := selectModelAtCursor(m)

	require.Len(t, newM.changes, 1)
	assert.Equal(t, "code-reviewer", newM.changes[0].Target,
		"the change target MUST be the agent name for per-agent edits")
	assert.Equal(t, "model", newM.changes[0].Field)
	assert.Equal(t, before, newM.changes[0].OldVal)
}

func TestRecordChange_SetsDirty(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	require.False(t, m.dirty, "precondition: a fresh model is not dirty")

	m.RecordChange("plan", "temperature", 0.4, 0.7)

	assert.True(t, m.dirty,
		"RecordChange MUST mark the model dirty")
	require.Len(t, m.changes, 1)
	assert.Equal(t, "plan", m.changes[0].Target)
	assert.Equal(t, "temperature", m.changes[0].Field)
	assert.Equal(t, 0.4, m.changes[0].OldVal)
	assert.Equal(t, 0.7, m.changes[0].NewVal)
}

func TestPerformSave_ClearsChanges(t *testing.T) {
	m := writableSaveConfirmModel(t, 5)
	m.changes = []Change{
		{Target: "code-reviewer", Field: "temperature", OldVal: nil, NewVal: 0.7},
	}
	require.NotEmpty(t, m.changes, "precondition: model has pending changes")

	newM, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, newM.dirty, "dirty MUST be false after a successful save")
	assert.Empty(t, newM.changes,
		"changes MUST be cleared after a successful save so a re-entry to the screen starts fresh")
}

func TestCommitFieldInput_RecordsChange(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenFieldInput
	m.navigationStack = []appState{ScreenAgentDetail}
	m.selectedAgent = "code-reviewer"
	m.fieldEditing = "temperature"
	m.fieldInput.SetValue("0.5")

	before, _ := m.config.GetAgentField("code-reviewer", "temperature")
	newM := commitFieldInput(m)

	require.Len(t, newM.changes, 1,
		"committing a field value MUST record a change")
	assert.Equal(t, "code-reviewer", newM.changes[0].Target)
	assert.Equal(t, "temperature", newM.changes[0].Field)
	assert.Equal(t, before, newM.changes[0].OldVal,
		"OldVal MUST reflect the previous field value")
	assert.Equal(t, 0.5, newM.changes[0].NewVal,
		"NewVal MUST be the parsed value, not the raw input string")
}

func TestToggleDisable_RecordsChange(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.state = ScreenAgentDetail
	m.selectedAgent = "plan"
	require.False(t, m.config.IsAgentDisabled("plan"),
		"precondition: 'plan' is enabled in the fixture")

	toggleDisable(&m)

	require.Len(t, m.changes, 1,
		"toggling disable MUST record a change")
	assert.Equal(t, "plan", m.changes[0].Target)
	assert.Equal(t, "disable", m.changes[0].Field)
	assert.Equal(t, false, m.changes[0].OldVal)
	assert.Equal(t, true, m.changes[0].NewVal)
}

// ---------------------------------------------------------------------------
// formatValue helper
// ---------------------------------------------------------------------------

func TestFormatValue(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil", nil, "(none)"},
		{"empty string", "", "(empty)"},
		{"non-empty string", "openai/gpt-5", "openai/gpt-5"},
		{"float64 whole", 1.0, "1"},
		{"float64 fractional", 0.7, "0.7"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", 42, "42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatValue(tc.in))
		})
	}
}

func TestPlural(t *testing.T) {
	assert.Equal(t, "", plural(1))
	assert.Equal(t, "s", plural(0))
	assert.Equal(t, "s", plural(2))
}

func TestViewSaveConfirm_LongDiffIsScrollableAndFooterVisible(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.width = 80
	m.height = 16
	for i := 0; i < 40; i++ {
		m.RecordChange("agent", "field-"+strconv.Itoa(i), i, i+1)
	}

	before := viewSaveConfirm(m)
	assert.LessOrEqual(t, lipgloss.Height(before), m.height)
	assert.Contains(t, before, "Review changes")
	assert.Contains(t, before, "Enter/Y Save to disk · Esc/N Back")
	assert.NotContains(t, before, "field-39")

	afterPage, _ := updateSaveConfirm(m, tea.KeyMsg{Type: tea.KeyPgDown})
	after := viewSaveConfirm(afterPage)
	assert.NotEqual(t, before, after)
	assert.Contains(t, after, "Enter/Y Save to disk · Esc/N Back")
}

func TestSaveConfirm_EscReturnsImmutableOrigin(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.navigationStack = []appState{ScreenAgentList, ScreenAgentDetail}

	for _, key := range []rune{'s', 's', 'q'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = updated.(Model)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	assert.Equal(t, ScreenAgentDetail, m.state)
	assert.Equal(t, []appState{ScreenAgentList}, m.navigationStack)
}

func TestSaveConfirm_UnrelatedKeysDoNotMutateState(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	m.changes = []Change{{Target: "plan", Field: "temperature", OldVal: 0.4, NewVal: 0.7}}
	originalStack := append([]appState(nil), m.navigationStack...)
	originalChanges := append([]Change(nil), m.changes...)

	for _, key := range []rune{'s', 'q', 'x'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = updated.(Model)
	}

	assert.Equal(t, ScreenSaveConfirm, m.state)
	assert.Equal(t, originalStack, m.navigationStack)
	assert.True(t, reflect.DeepEqual(originalChanges, m.changes))
	assert.True(t, m.dirty)
}

func TestSaveSuccess_ClearsOnNextUserAction(t *testing.T) {
	m := writableSaveConfirmModel(t, 0)
	m.RecordChange("code-reviewer", "temperature", nil, 0.7)
	m, _ = performSave(m)

	assert.Contains(t, viewAgentList(m), "✓ Saved successfully")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.False(t, m.saveSuccess)
	assert.NotContains(t, viewAgentList(m), "✓ Saved successfully")
}

func TestSaveReview_CoalescesRepeatedEditsToNetChange(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.RecordChange("plan", "temperature", 0.4, 0.5)
	m.RecordChange("plan", "temperature", 0.5, 0.7)

	require.Len(t, m.changes, 1)
	assert.Equal(t, 0.4, m.changes[0].OldVal)
	assert.Equal(t, 0.7, m.changes[0].NewVal)
	assert.True(t, m.dirty)
}

func TestSaveReview_RemovesNoOpRevertedChange(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.RecordChange("plan", "temperature", 0.4, 0.7)
	m.RecordChange("plan", "temperature", 0.7, 0.4)

	assert.Empty(t, m.changes)
	assert.False(t, m.dirty)
}

func TestSaveConfirm_ShowsBackupPathAndRetentionClearly(t *testing.T) {
	m := newSaveConfirmModel(t, true)
	out := viewSaveConfirm(m)

	assert.Contains(t, out, filepath.Join(filepath.Dir(m.config.Path()), "opencode.json.backup.YYYYMMDD-HHMMSS"))
	assert.Contains(t, out, "Retention: keep 5 backups")
	assert.Contains(t, out, "Save changes to opencode.json?")
}

func TestSaveConfirm_SuccessPreservesBackupBeforeWriteOrdering(t *testing.T) {
	cfg := writableConfig(t)
	original, ok := cfg.GetAgentField("plan", "temperature")
	require.True(t, ok)
	require.NoError(t, cfg.SetAgentField("plan", "temperature", 0.7))
	m := NewModel(cfg, sampleGrouped(), 5)
	m.state = ScreenSaveConfirm
	m.navigationStack = []appState{ScreenAgentList}
	m.RecordChange("plan", "temperature", original, 0.7)

	m, _ = performSave(m)
	require.True(t, m.saveSuccess)
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(cfg.Path()), "opencode.json.backup.*"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	backupBytes, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	assert.Contains(t, string(backupBytes), `"temperature": 0.4`)
	savedBytes, err := os.ReadFile(cfg.Path())
	require.NoError(t, err)
	assert.Contains(t, string(savedBytes), `"temperature": 0.7`)
}
