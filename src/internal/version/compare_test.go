package version

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int // sign only
	}{
		{"3.12.10", "3.12.9", 1},
		{"3.12.9", "3.12.10", -1},
		{"3.12.10", "3.12.10", 0},
		{"v22.0.0", "22.0.0", 0},
		{"4.0.0-preview2", "4.0.0", -1},
	}
	for _, tt := range tests {
		got := Compare(tt.a, tt.b)
		if (got > 0) != (tt.want > 0) || (got < 0) != (tt.want < 0) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMajorMinor(t *testing.T) {
	for in, want := range map[string]string{
		"3.12.10": "3.12",
		"v22.1.0": "22.1",
		"3.12":    "3.12",
		"3":       "3",
	} {
		if got := MajorMinor(in); got != want {
			t.Errorf("MajorMinor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNearest(t *testing.T) {
	signed := []string{"3.12.0", "3.12.9", "3.12.10", "3.13.9", "3.9.13"}
	tests := []struct {
		target, want string
	}{
		{"3.12.12", "3.12.10"}, // a security-only release: the last binary release below it
		{"3.12.10", "3.12.10"}, // present
		{"3.12.5", "3.12.0"},   // a gap: the highest below
		{"3.13.0", "3.13.9"},   // nothing below on the line: the lowest above
		{"3.11.4", ""},         // nothing on the line
		{"3.1.0", ""},          // "3.1" is not "3.12" or "3.13"
	}
	for _, tt := range tests {
		if got := Nearest(tt.target, signed); got != tt.want {
			t.Errorf("Nearest(%q) = %q, want %q", tt.target, got, tt.want)
		}
	}
	if got := Nearest("3.12.12", nil); got != "" {
		t.Errorf("Nearest(no candidates) = %q, want \"\"", got)
	}
}
