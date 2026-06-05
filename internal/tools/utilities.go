package tools

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// uriToPath converts a DocumentUri to a filesystem path. It is a non-panicking
// wrapper around DocumentUri.Path(), which panics on non-"file://" URIs (e.g.
// the "jdt://" URIs jdtls returns for symbols inside JARs). Returning an error
// lets callers degrade gracefully instead of crashing the tool. Behavior for
// ordinary file:// URIs is identical to calling Path() directly, including the
// percent-decoding that makes os.ReadFile find paths with encoded characters.
func uriToPath(uri protocol.DocumentUri) (path string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("unsupported document URI %q: %v", uri, r)
		}
	}()
	// An empty URI yields ("", nil): Path() maps "" to "" without panicking,
	// and an absent URI is a benign no-op for callers, not an error.
	return uri.Path(), nil
}

func ExtractTextFromLocation(loc protocol.Location) (string, error) {
	// Use uriToPath rather than trimming the "file://" prefix: it
	// percent-decodes the path (e.g. "%20" -> space) so os.ReadFile finds
	// files whose paths contain encoded characters, consistent with the
	// definition and references tools, and degrades gracefully on non-file URIs.
	path, err := uriToPath(loc.URI)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
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

// extractLineText returns the full text of the given 0-indexed line in the
// file referenced by uri. It builds a single-line Location range (start and end
// on the same line) covering the whole line, which works for every line
// including the last one in the file. Returns an error if the line is out of
// range or the file cannot be read.
func extractLineText(uri protocol.DocumentUri, line uint32) (string, error) {
	path, err := uriToPath(uri)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	idx := int(line)
	if idx < 0 || idx >= len(lines) {
		return "", fmt.Errorf("line %d out of range (%d lines)", line, len(lines))
	}
	return lines[idx], nil
}

func containsPosition(r protocol.Range, p protocol.Position) bool {
	if r.Start.Line > p.Line || r.End.Line < p.Line {
		return false
	}
	if r.Start.Line == p.Line && r.Start.Character > p.Character {
		return false
	}
	if r.End.Line == p.Line && r.End.Character <= p.Character {
		return false
	}
	return true
}

// addLineNumbers adds line numbers to each line of text with proper padding, starting from startLine
func addLineNumbers(text string, startLine int) string {
	lines := strings.Split(text, "\n")
	// Calculate padding width based on the number of digits in the last line
	// number. The last line printed is startLine+len(lines)-1 (the loop below
	// prints startLine+i for i in [0, len(lines))), so subtract one here to
	// avoid over-padding by a digit at power-of-ten boundaries.
	lastLineNum := startLine + len(lines) - 1
	padding := len(strconv.Itoa(lastLineNum))

	var result strings.Builder
	for i, line := range lines {
		// Format line number with padding and separator
		lineNum := strconv.Itoa(startLine + i)
		linePadding := strings.Repeat(" ", padding-len(lineNum))
		result.WriteString(fmt.Sprintf("%s%s|%s\n", linePadding, lineNum, line))
	}
	return result.String()
}

// LineRange represents a continuous range of lines to display
type LineRange struct {
	Start int
	End   int
}

// ConvertLinesToRanges converts a set of lines to continuous ranges
func ConvertLinesToRanges(linesToShow map[int]bool, totalLines int) []LineRange {
	// Convert map to sorted slice
	lineNumbers := make([]int, 0, len(linesToShow))
	for line := range linesToShow {
		if line >= 0 && line < totalLines {
			lineNumbers = append(lineNumbers, line)
		}
	}
	sort.Ints(lineNumbers)

	// Group into ranges
	var ranges []LineRange
	if len(lineNumbers) == 0 {
		return ranges
	}

	currentRange := LineRange{Start: lineNumbers[0], End: lineNumbers[0]}

	for i := 1; i < len(lineNumbers); i++ {
		if lineNumbers[i] == currentRange.End+1 {
			// Extend current range
			currentRange.End = lineNumbers[i]
		} else {
			// Start new range
			ranges = append(ranges, currentRange)
			currentRange = LineRange{Start: lineNumbers[i], End: lineNumbers[i]}
		}
	}

	// Add the last range
	ranges = append(ranges, currentRange)
	return ranges
}

// FormatLinesWithRanges formats file content using line ranges
func FormatLinesWithRanges(lines []string, ranges []LineRange) string {
	if len(ranges) == 0 {
		return ""
	}

	var result strings.Builder
	lastEnd := -1

	for _, r := range ranges {
		// Add skipped lines indicator
		if lastEnd != -1 && r.Start > lastEnd+1 {
			// skipped := r.Start - lastEnd - 1
			result.WriteString("...\n")
		}

		// Extract lines for this range
		rangeLines := lines[r.Start : r.End+1]

		// Add line numbers using the existing function
		numberedText := addLineNumbers(strings.Join(rangeLines, "\n"), r.Start+1)
		result.WriteString(numberedText)

		lastEnd = r.End
	}

	return result.String()
}
