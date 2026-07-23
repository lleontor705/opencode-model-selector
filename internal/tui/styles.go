// Package tui implements the Bubbletea terminal UI for the opencode model
// selector. This file declares the lipgloss styles shared by every screen
// (REQ-TUI-002, REQ-TUI-004, REQ-TUI-005, REQ-TUI-007).
//
// Styles are package-level variables so that any screen-rendering function
// added by subsequent tasks (agent_list.go, agent_detail.go, ...) can compose
// or override them without re-declaring color constants.
package tui

import "github.com/charmbracelet/lipgloss"

// Shared style declarations. Each style targets a specific visual element of
// the TUI as specified in the design (#606, Section 2 Decision 5/6) and the
// REQ-TUI-00x scenarios.
var (
	// TitleStyle renders the application title bar.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	// SectionHeader renders section dividers ("Primary Agents", "Subagents",
	// provider names in the model picker).
	SectionHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5C5C5C")).
			Underline(true)

	// AgentNormal renders a regular, selectable agent row.
	AgentNormal = lipgloss.NewStyle()

	// AgentDisabled renders a disabled (disable: true) agent row. The row is
	// dimmed and must NOT be focusable (REQ-TUI-002).
	AgentDisabled = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Faint(true)

	// AgentHidden renders a hidden agent (hidden: true). The row IS
	// selectable and carries a visual indicator added by the renderer.
	AgentHidden = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374"))

	// SelectedStyle renders the currently highlighted/cursor row.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4"))

	// FieldLabel renders the left-hand label on the Agent Detail screen
	// ("model:", "temperature:").
	FieldLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	// FieldValue renders the right-hand current value on the Agent Detail
	// screen.
	FieldValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// DirtyIndicator renders the "*" marker shown next to the title or row
	// when unsaved changes exist.
	DirtyIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FA8C16")).
			Bold(true)

	// ErrorStyle renders error messages returned by save/validate failures.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	// SuccessStyle renders success messages (e.g. "Saved successfully").
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	// HelpStyle renders the footer help text (keybindings).
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))
)
