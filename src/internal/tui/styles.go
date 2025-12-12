// Package tui provides styled console output using lipgloss for rich terminal UI.
// This package is only imported by the main dtvem CLI, not the shim executable,
// to keep the shim binary size minimal.
package tui

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Lazy initialization to avoid cold start penalty from lipgloss terminal detection
var (
	initOnce sync.Once

	// Colors
	colorPrimary   lipgloss.Color
	colorSecondary lipgloss.Color
	colorSuccess   lipgloss.Color
	colorWarning   lipgloss.Color
	colorError     lipgloss.Color
	colorMuted     lipgloss.Color

	// Text styles
	StyleTitle            lipgloss.Style
	StyleSubtitle         lipgloss.Style
	StyleVersion          lipgloss.Style
	StyleActiveVersion    lipgloss.Style
	StyleRuntime          lipgloss.Style
	StyleMuted            lipgloss.Style
	StyleIndicator        lipgloss.Style
	StyleWarningIndicator lipgloss.Style

	// Box styles
	StyleBox        lipgloss.Style
	StyleInfoBox    lipgloss.Style
	StyleSuccessBox lipgloss.Style
	StyleErrorBox   lipgloss.Style

	// Table styles
	StyleTableHeader    lipgloss.Style
	StyleTableCell      lipgloss.Style
	StyleTableRowActive lipgloss.Style
	StyleTableBorder    lipgloss.Style

	// Indicator strings
	CheckMark string
	CrossMark string
	Bullet    string
	Arrow     string
)

// initStyles initializes all lipgloss styles lazily
func initStyles() {
	initOnce.Do(func() {
		// Force TrueColor profile to skip slow terminal capability detection
		// See: https://github.com/charmbracelet/lipgloss/issues/86
		lipgloss.SetColorProfile(termenv.TrueColor)

		// Color palette
		colorPrimary = lipgloss.Color("39")    // Cyan
		colorSecondary = lipgloss.Color("213") // Magenta/Pink
		colorSuccess = lipgloss.Color("42")    // Green
		colorWarning = lipgloss.Color("214")   // Orange/Yellow
		colorError = lipgloss.Color("196")     // Red
		colorMuted = lipgloss.Color("245")     // Gray

		// Text styles
		StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

		StyleSubtitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

		StyleVersion = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

		StyleActiveVersion = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess)

		StyleRuntime = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

		StyleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

		StyleIndicator = lipgloss.NewStyle().
			Foreground(colorSuccess)

		StyleWarningIndicator = lipgloss.NewStyle().
			Foreground(colorWarning)

		// Box styles
		StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

		StyleInfoBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

		StyleSuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSuccess).
			Padding(0, 1)

		StyleErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorError).
			Padding(0, 1)

		// Table styles
		StyleTableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			PaddingRight(2)

		StyleTableCell = lipgloss.NewStyle().
			PaddingRight(2)

		StyleTableRowActive = lipgloss.NewStyle().
			Foreground(colorSuccess)

		StyleTableBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

		// Indicator constants with styles applied
		CheckMark = StyleIndicator.Render("✓")
		CrossMark = lipgloss.NewStyle().Foreground(colorError).Render("✗")
		Bullet = StyleMuted.Render("•")
		Arrow = lipgloss.NewStyle().Foreground(colorPrimary).Render("→")
	})
}

// Init ensures styles are initialized. Call this before using any styles.
func Init() {
	initStyles()
}

// RenderTitle renders a styled title
func RenderTitle(text string) string {
	initStyles()
	return StyleTitle.Render(text)
}

// RenderRuntime renders a runtime name with styling
func RenderRuntime(name string) string {
	initStyles()
	return StyleRuntime.Render(name)
}

// RenderVersion renders a version string with styling
func RenderVersion(version string) string {
	initStyles()
	return StyleVersion.Render(version)
}

// RenderActiveVersion renders an active version string with styling
func RenderActiveVersion(version string) string {
	initStyles()
	return StyleActiveVersion.Render(version)
}

// RenderMuted renders text in a muted/dim style
func RenderMuted(text string) string {
	initStyles()
	return StyleMuted.Render(text)
}

// RenderBox renders content in a rounded box
func RenderBox(content string) string {
	initStyles()
	return StyleBox.Render(content)
}

// RenderInfoBox renders content in an info-styled box
func RenderInfoBox(content string) string {
	initStyles()
	return StyleInfoBox.Render(content)
}

// GetCheckMark returns the styled checkmark indicator
func GetCheckMark() string {
	initStyles()
	return CheckMark
}

// GetCrossMark returns the styled cross indicator
func GetCrossMark() string {
	initStyles()
	return CrossMark
}
