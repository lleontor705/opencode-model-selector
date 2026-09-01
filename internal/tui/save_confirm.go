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

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
//	  <target>.model: <old> -> <new>
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
	footer := saveReviewHelp(m.width)
	if m.width <= 0 || m.height <= 0 {
		content := saveReviewContent(m)
		body := strings.Join([]string{header, content, footer}, "\n")
		return OverlayBoxStyle.Render(body)
	}

	syncSaveViewport(&m)
	textAreaWidth := max(1, m.width-OverlayBoxStyle.GetHorizontalFrameSize())
	body := strings.Join([]string{
		clipLines(header, textAreaWidth),
		m.saveViewport.View(),
		clipLines(footer, textAreaWidth),
	}, "\n")

	box := OverlayBoxStyle.Width(textAreaWidth).Render(body)
	return clipLines(box, m.width)
}

func saveReviewHelp(width int) string {
	return renderResponsiveHelp(width,
		"Enter/Y Save to disk · Esc/N Back",
		"Y Save · N/Esc Back",
	)
}

func saveReviewHeader(m Model) string {
	base := filepath.Base(m.config.Path())
	parts := []string{
		TitleStyle.Render("Review changes"),
		"Save changes to " + base + "?",
		"Config: " + m.config.Path(),
	}
	if m.backupCount > 0 {
		backupPath := filepath.Join(filepath.Dir(m.config.Path()), base+".backup.YYYYMMDD-HHMMSS")
		parts = append(parts,
			"Backup: "+backupPath,
			"Retention: keep "+strconv.Itoa(m.backupCount)+" backups",
		)
	} else {
		parts = append(parts, WarningStyle.Render("⚠ Backup: disabled (retention count is 0)"))
	}
	if m.saveError != "" {
		parts = append(parts, ErrorStyle.Render("✗ "+m.saveError))
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
		if ch.OldModel != ch.NewModel || (ch.OldVariant == "" && ch.NewVariant == "") {
			fmt.Fprintf(&b, "  %s.model: %s -> %s\n",
				ch.Target, formatModel(ch.OldModel), formatModel(ch.NewModel))
		}
		if ch.OldVariant != "" || ch.NewVariant != "" {
			fmt.Fprintf(&b, "  %s.variant: %s -> %s\n",
				ch.Target, formatModel(ch.OldVariant), formatModel(ch.NewVariant))
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func saveReviewViewportHeight(m Model) int {
	if m.height <= 0 || m.config == nil {
		return 0
	}
	textAreaWidth := max(1, m.width-OverlayBoxStyle.GetHorizontalFrameSize())
	headerH := lipgloss.Height(OverlayBoxStyle.Width(textAreaWidth).Render(clipLines(saveReviewHeader(m), textAreaWidth))) - OverlayBoxStyle.GetVerticalFrameSize()
	footerH := lipgloss.Height(OverlayBoxStyle.Width(textAreaWidth).Render(clipLines(saveReviewHelp(m.width), textAreaWidth))) - OverlayBoxStyle.GetVerticalFrameSize()
	fixed := headerH + footerH + OverlayBoxStyle.GetVerticalFrameSize()
	return max(1, m.height-fixed)
}

func syncSaveViewport(m *Model) {
	if m.width <= 0 || m.height <= 0 || m.config == nil {
		return
	}
	textAreaWidth := max(1, m.width-OverlayBoxStyle.GetHorizontalFrameSize())
	m.saveViewport.SetWidth(textAreaWidth)
	m.saveViewport.SetHeight(saveReviewViewportHeight(*m))
	offset := m.saveViewport.YOffset()
	m.saveViewport.SetContent(clipLines(saveReviewContent(*m), textAreaWidth))
	m.saveViewport.SetYOffset(offset)
}

func formatModel(model string) string {
	if model == "" {
		return "(none)"
	}
	return model
}

// updateSaveConfirm handles key presses on the Save Confirm screen.
//
// Keys:
//   - ENTER / 'y': confirm save → performSave()
//   - ESC / 'n':   cancel → return to immutable origin without saving
//
// Spec: REQ-TUI-007 — interaction.
func updateSaveConfirm(m Model, msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	// --- ENTER or 'y': confirm save ---
	case keyMsg.Code == tea.KeyEnter || keyMsg.Text == "y" || keyMsg.Text == "Y":
		return performSave(m)

	// --- ESC or 'n': cancel ---
	case keyMsg.Code == tea.KeyEsc || keyMsg.Text == "n" || keyMsg.Text == "N":
		m.popScreen()
		return m, nil

	// --- Scroll the diff while keeping modal actions fixed ---
	case keyMsg.Code == tea.KeyUp || keyMsg.Code == tea.KeyDown ||
		keyMsg.Code == tea.KeyPgUp || keyMsg.Code == tea.KeyPgDown:
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
	m.bulkTargets = nil
	m.fieldEditing = ""
	m.saveError = ""
	m.saveSuccess = true
	m.state = ScreenAgentList
	m.navigationStack = nil

	// Recompute the mdOnlyAgents cache so the [MD] badge stays live after a
	// save: once a JSON override exists for a previously md-only agent, the
	// merge layer unsets MdOnly and the badge should disappear. This mirrors
	// the NewModel initialization path.
	m.mdOnlyAgents = computeMdOnly(m.config)
	return m, nil
}
