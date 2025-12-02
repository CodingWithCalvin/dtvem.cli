package cmd

import (
	"reflect"
	"testing"
)

func TestParseSelection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxCount int
		expected []int
	}{
		{
			name:     "all lowercase",
			input:    "all",
			maxCount: 3,
			expected: []int{0, 1, 2},
		},
		{
			name:     "all uppercase",
			input:    "ALL",
			maxCount: 3,
			expected: []int{0, 1, 2},
		},
		{
			name:     "all mixed case",
			input:    "All",
			maxCount: 2,
			expected: []int{0, 1},
		},
		{
			name:     "single number",
			input:    "1",
			maxCount: 5,
			expected: []int{0},
		},
		{
			name:     "multiple numbers",
			input:    "1,3,5",
			maxCount: 5,
			expected: []int{0, 2, 4},
		},
		{
			name:     "numbers with spaces",
			input:    "1, 3, 5",
			maxCount: 5,
			expected: []int{0, 2, 4},
		},
		{
			name:     "numbers with extra spaces",
			input:    "  1  ,  3  ,  5  ",
			maxCount: 5,
			expected: []int{0, 2, 4},
		},
		{
			name:     "out of range numbers ignored",
			input:    "1,10,3",
			maxCount: 5,
			expected: []int{0, 2},
		},
		{
			name:     "negative numbers ignored",
			input:    "-1,2,3",
			maxCount: 5,
			expected: []int{1, 2},
		},
		{
			name:     "zero ignored",
			input:    "0,1,2",
			maxCount: 5,
			expected: []int{0, 1},
		},
		{
			name:     "invalid input",
			input:    "abc,def",
			maxCount: 5,
			expected: []int{},
		},
		{
			name:     "mixed valid and invalid",
			input:    "1,abc,3",
			maxCount: 5,
			expected: []int{0, 2},
		},
		{
			name:     "empty string",
			input:    "",
			maxCount: 3,
			expected: []int{},
		},
		{
			name:     "all with zero maxCount",
			input:    "all",
			maxCount: 0,
			expected: []int{},
		},
		{
			name:     "duplicate numbers",
			input:    "1,1,2,2,3",
			maxCount: 5,
			expected: []int{0, 0, 1, 1, 2},
		},
		{
			name:     "only commas",
			input:    ",,,",
			maxCount: 3,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSelection(tt.input, tt.maxCount)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseSelection(%q, %d) = %v, want %v",
					tt.input, tt.maxCount, result, tt.expected)
			}
		})
	}
}

func TestParseSelection_ReturnedIndices(t *testing.T) {
	// Verify that returned indices are 0-based
	result := parseSelection("1,2,3", 5)
	expected := []int{0, 1, 2}

	if len(result) != len(expected) {
		t.Fatalf("parseSelection() returned %d indices, want %d", len(result), len(expected))
	}

	for i, idx := range result {
		if idx != expected[i] {
			t.Errorf("parseSelection()[%d] = %d, want %d (indices should be 0-based)", i, idx, expected[i])
		}
	}
}

func TestParseSelection_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxCount int
		checkFn  func([]int) bool
		desc     string
	}{
		{
			name:     "maxCount boundary",
			input:    "5",
			maxCount: 5,
			checkFn:  func(result []int) bool { return len(result) == 1 && result[0] == 4 },
			desc:     "should accept number equal to maxCount",
		},
		{
			name:     "maxCount exceeded",
			input:    "6",
			maxCount: 5,
			checkFn:  func(result []int) bool { return len(result) == 0 },
			desc:     "should reject number greater than maxCount",
		},
		{
			name:     "large numbers",
			input:    "1,100,200",
			maxCount: 10,
			checkFn:  func(result []int) bool { return len(result) == 1 && result[0] == 0 },
			desc:     "should ignore numbers way out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSelection(tt.input, tt.maxCount)
			if !tt.checkFn(result) {
				t.Errorf("parseSelection(%q, %d): %s, got %v",
					tt.input, tt.maxCount, tt.desc, result)
			}
		})
	}
}
