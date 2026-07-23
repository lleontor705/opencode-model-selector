// Package tui — save confirmation screen implementation (REQ-TUI-007).
//
// This file implements the Save Confirm screen: rendering and key handling.
// It shows a summary of the pending save (config path, backup retention) and
// triggers the atomic save flow (backup → write → cleanup) on confirmation.
//
// Save flow (REQ-TUI-007):
//  1. dirty == false → "No changes to save", return to previousState
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
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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

	var b strings.Builder

	// --- Title ---
	b.WriteString(TitleStyle.Render("Save Changes?"))
	b.WriteString("\n\n")

	// --- Error message (shown if a previous save attempt failed) ---
	if m.saveError != "" {
		b.WriteString(ErrorStyle.Render(m.saveError))
		b.WriteString("\n\n")
	}

	// --- Summary ---
	b.WriteString("You have unsaved changes. Save to config?")
	b.WriteString("\n\n")

	b.WriteString("Config: " + m.config.Path())
	b.WriteByte('\n')
	b.WriteString("Backups: " + strconv.Itoa(m.backupCount) + " (retention)")
	b.WriteString("\n\n")

	// --- Diff preview ---
	if len(m.changes) > 0 {
		b.WriteString(DiffSummary.Render(fmt.Sprintf("Saving %d change%s:", len(m.changes), plural(len(m.changes)))))
		b.WriteString("\n")
		for _, ch := range m.changes {
			fmt.Fprintf(&b, "  %s.%s: %s -> %s\n",
				ch.Target, ch.Field,
				formatValue(ch.OldVal),
				formatValue(ch.NewVal))
		}
		b.WriteString("\n")
	}

	// --- Help footer ---
	b.WriteString(HelpStyle.Render("ENTER: save  ESC: cancel"))

	return b.String()
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
//   - ESC / 'n':   cancel → return to previousState without saving
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
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'y'):
		return performSave(m)

	// --- ESC or 'n': cancel ---
	case keyMsg.Type == tea.KeyEsc || keyMsg.Type == tea.KeyEscape ||
		(keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == 'n'):
		m.state = m.previousState
		return m, nil

	// --- Unmapped key: no-op ---
	default:
		return m, nil
	}
}

// performSave executes the save operation: backup → write → cleanup.
//
// Flow:
//  1. If not dirty: set informational message and return to previousState.
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
		m.state = m.previousState
		return m, nil
	}

	// --- Create backup (if retention > 0) ---
	if m.backupCount > 0 {
		if _, err := config.CreateBackup(m.config.Path()); err != nil {
			m.saveError = "Backup failed: " + err.Error()
			// Stay on screen, keep dirty so the user can retry.
			return m, nil
		}
	}

	// --- Save config atomically ---
	if err := m.config.Save(); err != nil {
		m.saveError = "Save failed: " + err.Error()
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
	return m, nil
}
