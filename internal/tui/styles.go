// Package tui implements the Bubbletea terminal UI for the opencode model
// selector. This file declares the lipgloss styles shared by every screen
// (REQ-TUI-002, REQ-TUI-004, REQ-TUI-005, REQ-TUI-007).
//
// Styles are package-level variables so that any screen-rendering function
// added by subsequent tasks (agent_list.go, agent_detail.go, ...) can compose
// or override them without re-declaring color constants.
//
// The palette is intentionally cohesive (inspired by Tokyo Night / Catppuccin
// Mocha / Material 3) — purple as primary accent, cyan for sections, orange
// for dirty/pending, green for success, red for error. Soft backgrounds are
// used only on the title bar and status bar to anchor the visual hierarchy.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — keep all hex values centralized here so palette tweaks
// happen in one place. Values are tuned for both light and dark terminals.
const (
	colorPrimary    = "#7D56F4" // purple — title, accent, selected highlight
	colorPrimaryDim = "#5C3FBE" // darker purple for borders/badges
	colorSecondary  = "#5FB3B3" // cyan — section headers, decorative
	colorTertiary   = "#9ECE6A" // green — success, current model
	colorWarning    = "#FA8C16" // orange — dirty, in-progress, warnings
	colorError      = "#F7768E" // soft red — error states
	colorMuted      = "#565F89" // dim blue-grey — muted text, disabled
	colorSubtle     = "#414868" // very dim — borders, subtle dividers
	colorBright     = "#C0CAF5" // off-white — primary foreground
	colorFaint      = "#3B4261" // very dark — disabled items, faint backgrounds
)

// Shared style declarations. Each style targets a specific visual element of
// the TUI as specified in the design (#606, Section 2 Decision 5/6) and the
// REQ-TUI-00x scenarios.
var (
	// TitleStyle renders the application title bar. Bold purple text on a
	// subtle dark background makes the title pop without overwhelming.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorPrimary)).
			Padding(0, 1)

	// HeaderTaglineStyle renders the descriptive tagline shown beneath the
	// title bar. Muted color keeps it subordinate to the title.
	HeaderTaglineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted)).
				Italic(true)

	// HeaderBoxStyle wraps the title and tagline in a bordered box with
	// rounded corners. Uses a dim border to stay out of the way.
	HeaderBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorPrimaryDim)).
			Padding(0, 1)

	// SectionHeader renders section dividers ("Primary Agents", "Subagents",
	// provider names in the model picker). Cyan with bold makes them stand
	// out without competing with the title.
	SectionHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorSecondary)).
			MarginTop(1).
			MarginBottom(0)

	// AgentNormal renders a regular, selectable agent row.
	AgentNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBright))

	// AgentDisabled renders a disabled (disable: true) agent row. The row is
	// dimmed and must NOT be focusable (REQ-TUI-002).
	AgentDisabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Faint(true).
			Strikethrough(true)

	// AgentHidden renders a hidden agent (hidden: true). The row IS
	// selectable and carries a visual indicator added by the renderer.
	AgentHidden = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Italic(true)

	// SelectedStyle renders the currently highlighted/cursor row. A bold
	// white-on-purple combo is the standard "you are here" cue.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorPrimary))

	// SelectedPrefix renders just the cursor marker (">") when an item is
	// selected. Used for inline highlighting of the cursor arrow.
	SelectedPrefix = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPrimary))

	// FieldLabel renders the left-hand label on the Agent Detail screen
	// ("model:", "temperature:").
	FieldLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPrimary))

	// FieldValue renders the right-hand current value on the Agent Detail
	// screen. Off-white reads well on dark terminals.
	FieldValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBright))

	// FieldHint renders parenthetical hints like "(default: 0.7)" — dimmed
	// so the user knows they're optional guidance, not the active value.
	FieldHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Italic(true)

	// BoolEnabled renders "✓ enabled" with a green checkmark.
	BoolEnabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTertiary)).
			Bold(true)

	// BoolDisabled renders "✗ disabled" with a soft red X.
	BoolDisabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError))

	// DirtyIndicator renders the "*" marker shown next to the title or row
	// when unsaved changes exist.
	DirtyIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	// ErrorStyle renders error messages returned by save/validate failures.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true)

	// SuccessStyle renders success messages (e.g. "Saved successfully").
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTertiary)).
			Bold(true)

	// HelpStyle renders the footer help text (keybindings).
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))

	// HelpKey renders a single key name inside a help line — bold primary
	// so the eye locks onto "ENTER" or "ESC" quickly.
	HelpKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			Bold(true)

	// StatusBarStyle renders the persistent bottom status bar. A subtle dark
	// background anchors the bar visually without competing with content.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorFaint)).
			Padding(0, 1)

	// StatusBarKey renders the "? for help" hint inside the status bar.
	StatusBarKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSecondary)).
			Bold(true)

	// StatusBarDirty renders the unsaved-changes indicator inside the status
	// bar. Kept visually distinct so it draws the eye.
	StatusBarDirty = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	// ProviderBadge renders a provider name as a colored chip in the model
	// selection screen. The background tint is per-provider so the user can
	// scan provider groups quickly.
	ProviderBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorPrimaryDim)).
			Padding(0, 1)

	// CurrentBadge renders the "★ CURRENT" pill next to the active model in
	// the picker. Green-on-dark for at-a-glance recognition.
	CurrentBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorTertiary)).
			Bold(true).
			Padding(0, 1)

	// SavePrompt renders the "💾 Save changes..." title on the save
	// confirm screen. Bright purple so the screen stands out as a modal.
	SavePrompt = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorBright)).
			Background(lipgloss.Color(colorPrimary)).
			Padding(0, 1)

	// SaveConfirmYes renders the "[Y] Save" hint in green.
	SaveConfirmYes = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTertiary)).
			Bold(true)

	// SaveConfirmNo renders the "[N] Cancel" hint in red.
	SaveConfirmNo = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true)

	// DiffSummary renders "N changes pending" in warning orange so the
	// gravity is clear before the user confirms.
	DiffSummary = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	// SearchLabel renders the "🔍 Search:" prefix in the model picker.
	SearchLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorSecondary))
)
