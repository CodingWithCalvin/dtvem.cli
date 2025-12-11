// Package ui provides colored console output utilities for user interfaces
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	// Color functions for different message types
	successColor  = color.New(color.FgGreen, color.Bold)
	errorColor    = color.New(color.FgRed, color.Bold)
	warningColor  = color.New(color.FgYellow, color.Bold)
	infoColor     = color.New(color.FgCyan)
	progressColor = color.New(color.FgBlue)

	// Symbols
	successSymbol = "✓"
	errorSymbol   = "✗"
	warningSymbol = "⚠"
	infoSymbol    = "→"
)

// Success prints a success message in green with a checkmark
func Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = successColor.Printf("%s %s\n", successSymbol, message)
}

// Error prints an error message in red with an X
func Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = errorColor.Printf("%s %s\n", errorSymbol, message)
}

// Warning prints a warning message in yellow with a warning symbol
func Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = warningColor.Printf("%s %s\n", warningSymbol, message)
}

// Info prints an info message in cyan with an arrow
func Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = infoColor.Printf("%s %s\n", infoSymbol, message)
}

// Progress prints a progress message in blue with an arrow
func Progress(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = progressColor.Printf("  %s %s\n", infoSymbol, message)
}

// Println prints a regular message without color
func Println(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Printf prints a regular message without color (no newline)
func Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Header prints a bold header message
func Header(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	bold := color.New(color.Bold)
	_, _ = bold.Println(message)
}

// Highlight prints text in a highlighted color (for emphasis)
func Highlight(text string) string {
	return color.New(color.FgCyan, color.Bold).Sprint(text)
}

// HighlightVersion prints a version string in a highlighted color
func HighlightVersion(version string) string {
	return color.New(color.FgMagenta, color.Bold).Sprint(version)
}

// ActiveVersion prints a version string in green (for currently active version)
func ActiveVersion(version string) string {
	return color.New(color.FgGreen, color.Bold).Sprint(version)
}

// DimText prints text in a dimmed/gray color (for inactive items)
func DimText(text string) string {
	return color.New(color.Faint).Sprint(text)
}

// PromptInstall prompts the user to install a missing version.
// Returns true if the user wants to install, false otherwise.
// Respects DTVEM_AUTO_INSTALL environment variable:
//   - "true": auto-install without prompting
//   - "false": never prompt, return false
//   - unset: prompt interactively
func PromptInstall(displayName, version string) bool {
	// Check if running in non-interactive mode (CI/automation)
	if os.Getenv("DTVEM_AUTO_INSTALL") == "false" {
		return false
	}

	// If DTVEM_AUTO_INSTALL=true, auto-install without prompting
	if os.Getenv("DTVEM_AUTO_INSTALL") == "true" {
		return true
	}

	// Interactive prompt
	Printf("Install %s %s now? [Y/n]: ", displayName, version)

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	// Default to "yes" if empty response
	return response == "" || response == "y" || response == "yes"
}

// MissingRuntime represents a runtime that needs to be installed
type MissingRuntime interface {
	DisplayName() string
	Version() string
}

// PromptInstallMissing prompts the user to install multiple missing versions.
// Returns true if the user wants to install, false otherwise.
// Respects DTVEM_AUTO_INSTALL environment variable.
func PromptInstallMissing[T any](missing []T) bool {
	if len(missing) == 0 {
		return false
	}

	// Check if running in non-interactive mode (CI/automation)
	if os.Getenv("DTVEM_AUTO_INSTALL") == "false" {
		return false
	}

	// If DTVEM_AUTO_INSTALL=true, auto-install without prompting
	if os.Getenv("DTVEM_AUTO_INSTALL") == "true" {
		return true
	}

	// Interactive prompt
	Printf("Install missing version(s)? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	// Default to "yes" if empty response
	return response == "" || response == "y" || response == "yes"
}
