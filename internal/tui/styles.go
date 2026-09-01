// Package tui implements the Bubbletea terminal UI for the opencode model
// selector. This file declares the Catppuccin Mocha palette and the lipgloss
// styles shared by every screen.
//
// Styles are package-level variables so that any screen-rendering function
// added by subsequent tasks can compose or override them without re-declaring
// color constants.
//
// The palette is the exact Catppuccin Mocha role set (design.md, REQ-TUI-PRO-002):
// mauve is the primary accent, blue and teal carry secondary/informational
// accents, green/yellow/peach/red carry success/dirty/warning/error states,
// and the base/mantle/surface ladder anchors backgrounds, chips, and borders.
// Color is never the sole meaning carrier: textual markers ("*", "▶", "★",
// "[MD]", "[H]", "[DISABLED]", "◆") stay visible after ANSI stripping.
package tui

import "charm.land/lipgloss/v2"

// Catppuccin Mocha palette — exact hex roles, centralized so palette tweaks
// happen in one place. Each role lists its semantic consumers.
const (
	mochaBase     = "#1E1E2E" // base — default app background
	mochaMantle   = "#181825" // mantle — status bar background
	mochaSurface0 = "#313244" // surface0 — subtle fills (input wells, panels)
	mochaSurface1 = "#45475A" // surface1 — borders, chips, badges
	mochaText     = "#CDD6F4" // text — primary foreground
	mochaSubtext0 = "#A6ADC8" // subtext0 — muted foreground, help text
	mochaMauve    = "#CBA6F7" // mauve — primary accent (title, selection)
	mochaBlue     = "#89B4FA" // blue — secondary accent (section headers)
	mochaTeal     = "#94E2D5" // teal — informational accent (search, status keys)
	mochaGreen    = "#A6E3A1" // green — success, current model
	mochaYellow   = "#F9E2AF" // yellow — dirty / pending changes
	mochaPeach    = "#FAB387" // peach — warnings
	mochaRed      = "#F38BA8" // red — errors
)

// Shared style declarations. Each style targets a specific visual element of
// the TUI as specified in the design and the REQ-TUI-PRO-002 scenarios.
var (
	// TitleStyle renders the application title bar. Bold text on a mauve
	// background makes the title pop without overwhelming.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaMauve)).
			Padding(0, 1)

	// HeaderTaglineStyle renders the descriptive tagline shown beneath the
	// title bar. Subtext keeps it subordinate to the title.
	HeaderTaglineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(mochaSubtext0)).
				Italic(true)

	// HeaderBoxStyle wraps the title and tagline in a bordered box with
	// rounded corners. A surface border stays out of the way.
	HeaderBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(mochaSurface1)).
			Padding(0, 1)

	// SectionHeader renders section dividers ("Primary Agents", "Subagents",
	// provider names in the model picker). Bold blue stands out without
	// competing with the title.
	SectionHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaBlue)).
			MarginTop(1).
			MarginBottom(0)

	// AgentNormal renders a regular, selectable agent row.
	AgentNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaText))

	// AgentDisabled renders an unavailable agent row. The row is
	// dimmed and must NOT be focusable (REQ-TUI-002).
	AgentDisabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaSubtext0)).
			Faint(true).
			Strikethrough(true)

	// AgentHidden renders a hidden agent (hidden: true). The row IS
	// selectable and carries a visual indicator added by the renderer.
	AgentHidden = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaSubtext0)).
			Italic(true)

	// SelectedStyle renders the currently highlighted/cursor row. A bold
	// text-on-mauve combo is the standard "you are here" cue.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaMauve))

	// SelectedPrefix renders just the cursor marker (">") when an item is
	// selected. Used for inline highlighting of the cursor arrow.
	SelectedPrefix = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaMauve))

	// FieldLabel renders labels in model assignment rows.
	FieldLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaMauve))

	// FieldValue renders current model values.
	FieldValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaText))

	// DirtyIndicator renders the "*" marker shown next to the title or row
	// when unsaved changes exist. The text marker itself carries the meaning.
	DirtyIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaYellow)).
			Bold(true)

	// ErrorStyle renders error messages returned by save/validate failures.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaRed)).
			Bold(true)

	// WarningStyle renders warning feedback (e.g. catalog unavailable).
	// Distinct from errors so severity stays scannable; the leading "⚠"
	// marker keeps meaning color-independent.
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaPeach)).
			Bold(true)

	// SuccessStyle renders success messages (e.g. "Saved successfully").
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaGreen)).
			Bold(true)

	// HelpStyle renders the footer help text (keybindings).
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaSubtext0))

	// HelpKey renders a single key name inside a help line — bold mauve
	// so the eye locks onto "ENTER" or "ESC" quickly.
	HelpKey = lipgloss.NewStyle().
		Foreground(lipgloss.Color(mochaMauve)).
		Bold(true)

	// StatusBarStyle renders the persistent bottom status bar. The mantle
	// background anchors the bar visually without competing with content.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaMantle)).
			Padding(0, 1)

	// StatusBarKey renders the screen-name chip and the "? for help" hint
	// inside the status bar.
	StatusBarKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaTeal)).
			Bold(true)

	// StatusBarDirty renders the unsaved-changes indicator inside the status
	// bar. Kept visually distinct so it draws the eye.
	StatusBarDirty = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaYellow)).
			Bold(true)

	// ProviderBadge renders a provider name as a colored chip in the model
	// selection screen. The neutral surface tint keeps provider groups
	// scannable without stealing focus from the content.
	ProviderBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaSurface1)).
			Padding(0, 1)

	// CurrentBadge renders the "★ CURRENT" pill next to the active model in
	// the picker. Text-on-green for at-a-glance recognition.
	CurrentBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaGreen)).
			Bold(true).
			Padding(0, 1)

	// SavePrompt renders the "💾 Save changes..." title on the save
	// confirm screen. Text on mauve so the screen stands out as a modal.
	SavePrompt = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaText)).
			Background(lipgloss.Color(mochaMauve)).
			Padding(0, 1)

	// SaveConfirmYes renders the "[Y] Save" hint in green.
	SaveConfirmYes = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaGreen)).
			Bold(true)

	// SaveConfirmNo renders the "[N] Cancel" hint in red.
	SaveConfirmNo = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaRed)).
			Bold(true)

	// DiffSummary renders "N changes pending" in warning yellow so the
	// gravity is clear before the user confirms.
	DiffSummary = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mochaYellow)).
			Bold(true)

	// SearchLabel renders the "🔍 Search:" prefix in the model picker.
	SearchLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mochaTeal))

	// OverlayBoxStyle wraps transient overlay content (quit confirmation,
	// catalog warnings, save feedback) in a bordered, focused treatment
	// instead of appended ad-hoc warning lines. Screens compose it around
	// their own content; the message text stays color-independent.
	OverlayBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(mochaSurface1)).
			Padding(0, 1)
)
