package tools

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
