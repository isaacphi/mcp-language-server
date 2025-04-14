package tools

import (
	"context"
	"fmt"
	"net/url"
	"os"
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
	if r.End.Line == p.Line && r.End.Character < p.Character {
		return false
	}
	return true
}

// Gets the full code block surrounding the start of the input location
func GetFullDefinition(ctx context.Context, client *lsp.Client, startLocation protocol.Location) (string, protocol.Location, error) {
	symParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: startLocation.URI,
		},
	}

	// Get all symbols in document
	symResult, err := client.DocumentSymbol(ctx, symParams)
	if err != nil {
		return "", protocol.Location{}, fmt.Errorf("failed to get document symbols: %w", err)
	}

	symbols, err := symResult.Results()
	if err != nil {
		return "", protocol.Location{}, fmt.Errorf("failed to process document symbols: %w", err)
	}

	var symbolRange protocol.Range
	found := false

	// Search for symbol at startLocation
	var searchSymbols func(symbols []protocol.DocumentSymbolResult) bool
	searchSymbols = func(symbols []protocol.DocumentSymbolResult) bool {
		for _, sym := range symbols {
			if containsPosition(sym.GetRange(), startLocation.Range.Start) {
				symbolRange = sym.GetRange()
				found = true
				return true
			}
			// Handle nested symbols if it's a DocumentSymbol
			if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
				childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
				for i := range ds.Children {
					childSymbols[i] = &ds.Children[i]
				}
				if searchSymbols(childSymbols) {
					return true
				}
			}
		}
		return false
	}

	searchSymbols(symbols)

	if !found {
		// Fall back to the original location if we can't find a better range
		symbolRange = startLocation.Range
	}

	if found {
		// Convert URI to filesystem path
		filePath, err := url.PathUnescape(strings.TrimPrefix(string(startLocation.URI), "file://"))
		if err != nil {
			return "", protocol.Location{}, fmt.Errorf("failed to unescape URI: %w", err)
		}

		// Read the file to get the full lines of the definition
		// because we may have a start and end column
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", protocol.Location{}, fmt.Errorf("failed to read file: %w", err)
		}

		lines := strings.Split(string(content), "\n")

		// Extend start to beginning of line
		symbolRange.Start.Character = 0

		// Get the line at the end of the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("line number out of range")
		}

		line := lines[symbolRange.End.Line]
		trimmedLine := strings.TrimSpace(line)

		// In some cases, constant definitions do not include the full body and instead
		// end with an opening bracket. In this case, parse the file until the closing bracket
		if len(trimmedLine) > 0 {
			lastChar := trimmedLine[len(trimmedLine)-1]
			if lastChar == '(' || lastChar == '[' || lastChar == '{' || lastChar == '<' {
				// Find matching closing bracket
				bracketStack := []rune{rune(lastChar)}
				lineNum := symbolRange.End.Line + 1

				for lineNum < uint32(len(lines)) {
					line := lines[lineNum]
					for pos, char := range line {
						if char == '(' || char == '[' || char == '{' || char == '<' {
							bracketStack = append(bracketStack, char)
						} else if char == ')' || char == ']' || char == '}' || char == '>' {
							if len(bracketStack) > 0 {
								lastOpen := bracketStack[len(bracketStack)-1]
								if (lastOpen == '(' && char == ')') ||
									(lastOpen == '[' && char == ']') ||
									(lastOpen == '{' && char == '}') ||
									(lastOpen == '<' && char == '>') {
									bracketStack = bracketStack[:len(bracketStack)-1]
									if len(bracketStack) == 0 {
										// Found matching bracket - update range
										symbolRange.End.Line = lineNum
										symbolRange.End.Character = uint32(pos + 1)
										goto foundClosing
									}
								}
							}
						}
					}
					lineNum++
				}
			foundClosing:
			}
		}

		// Update location with new range
		startLocation.Range = symbolRange

		// Return the text within the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("end line out of range")
		}

		selectedLines := lines[symbolRange.Start.Line : symbolRange.End.Line+1]
		return strings.Join(selectedLines, "\n"), startLocation, nil
	}

	return "", protocol.Location{}, fmt.Errorf("symbol not found")
}

// addLineNumbers adds line numbers to each line of text with proper padding, starting from startLine
// If highlightLines is provided, those line numbers (0-indexed relative to the start of the text) will be marked
func addLineNumbers(text string, startLine int, highlightLines ...int) string {
	lines := strings.Split(text, "\n")
	// Calculate padding width based on the number of digits in the last line number
	lastLineNum := startLine + len(lines) - 1
	padding := len(strconv.Itoa(lastLineNum))

	// Convert highlight lines to a map for efficient lookup
	highlights := make(map[int]bool)
	for _, line := range highlightLines {
		highlights[line] = true
	}

	var result strings.Builder
	for i, line := range lines {
		// Format line number with padding and separator
		lineNum := startLine + i
		lineNumStr := strconv.Itoa(lineNum)
		linePadding := strings.Repeat(" ", padding-len(lineNumStr))

		// Determine if this line should be highlighted
		marker := "|"
		if highlights[i] {
			marker = ">" // Use '>' to indicate highlighted lines
		}

		result.WriteString(fmt.Sprintf("%s%s%s %s\n", linePadding, lineNumStr, marker, line))
	}
	return result.String()
}

// GetDefinitionWithContext returns the text around a given position with configurable context,
// along with the location (Range) corresponding to that returned text.
// contextLines specifies how many lines before and after the reference line to include.
// loc is the location of the original reference point.
func GetDefinitionWithContext(ctx context.Context, client *lsp.Client /* Remove client if not used */, loc protocol.Location, contextLines int) (string, protocol.Location, error) {
	// Convert URI to filesystem path
	filePath, err := url.PathUnescape(strings.TrimPrefix(string(loc.URI), "file://"))
	if err != nil {
		return "", protocol.Location{}, fmt.Errorf("failed to unescape URI: %w", err)
	}

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		// Return zero location on error
		return "", protocol.Location{}, fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	// It's generally safer to handle different line endings
	// Replace CRLF with LF for consistent splitting
	normalizedContent := strings.ReplaceAll(string(content), "\r\n", "\n")
	fileLines := strings.Split(normalizedContent, "\n")

	// Calculate the range to show, ensuring we don't go out of bounds
	refLine := int(loc.Range.Start.Line) // The line where the reference occurs

	// Check if the reference line itself is valid
	if refLine < 0 || refLine >= len(fileLines) {
		return "", protocol.Location{}, fmt.Errorf("reference line %d is out of bounds for file %s (0-%d)", refLine+1, filePath, len(fileLines)-1)
	}

	startLine := refLine - contextLines
	if startLine < 0 {
		startLine = 0
	}

	endLine := refLine + contextLines
	if endLine >= len(fileLines) {
		endLine = len(fileLines) - 1
	}

	// Ensure startLine is not greater than endLine (can happen if contextLines is large and file is small)
	if startLine > endLine {
		startLine = endLine
	}

	// Extract the lines
	selectedLines := fileLines[startLine : endLine+1]
	contextText := strings.Join(selectedLines, "\n")

	// Create the location corresponding to the extracted text
	// Start position: beginning of the startLine
	// End position: end of the endLine (use a large character number or actual length if needed,
	// but for scope identification, just the lines are often sufficient).
	// Using length of last line for slightly more accuracy.
	endChar := uint32(0)
	if endLine >= 0 && endLine < len(fileLines) { // Check bounds for fileLines[endLine]
		endChar = uint32(len(fileLines[endLine]))
	}

	contextLocation := protocol.Location{
		URI: loc.URI, // Use the original URI
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startLine),
				Character: 0, // Start of the line
			},
			End: protocol.Position{
				Line:      uint32(endLine),
				Character: endChar, // End of the last included line
			},
		},
	}

	// Return the extracted text, its location, and nil error
	return contextText, contextLocation, nil
}

// TruncateDefinition shortens a definition if it's too long
// It keeps the beginning, the context around targetLine, and the end
func TruncateDefinition(definition string, targetLine int, contextSize int, maxLines int) string {
	lines := strings.Split(definition, "\n")

	// If the definition is already short enough, just return it
	if len(lines) <= maxLines {
		return definition
	}

	// Calculate the range to keep around the target line
	contextStart := targetLine - contextSize
	if contextStart < 0 {
		contextStart = 0
	}

	contextEnd := targetLine + contextSize
	if contextEnd >= len(lines) {
		contextEnd = len(lines) - 1
	}

	// Decide how many lines to keep from beginning and end
	remainingLines := maxLines - (contextEnd - contextStart + 1) - 2 // -2 for ellipsis markers
	startLines := remainingLines / 2
	endLines := remainingLines - startLines

	// Adjust if context overlaps with start/end segments
	if contextStart < startLines {
		startLines = contextStart
		endLines = remainingLines - startLines
	}

	if contextEnd > (len(lines) - 1 - endLines) {
		endLines = len(lines) - 1 - contextEnd
		startLines = remainingLines - endLines
	}

	// Create the resulting truncated definition
	var result []string

	// Add beginning lines if not overlapping with context
	if contextStart > startLines {
		result = append(result, lines[:startLines]...)
		result = append(result, "...")
	} else {
		// Just use all lines up to context start
		result = append(result, lines[:contextStart]...)
	}

	// Add the context around the target line
	result = append(result, lines[contextStart:contextEnd+1]...)

	// Add end lines if not overlapping with context
	if contextEnd < len(lines)-1-endLines {
		result = append(result, "...")
		result = append(result, lines[len(lines)-endLines:]...)
	} else {
		// Just use all lines from context end
		result = append(result, lines[contextEnd+1:]...)
	}

	return strings.Join(result, "\n")
}
