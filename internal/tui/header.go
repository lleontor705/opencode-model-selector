// Package tui — visual frame elements shared by every screen.
//
// This file contains renderHeader() and renderStatusBar(), the two visual
// anchors that appear at the top and bottom of every TUI screen. They were
// extracted from individual screens so that the banner design lives in ONE
// place — changing the header here updates every screen consistently.
//
// Layout (terminal, top to bottom):
//
//	╭─ opencode-model-selector ─────────────────────╮
//	│  Interactive model selector for OpenCode...   │
//	╰───────────────────────────────────────────────╯
//
//	<screen-specific content>
//
//	[ <screen name> │ N models │ * unsaved │ ? for help ]
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// App constants — surfaced here so the header, status bar, and any future
// "About" dialog share the same source of truth.
const (
	// appName is the public name of the tool, used in the header banner.
	appName = "opencode-model-selector"
	// appTagline is the one-line description under the title.
	appTagline = "Interactive model selector for OpenCode agents"
	// appVersion is the runtime version. It can be overridden at link time
	// via `-ldflags "-X github.com/lleontor705/opencode-model-selector/internal/tui.appVersion=v0.x.y"`
	// but defaults to a sensible dev tag so the banner always renders.
	appVersion = "v0.1.0"
	// headerBoxWidth is the target width of the rounded header box. Set
	// conservatively so the banner looks balanced in typical 80-120 column
	// terminals; on wider terminals the right padding absorbs the slack.
	headerBoxWidth = 60
)

// renderHeader renders the stylized top-of-screen banner. The banner is a
// rounded box with the app name as a chip-style title and the tagline below.
// The dirty marker ("*") is prepended when dirty=true so the user always
// knows whether there are unsaved changes at a glance, regardless of which
// screen they're on.
//
// The output does NOT include a trailing newline — callers add one if needed.
func renderHeader(m Model, screenTitle string) string {
	// Build the title chip: "opencode-model-selector" with optional dirty marker.
	// The dirty marker is the literal '*' so existing tests + muscle memory
	// continue to work; it is colored orange via DirtyIndicator style.
	var titleParts []string
	if m.dirty {
		titleParts = append(titleParts, DirtyIndicator.Render("*"))
	}
	titleParts = append(titleParts, TitleStyle.Render(appName), HeaderTaglineStyle.Render(appVersion))

	if screenTitle != "" {
		titleParts = append(titleParts,
			HeaderTaglineStyle.Render("› "+screenTitle),
		)
	}

	titleLine := lipgloss.JoinHorizontal(lipgloss.Left, titleParts...)

	// Build the tagline row.
	taglineLine := HeaderTaglineStyle.Render(appTagline)

	// Compose the inner content and wrap in the rounded box.
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, taglineLine)

	box := HeaderBoxStyle.
		Width(headerBoxWidth).
		Render(content)

	return box
}

// renderStatusBar renders the persistent bottom-of-screen status bar.
// Content (left → right):
//
//	[Screen Name] · N models · [● unsaved]                       ? for help
//
// The bar is one rendered line with the Help key on the right. The screen
// name and dirty indicator come from the Model; the model count is derived
// from the (possibly filtered) list visible to the user on the picker screen
// and the full flat list elsewhere.
func renderStatusBar(m Model, screenName string, modelCount int) string {
	var parts []string

	// --- Screen name (always shown) ---
	parts = append(parts, StatusBarKey.Render(screenName))

	// --- Model count (shown when there are models OR when on a model screen) ---
	if modelCount > 0 {
		parts = append(parts,
			fmt.Sprintf("%d model%s", modelCount, plural(modelCount)))
	}

	// --- Dirty indicator (shown only when there are unsaved changes) ---
	if m.dirty {
		parts = append(parts, StatusBarDirty.Render("* unsaved"))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Left, joinWithDot(parts)...)

	// --- Right side: help hint ---
	right := StatusBarStyle.Render(StatusBarKey.Render("?") +
		" " +
		HelpStyle.Render("for help"))

	// Fill the middle with spaces. Width is conservative — the bar collapses
	// gracefully on narrow terminals.
	const barWidth = 80
	gap := barWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return StatusBarStyle.Render(
		left +
			lipgloss.NewStyle().Width(gap).Render("") +
			right,
	)
}

// joinWithDot joins string parts with " · " separators — the visual glue used
// throughout the status bar.
func joinWithDot(parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	out := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			out = append(out, HelpStyle.Render("·"))
		}
		out = append(out, p)
	}
	return out
}

// helpItem is one entry in a keybinding help line: a key name (shown in bold
// primary color) and a short description (shown in muted color).
type helpItem struct {
	key  string
	desc string
}

// helpLine renders a single-line keybinding hint like:
//   [j/k] navigate  [ENTER] edit  [s] save  [q] quit
//
// The keys are colored with HelpKey and descriptions with HelpStyle; an
// em-space-equivalent gap separates each pair so the line is easy to scan.
func helpLine(items []helpItem) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts,
			HelpKey.Render(it.key)+
				" "+
				HelpStyle.Render(it.desc),
		)
	}
	// Use a single space as the inter-pair separator; the gap is enough to
	// visually separate key+desc pairs without crowding.
	return HelpStyle.Render("  ") + joinHelp(parts, HelpStyle.Render("  "))
}

// joinHelp joins string parts with a single separator (kept separate from
// joinWithDot to avoid coupling the dot separator with keybinding lines).
func joinHelp(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// plural returns "s" when n != 1, "" otherwise. Tiny helper to keep the
// status bar grammatically correct without importing a full pluralize lib.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

