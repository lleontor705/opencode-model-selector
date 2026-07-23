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
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	m.previousState = ScreenAgentList
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
	m.previousState = ScreenAgentList
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
	m.previousState = ScreenAgentList
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
	m.previousState = ScreenAgentList
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
	m.previousState = ScreenAgentList
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
	assert.Contains(t, out, "ENTER")
	assert.Contains(t, out, "ESC")
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
