// Package tui — save confirmation screen implementation (REQ-TUI-007).
//
// This file implements the Save Confirm screen: rendering and key handling.
// It shows a summary of the pending save (config path, backup retention) and
// triggers the atomic save flow (backup → write → cleanup) on confirmation.
//
// Save flow (REQ-TUI-007):
//  1. dirty == false → "No changes to save", return to immutable origin
//  2. dirty == true:
//     a. backupCount > 0 → CreateBackup (on error: show error, stay)
//     b. config.Save() (on error: show error, keep dirty, stay)
//     c. backupCount > 0 → CleanOldBackups (best-effort)
//     d. dirty = false, saveSuccess = true
//     e. Return to ScreenAgentList
//
// Spec coverage:
//   - REQ-TUI-007: Save confirm rendering (title, config path, backup count)
//   - REQ-TUI-007: Save flow (backup, write, cleanup, success/failure)
//   - REQ-TUI-007: Interaction (ENTER/y confirm, ESC/n cancel)
package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lleontor705/opencode-model-selector/internal/config"
)

// viewSaveConfirm renders the Save Confirm screen. The layout is:
//
//	Save Changes?
//
//	You have unsaved changes. Save to config?
//
//	Config: <path>
//	Backups: <count> (retention)
//
//	Saving N change(s):
//	  <target>.<field>: <old> -> <new>
//	  ...
//
//	<error message if any>
//
//	ENTER: save  ESC: cancel
//
// Spec: REQ-TUI-007 — rendering.
func viewSaveConfirm(m Model) string {
	if m.config == nil {
		return ErrorStyle.Render("no config loaded")
	}

	header := saveReviewHeader(m)
	footer := HelpStyle.Render("Enter/Y Save to disk · Esc/N Back")
	if m.width <= 0 || m.height <= 0 {
		content := saveReviewContent(m)
		if content == "" {
			return strings.Join([]string{header, footer}, "\n")
		}
		return strings.Join([]string{header, content, footer}, "\n")
	}

	syncSaveViewport(&m)
	return strings.Join([]string{
		clipLines(header, m.width),
		m.saveViewport.View(),
		clipLines(footer, m.width),
	}, "\n")
}

func saveReviewHeader(m Model) string {
	parts := []string{
		TitleStyle.Render("Review changes"),
		"Save changes to opencode.json?",
		"Config: " + m.config.Path(),
	}
	if m.backupCount > 0 {
		backupPath := filepath.Join(filepath.Dir(m.config.Path()), "opencode.json.backup.YYYYMMDD-HHMMSS")
		parts = append(parts,
			"Backup: "+backupPath,
			"Retention: keep "+strconv.Itoa(m.backupCount)+" backups",
		)
	} else {
		parts = append(parts, "Backup: disabled (retention count is 0)")
	}
	if m.saveError != "" {
		parts = append(parts, ErrorStyle.Render(m.saveError))
	}
	return strings.Join(parts, "\n")
}

func saveReviewContent(m Model) string {
	if len(m.changes) == 0 {
		return HelpStyle.Render("No net changes to review")
	}
	var b strings.Builder
	b.WriteString(DiffSummary.Render(fmt.Sprintf("%d net change%s:", len(m.changes), plural(len(m.changes)))))
	b.WriteByte('\n')
	for _, ch := range m.changes {
		fmt.Fprintf(&b, "  %s.%s: %s -> %s\n",
			ch.Target, ch.Field, formatValue(ch.OldVal), formatValue(ch.NewVal))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func saveReviewViewportHeight(m Model) int {
	if m.height <= 0 || m.config == nil {
		return 0
	}
	fixed := lipgloss.Height(saveReviewHeader(m)) + lipgloss.Height(HelpStyle.Render("Enter/Y Save to disk · Esc/N Back"))
	return max(1, m.height-fixed-2)
}

func syncSaveViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 || m.config == nil {
		return
	}
	m.saveViewport.Width = max(1, m.width)
	m.saveViewport.Height = saveReviewViewportHeight(*m)
	m.saveViewport.SetContent(clipLines(saveReviewContent(*m), m.width))
}

// formatValue renders a config value (interface{}) as a human-readable string
// for the save-confirm diff preview. Nil and empty values get explicit
// placeholders so the user can distinguish "no change yet" from "cleared".
func formatValue(v interface{}) string {
	if v == nil {
		return "(none)"
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return "(empty)"
		}
		return val
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// updateSaveConfirm handles key presses on the Save Confirm screen.
//
// Keys:
//   - ENTER / 'y': confirm save → performSave()
//   - ESC / 'n':   cancel → return to immutable origin without saving
//
// Spec: REQ-TUI-007 — interaction.
func updateSaveConfirm(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	// --- ENTER or 'y': confirm save ---
	case keyMsg.Type == tea.KeyEnter ||
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 &&
			(keyMsg.Runes[0] == 'y' || keyMsg.Runes[0] == 'Y')):
		return performSave(m)

	// --- ESC or 'n': cancel ---
	case keyMsg.Type == tea.KeyEsc || keyMsg.Type == tea.KeyEscape ||
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 &&
			(keyMsg.Runes[0] == 'n' || keyMsg.Runes[0] == 'N')):
		m.popScreen()
		return m, nil

	// --- Scroll the diff while keeping modal actions fixed ---
	case keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyDown ||
		keyMsg.Type == tea.KeyPgUp || keyMsg.Type == tea.KeyPgDown:
		syncSaveViewport(&m)
		var cmd tea.Cmd
		m.saveViewport, cmd = m.saveViewport.Update(keyMsg)
		return m, cmd

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// performSave executes the save operation: backup → write → cleanup.
//
// Flow:
//  1. If not dirty: set informational message and return to immutable origin.
//  2. If backupCount > 0: create a timestamped backup. On failure, show error
//     and stay on screen.
//  3. Save config atomically. On failure, show error, keep dirty, stay.
//  4. If backupCount > 0: clean old backups (best-effort — errors are ignored).
//  5. Mark dirty=false, saveSuccess=true, return to ScreenAgentList.
//
// Spec: REQ-TUI-007 — save flow.
func performSave(m Model) (Model, tea.Cmd) {
	// --- No changes to save ---
	if !m.dirty {
		m.saveError = "No changes to save"
		m.popScreen()
		return m, nil
	}

	// --- Create backup (if retention > 0) ---
	if m.backupCount > 0 {
		if _, err := config.CreateBackup(m.config.Path()); err != nil {
			m.saveError = "Backup failed; verify the config directory is writable, then retry: " + err.Error()
			// Stay on screen, keep dirty so the user can retry.
			return m, nil
		}
	}

	// --- Save config atomically ---
	if err := m.config.Save(); err != nil {
		m.saveError = "Save failed; verify disk space and file permissions, then retry: " + err.Error()
		// Stay on screen, keep dirty.
		return m, nil
	}

	// --- Clean old backups (best-effort — do not fail save on cleanup error) ---
	if m.backupCount > 0 {
		_ = config.CleanOldBackups(m.config.Path(), m.backupCount)
	}

	// --- Success ---
	m.dirty = false
	m.changes = nil
	m.saveError = ""
	m.saveSuccess = true
	m.state = ScreenAgentList
	m.navigationStack = nil
	return m, nil
}
