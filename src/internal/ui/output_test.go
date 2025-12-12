package ui

import (
	"strings"
	"testing"
)

func TestHighlight(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple text",
			input: "test",
		},
		{
			name:  "text with spaces",
			input: "hello world",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "special characters",
			input: "test@123!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Highlight(tt.input)

			// The result should contain the input text
			// Note: In test environments, colors may be disabled, so the result
			// might be identical to the input. We just verify it contains the text.
			if !strings.Contains(result, tt.input) && tt.input != "" {
				t.Errorf("Highlight(%q) result does not contain input text", tt.input)
			}

			// Empty string should return empty string
			if tt.input == "" && result != "" {
				t.Errorf("Highlight(%q) = %q, want empty string", tt.input, result)
			}

			// Verify the function returns something (even if colors are disabled)
			if tt.input != "" && result == "" {
				t.Errorf("Highlight(%q) returned empty string", tt.input)
			}
		})
	}
}

func TestHighlightVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{
			name:    "semantic version",
			version: "3.11.0",
		},
		{
			name:    "version with v prefix",
			version: "v18.16.0",
		},
		{
			name:    "empty string",
			version: "",
		},
		{
			name:    "prerelease version",
			version: "1.0.0-beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightVersion(tt.version)

			// The result should contain the version text
			// Note: In test environments, colors may be disabled
			if !strings.Contains(result, tt.version) && tt.version != "" {
				t.Errorf("HighlightVersion(%q) result does not contain version text", tt.version)
			}

			// Empty string should return empty string
			if tt.version == "" && result != "" {
				t.Errorf("HighlightVersion(%q) = %q, want empty string", tt.version, result)
			}

			// Verify the function returns something (even if colors are disabled)
			if tt.version != "" && result == "" {
				t.Errorf("HighlightVersion(%q) returned empty string", tt.version)
			}
		})
	}
}

func TestHighlight_VS_HighlightVersion(t *testing.T) {
	// Verify that both functions work correctly
	text := "3.11.0"
	highlighted := Highlight(text)
	highlightedVersion := HighlightVersion(text)

	// Both should contain the input
	if !strings.Contains(highlighted, text) {
		t.Error("Highlight() result does not contain input")
	}
	if !strings.Contains(highlightedVersion, text) {
		t.Error("HighlightVersion() result does not contain input")
	}

	// Both should return non-empty results for non-empty input
	if highlighted == "" {
		t.Error("Highlight() returned empty string for non-empty input")
	}
	if highlightedVersion == "" {
		t.Error("HighlightVersion() returned empty string for non-empty input")
	}

	// Note: In test environments, colors may be disabled, so we can't reliably
	// test that they produce different results. We just verify they both work.
}

func TestHighlight_Symbols(t *testing.T) {
	// Verify that symbols are defined
	if successSymbol == "" {
		t.Error("successSymbol should not be empty")
	}
	if errorSymbol == "" {
		t.Error("errorSymbol should not be empty")
	}
	if warningSymbol == "" {
		t.Error("warningSymbol should not be empty")
	}
	if infoSymbol == "" {
		t.Error("infoSymbol should not be empty")
	}
	if debugSymbol == "" {
		t.Error("debugSymbol should not be empty")
	}
}

func TestVerboseMode(t *testing.T) {
	// Test that verbose mode can be toggled
	// First ensure verbose mode is off
	SetVerbose(false)
	if IsVerbose() {
		t.Error("Verbose mode should be off after SetVerbose(false)")
	}

	// Enable verbose mode
	SetVerbose(true)
	if !IsVerbose() {
		t.Error("Verbose mode should be on after SetVerbose(true)")
	}

	// Disable verbose mode again
	SetVerbose(false)
	if IsVerbose() {
		t.Error("Verbose mode should be off after SetVerbose(false)")
	}
}

func TestCheckVerboseEnv(t *testing.T) {
	// Save original state
	originalVerbose := verboseMode

	// Test with DTVEM_VERBOSE=1
	SetVerbose(false)
	t.Setenv("DTVEM_VERBOSE", "1")
	CheckVerboseEnv()
	if !IsVerbose() {
		t.Error("Verbose mode should be on when DTVEM_VERBOSE=1")
	}

	// Test with DTVEM_VERBOSE=true
	SetVerbose(false)
	t.Setenv("DTVEM_VERBOSE", "true")
	CheckVerboseEnv()
	if !IsVerbose() {
		t.Error("Verbose mode should be on when DTVEM_VERBOSE=true")
	}

	// Test with DTVEM_VERBOSE=false (should not enable)
	SetVerbose(false)
	t.Setenv("DTVEM_VERBOSE", "false")
	CheckVerboseEnv()
	if IsVerbose() {
		t.Error("Verbose mode should remain off when DTVEM_VERBOSE=false")
	}

	// Test with DTVEM_VERBOSE unset
	SetVerbose(false)
	t.Setenv("DTVEM_VERBOSE", "")
	CheckVerboseEnv()
	if IsVerbose() {
		t.Error("Verbose mode should remain off when DTVEM_VERBOSE is empty")
	}

	// Restore original state
	verboseMode = originalVerbose
}

func TestDebugOutput(t *testing.T) {
	// Save original state
	originalVerbose := verboseMode

	// Debug should not output anything when verbose is off
	SetVerbose(false)
	// We can't easily capture output in this test framework,
	// but we can at least verify the function doesn't panic
	Debug("test message %s", "arg")
	Debugf("test message %s", "arg")

	// Enable verbose and verify Debug runs without panic
	SetVerbose(true)
	Debug("test message %s", "arg")
	Debugf("test message %s", "arg")

	// Restore original state
	verboseMode = originalVerbose
}
