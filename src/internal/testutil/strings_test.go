package testutil

import "testing"

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "hello world",
			substr:   "lo wo",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "hello",
			substr:   "hello world",
			expected: false,
		},
		{
			name:     "case sensitive - no match",
			s:        "hello",
			substr:   "HELLO",
			expected: false,
		},
		{
			name:     "path with version",
			s:        "/home/user/.dtvem/versions/python/3.11.0/bin",
			substr:   "python",
			expected: true,
		},
		{
			name:     "path with version number",
			s:        "/home/user/.dtvem/versions/python/3.11.0/bin",
			substr:   "3.11.0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("ContainsSubstring(%q, %q) = %v, want %v",
					tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
