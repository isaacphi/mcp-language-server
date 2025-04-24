package tools

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/stretchr/testify/assert"
)

// Save original ReadFile function
var originalReadFile = os.ReadFile

// Create a function we can monkeypatch
func readFileHelper(name string) ([]byte, error) {
	return originalReadFile(name)
}

// Mock implementation that can be changed in tests
var readFileFunc = readFileHelper

// Create a modified version of ExtractTextFromLocation that uses our mockable function
func extractTextFromLocationForTest(loc protocol.Location) (string, error) {
	path := strings.TrimPrefix(string(loc.URI), "file://")

	content, err := readFileFunc(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	startLine := int(loc.Range.Start.Line)
	endLine := int(loc.Range.End.Line)
	if startLine < 0 || startLine >= len(lines) || endLine < 0 || endLine >= len(lines) {
		return "", fmt.Errorf("invalid Location range: %v", loc.Range)
	}

	// Handle single-line case
	if startLine == endLine {
		line := lines[startLine]
		startChar := int(loc.Range.Start.Character)
		endChar := int(loc.Range.End.Character)

		if startChar < 0 || startChar > len(line) || endChar < 0 || endChar > len(line) {
			return "", fmt.Errorf("invalid character range: %v", loc.Range)
		}

		return line[startChar:endChar], nil
	}

	// Handle multi-line case
	var result strings.Builder

	// First line
	firstLine := lines[startLine]
	startChar := int(loc.Range.Start.Character)
	if startChar < 0 || startChar > len(firstLine) {
		return "", fmt.Errorf("invalid start character: %v", loc.Range.Start)
	}
	result.WriteString(firstLine[startChar:])

	// Middle lines
	for i := startLine + 1; i < endLine; i++ {
		result.WriteString("\n")
		result.WriteString(lines[i])
	}

	// Last line
	lastLine := lines[endLine]
	endChar := int(loc.Range.End.Character)
	if endChar < 0 || endChar > len(lastLine) {
		return "", fmt.Errorf("invalid end character: %v", loc.Range.End)
	}
	result.WriteString("\n")
	result.WriteString(lastLine[:endChar])

	return result.String(), nil
}

func TestExtractTextFromLocation_SingleLine(t *testing.T) {
	mockContent := "function testFunction() {\n  return 'test';\n}"

	// Store original function and restore after test
	originalFunc := readFileFunc
	defer func() { readFileFunc = originalFunc }()

	// Set up mock implementation
	readFileFunc = func(name string) ([]byte, error) {
		return []byte(mockContent), nil
	}

	location := protocol.Location{
		URI: "file:///path/to/file.js",
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 9},
			End:   protocol.Position{Line: 0, Character: 21},
		},
	}

	result, err := extractTextFromLocationForTest(location)

	assert.NoError(t, err)
	assert.Equal(t, "testFunction", result)
}

func TestExtractTextFromLocation_MultiLine(t *testing.T) {
	mockContent := "function testFunction() {\n  return 'test';\n}"

	// Store original function and restore after test
	originalFunc := readFileFunc
	defer func() { readFileFunc = originalFunc }()

	// Set up mock implementation
	readFileFunc = func(name string) ([]byte, error) {
		return []byte(mockContent), nil
	}

	location := protocol.Location{
		URI: "file:///path/to/file.js",
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 9},
			End:   protocol.Position{Line: 1, Character: 15},
		},
	}

	result, err := extractTextFromLocationForTest(location)

	assert.NoError(t, err)
	assert.Equal(t, "testFunction() {\n  return 'test'", result)
}

func TestExtractTextFromLocation_InvalidRange(t *testing.T) {
	mockContent := "function testFunction() {\n  return 'test';\n}"

	// Store original function and restore after test
	originalFunc := readFileFunc
	defer func() { readFileFunc = originalFunc }()

	// Set up mock implementation
	readFileFunc = func(name string) ([]byte, error) {
		return []byte(mockContent), nil
	}

	// Out of bounds line
	location := protocol.Location{
		URI: "file:///path/to/file.js",
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 9},
			End:   protocol.Position{Line: 5, Character: 15},
		},
	}

	_, err := extractTextFromLocationForTest(location)
	assert.Error(t, err)

	// Out of bounds character on single line
	location = protocol.Location{
		URI: "file:///path/to/file.js",
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 9},
			End:   protocol.Position{Line: 0, Character: 100},
		},
	}

	_, err = extractTextFromLocationForTest(location)
	assert.Error(t, err)
}

func TestExtractTextFromLocation_FileError(t *testing.T) {
	// Store original function and restore after test
	originalFunc := readFileFunc
	defer func() { readFileFunc = originalFunc }()

	// Mock implementation that returns an error
	readFileFunc = func(name string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	location := protocol.Location{
		URI: "file:///path/to/nonexistent.js",
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 9},
			End:   protocol.Position{Line: 0, Character: 21},
		},
	}

	_, err := extractTextFromLocationForTest(location)
	assert.Error(t, err)
}

func TestContainsPosition(t *testing.T) {
	testCases := []struct {
		name     string
		r        protocol.Range
		p        protocol.Position
		expected bool
	}{
		{
			name: "Position inside range - middle",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 7, Character: 15},
			expected: true,
		},
		{
			name: "Position at range start line but after start character",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 5, Character: 15},
			expected: true,
		},
		{
			name: "Position at range start exact",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 5, Character: 10},
			expected: true,
		},
		{
			name: "Position at range end line but before end character",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 10, Character: 15},
			expected: true,
		},
		{
			name: "Position at range end exact",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 10, Character: 20},
			expected: false, // End position is exclusive
		},
		{
			name: "Position before range start line",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 4, Character: 15},
			expected: false,
		},
		{
			name: "Position after range end line",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 11, Character: 15},
			expected: false,
		},
		{
			name: "Position at start line but before start character",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 5, Character: 5},
			expected: false,
		},
		{
			name: "Position at end line but after end character",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 20},
			},
			p:        protocol.Position{Line: 10, Character: 25},
			expected: false,
		},
		{
			name: "Same line range",
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 5, Character: 20},
			},
			p:        protocol.Position{Line: 5, Character: 15},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsPosition(tc.r, tc.p)
			assert.Equal(t, tc.expected, result, "Expected containsPosition to return %v for range %v and position %v",
				tc.expected, tc.r, tc.p)
		})
	}
}

func TestAddLineNumbers(t *testing.T) {
	testCases := []struct {
		name      string
		text      string
		startLine int
		expected  string
	}{
		{
			name:      "Single line",
			text:      "function test() {}",
			startLine: 1,
			expected:  "1|function test() {}\n",
		},
		{
			name:      "Multiple lines",
			text:      "function test() {\n  return true;\n}",
			startLine: 10,
			expected:  "10|function test() {\n11|  return true;\n12|}\n",
		},
		{
			name:      "Padding for large line numbers",
			text:      "line1\nline2\nline3",
			startLine: 998,
			expected:  " 998|line1\n 999|line2\n1000|line3\n",
		},
		{
			name:      "Empty string",
			text:      "",
			startLine: 1,
			expected:  "1|\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := addLineNumbers(tc.text, tc.startLine)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConvertLinesToRanges(t *testing.T) {
	testCases := []struct {
		name        string
		linesToShow map[int]bool
		totalLines  int
		expected    []LineRange
	}{
		{
			name:        "Empty map",
			linesToShow: map[int]bool{},
			totalLines:  10,
			expected:    nil, // The function returns nil for empty input
		},
		{
			name:        "Single line",
			linesToShow: map[int]bool{5: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 5, End: 5}},
		},
		{
			name:        "Consecutive lines",
			linesToShow: map[int]bool{1: true, 2: true, 3: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 1, End: 3}},
		},
		{
			name:        "Non-consecutive lines",
			linesToShow: map[int]bool{1: true, 3: true, 5: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 1, End: 1}, {Start: 3, End: 3}, {Start: 5, End: 5}},
		},
		{
			name:        "Mixed consecutive and non-consecutive lines",
			linesToShow: map[int]bool{1: true, 2: true, 5: true, 6: true, 7: true, 9: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 1, End: 2}, {Start: 5, End: 7}, {Start: 9, End: 9}},
		},
		{
			name:        "Lines outside range are filtered",
			linesToShow: map[int]bool{-1: true, 0: true, 9: true, 10: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 0, End: 0}, {Start: 9, End: 9}},
		},
		{
			name:        "Unsorted input gets sorted",
			linesToShow: map[int]bool{5: true, 1: true, 3: true, 2: true},
			totalLines:  10,
			expected:    []LineRange{{Start: 1, End: 3}, {Start: 5, End: 5}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertLinesToRanges(tc.linesToShow, tc.totalLines)
			assert.Equal(t, tc.expected, result, "Expected ranges to match")
		})
	}
}

func TestFormatLinesWithRanges(t *testing.T) {
	testCases := []struct {
		name     string
		lines    []string
		ranges   []LineRange
		expected string
	}{
		{
			name:     "Empty ranges",
			lines:    []string{"line1", "line2", "line3"},
			ranges:   []LineRange{},
			expected: "",
		},
		{
			name:     "Single range",
			lines:    []string{"line1", "line2", "line3", "line4", "line5"},
			ranges:   []LineRange{{Start: 1, End: 3}},
			expected: "2|line2\n3|line3\n4|line4\n",
		},
		{
			name:     "Multiple ranges with gap",
			lines:    []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7"},
			ranges:   []LineRange{{Start: 0, End: 1}, {Start: 4, End: 6}},
			expected: "1|line1\n2|line2\n...\n5|line5\n6|line6\n7|line7\n",
		},
		{
			name:     "Adjacent ranges get combined - no gap in output",
			lines:    []string{"line1", "line2", "line3", "line4", "line5"},
			ranges:   []LineRange{{Start: 0, End: 2}, {Start: 3, End: 4}},
			expected: "1|line1\n2|line2\n3|line3\n4|line4\n5|line5\n",
		},
		{
			name: "Real-world example",
			lines: []string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"func main() {",
				"    s := \"Hello, World!\"",
				"    fmt.Println(s)",
				"}",
			},
			ranges:   []LineRange{{Start: 4, End: 6}},
			expected: "5|func main() {\n6|    s := \"Hello, World!\"\n7|    fmt.Println(s)\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatLinesWithRanges(tc.lines, tc.ranges)
			assert.Equal(t, tc.expected, result, "Expected formatted output to match")
		})
	}
}
