package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ExtractTextFromLocation(loc protocol.Location) (string, error) {
	path := strings.TrimPrefix(string(loc.URI), "file://")

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
	// Calculate padding width based on the number of digits in the last line number
	lastLineNum := startLine + len(lines)
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

func doQuerySymbol(ctx context.Context, client *lsp.Client, symbolName string) ([]protocol.WorkspaceSymbolResult, error) {

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %v", err)
	}
	return results, nil
}

func QuerySymbol(ctx context.Context, client *lsp.Client, symbolName string) (string, []protocol.WorkspaceSymbolResult, error) {
	results, err := doQuerySymbol(ctx, client, symbolName)

	// clangd doesn't resolve "struct foo", only "foo"
	if len(results) == 0 && strings.HasPrefix(symbolName, "struct ") {
		results, err = doQuerySymbol(ctx, client, symbolName[7:])
		if len(results) > 0 {
			symbolName = symbolName[7:]
		}
	}

	return symbolName, results, err
}
