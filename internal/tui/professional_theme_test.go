package tui

import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/lleontor705/opencode-model-selector/internal/appname"
)

// hexColor renders a color.Color as an uppercase #RRGGBB string so style
// assertions can pin exact Catppuccin Mocha palette values regardless of the
// concrete lipgloss color implementation.
func hexColor(t *testing.T, c color.Color) string {
	t.Helper()
	if c == nil {
		return "<nil>"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

// TestCatppuccinChromeStyles_UseExactMochaRoles pins the shared style tokens
// to the exact Catppuccin Mocha roles required by design.md (REQ-TUI-PRO-002).
func TestCatppuccinChromeStyles_UseExactMochaRoles(t *testing.T) {
	tests := []struct {
		name  string
		style lipgloss.Style
		wantF string // expected foreground hex, "" skips the check
		wantB string // expected background hex, "" skips the check
	}{
		{"TitleStyle", TitleStyle, "#CDD6F4", "#CBA6F7"},
		{"SelectedStyle", SelectedStyle, "#CDD6F4", "#CBA6F7"},
		{"SelectedPrefix", SelectedPrefix, "#CBA6F7", ""},
		{"SectionHeader", SectionHeader, "#89B4FA", ""},
		{"AgentNormal", AgentNormal, "#CDD6F4", ""},
		{"AgentDisabled", AgentDisabled, "#A6ADC8", ""},
		{"FieldValue", FieldValue, "#CDD6F4", ""},
		{"DirtyIndicator", DirtyIndicator, "#F9E2AF", ""},
		{"ErrorStyle", ErrorStyle, "#F38BA8", ""},
		{"SuccessStyle", SuccessStyle, "#A6E3A1", ""},
		{"HelpStyle", HelpStyle, "#A6ADC8", ""},
		{"StatusBarStyle", StatusBarStyle, "#CDD6F4", "#181825"},
		{"StatusBarKey", StatusBarKey, "#94E2D5", ""},
		{"StatusBarDirty", StatusBarDirty, "#F9E2AF", ""},
		{"CurrentBadge", CurrentBadge, "#CDD6F4", "#A6E3A1"},
		{"SearchLabel", SearchLabel, "#94E2D5", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantF != "" {
				assert.Equal(t, tt.wantF, hexColor(t, tt.style.GetForeground()), "foreground must be the exact Mocha role")
			}
			if tt.wantB != "" {
				assert.Equal(t, tt.wantB, hexColor(t, tt.style.GetBackground()), "background must be the exact Mocha role")
			}
		})
	}
}

// TestSharedChrome_HeaderLabelsSurviveAnsiStrip proves the header carries its
// meaning through color-independent text: app name, version, tagline, screen
// title marker, and the "*" dirty marker (REQ-TUI-PRO-003).
func TestSharedChrome_HeaderLabelsSurviveAnsiStrip(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.width = 80
	m.dirty = true

	stripped := ansi.Strip(renderHeader(m, "Agents"))
	assert.Contains(t, stripped, appname.Name)
	assert.Contains(t, stripped, appVersion)
	assert.Contains(t, stripped, appTagline)
	assert.Contains(t, stripped, "› Agents")
	assert.Contains(t, stripped, "*", "dirty state must be carried by the text marker, not color")

	m.dirty = false
	stripped = ansi.Strip(renderHeader(m, "Agents"))
	assert.NotContains(t, stripped, "*", "clean state must not show the dirty marker")
	assert.Contains(t, stripped, appname.Name)
}

// TestSharedChrome_StatusBarCarriesColorIndependentState proves the status
// bar state (screen name, model count, dirty, help hint) survives ANSI
// stripping so color is never the sole meaning carrier.
func TestSharedChrome_StatusBarCarriesColorIndependentState(t *testing.T) {
	m := NewModel(fixtureConfig(t), sampleGrouped(), 5)
	m.width = 80
	m.dirty = true

	stripped := ansi.Strip(renderStatusBar(m, "Agents", 7))
	assert.Contains(t, stripped, "Agents")
	assert.Contains(t, stripped, "7 models")
	assert.Contains(t, stripped, "* unsaved")
	assert.Contains(t, stripped, "?")
	assert.Contains(t, stripped, "for help")

	single := ansi.Strip(renderStatusBar(m, "Agents", 1))
	assert.Contains(t, single, "1 model", "singular form must be preserved")

	m.dirty = false
	stripped = ansi.Strip(renderStatusBar(m, "Agents", 0))
	assert.NotContains(t, stripped, "unsaved")
	assert.NotContains(t, stripped, "models", "zero count must not render a model clause")
}

// TestCatppuccinPaletteTokens_AreExactMochaRoles pins every palette token to
// the exact hex value mandated by design.md; the closed-set palette decision
// is intentionally coupled to this test.
func TestCatppuccinPaletteTokens_AreExactMochaRoles(t *testing.T) {
	tests := []struct{ name, got, want string }{
		{"base", mochaBase, "#1E1E2E"},
		{"mantle", mochaMantle, "#181825"},
		{"surface0", mochaSurface0, "#313244"},
		{"surface1", mochaSurface1, "#45475A"},
		{"text", mochaText, "#CDD6F4"},
		{"subtext0", mochaSubtext0, "#A6ADC8"},
		{"mauve", mochaMauve, "#CBA6F7"},
		{"blue", mochaBlue, "#89B4FA"},
		{"teal", mochaTeal, "#94E2D5"},
		{"green", mochaGreen, "#A6E3A1"},
		{"yellow", mochaYellow, "#F9E2AF"},
		{"peach", mochaPeach, "#FAB387"},
		{"red", mochaRed, "#F38BA8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got, "Mocha role %s must be exact", tt.name)
		})
	}
}

// TestOverlayChrome_WarningFeedbackKeepsMarkerWithoutColor proves warning
// feedback uses the peach role while remaining legible without color via the
// "⚠" text marker (REQ-TUI-PRO-003).
func TestOverlayChrome_WarningFeedbackKeepsMarkerWithoutColor(t *testing.T) {
	assert.Equal(t, "#FAB387", hexColor(t, WarningStyle.GetForeground()))
	stripped := ansi.Strip(WarningStyle.Render("⚠ catalog unavailable — showing configured models"))
	assert.Contains(t, stripped, "⚠ catalog unavailable")
}

// TestOverlayChrome_OverlayBoxPreservesLabelText proves the shared overlay
// treatment keeps the wrapped message fully legible after ANSI stripping.
func TestOverlayChrome_OverlayBoxPreservesLabelText(t *testing.T) {
	message := "⚠ You have unsaved changes. Quit anyway? (y/n)"
	stripped := ansi.Strip(OverlayBoxStyle.Render(message))
	assert.Contains(t, stripped, message)
	assert.Contains(t, stripped, "╭", "overlay must be a bordered treatment")
}

// TestTerminalTooSmall_ClipsActionableMessageToTerminal proves the shared
// too-small overlay keeps its actionable message and respects the terminal
// bounds at narrow widths and tiny heights.
func TestTerminalTooSmall_ClipsActionableMessageToTerminal(t *testing.T) {
	out := renderTerminalTooSmall(30, 8)
	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "Terminal too small")
	assert.Contains(t, stripped, "Need at least", "actionable guidance must survive clipping")
	assertRenderedWidthAtMost(t, out, 30)

	full := renderTerminalTooSmall(40, 8)
	stripped = ansi.Strip(full)
	assert.Contains(t, stripped, fmt.Sprintf("Need at least %dx%d", minTerminalWidth, minTerminalHeight))
	assert.Contains(t, stripped, "40x8")
	assertRenderedWidthAtMost(t, full, 40)

	tiny := renderTerminalTooSmall(40, 1)
	assert.LessOrEqual(t, renderedLineCount(tiny), 1, "height clipping must bound the overlay")
	assert.Contains(t, ansi.Strip(tiny), "Terminal too small")
}
