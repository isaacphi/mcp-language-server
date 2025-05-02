package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

type TextEdit struct {
	StartLine int    `json:"startLine" jsonschema:"required,description=Start line to replace, inclusive"`
	EndLine   int    `json:"endLine" jsonschema:"required,description=End line to replace, inclusive"`
	NewText   string `json:"newText" jsonschema:"description=Replacement text. Replace with the new text. Leave blank to remove lines."`
}

func ApplyTextEdits(ctx context.Context, client *lsp.Client, filePath string, edits []TextEdit) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Create a sorted copy of edits for reporting
	sortedEdits := make([]TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		return sortedEdits[i].StartLine < sortedEdits[j].StartLine
	})

	// Track lines added and removed for sorted edits
	linesRemovedSorted := 0
	linesAddedSorted := 0
	for _, edit := range sortedEdits {
		// Calculate lines removed: end - start + 1
		removedLineCount := edit.EndLine - edit.StartLine + 1
		linesRemovedSorted += removedLineCount

		// Calculate lines added: count newlines in the replacement text + 1
		addedLineCount := 1
		if edit.NewText != "" {
			addedLineCount = strings.Count(edit.NewText, "\n") + 1
		} else if edit.NewText == "" {
			addedLineCount = 0
		}
		linesAddedSorted += addedLineCount
	}

	// Sort edits by line number in descending order to process from bottom to top
	// This way line numbers don't shift under us as we make edits
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].StartLine > edits[j].StartLine
	})

	// Convert from input format to protocol.TextEdit
	var textEdits []protocol.TextEdit
	for _, edit := range edits {
		// Get the range covering the requested lines
		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// Always do a replacement
		textEdits = append(textEdits, protocol.TextEdit{
			Range:   rng,
			NewText: edit.NewText,
		})
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri(filePath): textEdits,
		},
	}

	if err := utilities.ApplyWorkspaceEdit(edit); err != nil {
		return "", fmt.Errorf("failed to apply text edits: %v", err)
	}

	return fmt.Sprintf("Successfully applied text edits. %d lines removed, %d lines added.", linesRemovedSorted, linesAddedSorted), nil
}

// getRange creates a protocol.Range that covers the specified start and end lines
func getRange(startLine, endLine int, filePath string) (protocol.Range, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Split lines without the line endings
	lines := strings.Split(string(content), lineEnding)

	// Handle start line positioning
	if startLine < 1 {
		return protocol.Range{}, fmt.Errorf("start line must be >= 1, got %d", startLine)
	}

	// Convert to 0-based line numbers
	startIdx := startLine - 1
	endIdx := endLine - 1

	// Handle EOF positioning
	if startIdx >= len(lines) {
		// For EOF, we want to point to the end of the last content-bearing line
		lastContentLineIdx := len(lines) - 1
		if lastContentLineIdx >= 0 && lines[lastContentLineIdx] == "" {
			lastContentLineIdx--
		}

		if lastContentLineIdx < 0 {
			lastContentLineIdx = 0
		}

		pos := protocol.Position{
			Line:      uint32(lastContentLineIdx),
			Character: uint32(len(lines[lastContentLineIdx])),
		}

		return protocol.Range{
			Start: pos,
			End:   pos,
		}, nil
	}

	// Normal range handling
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	// Always use the full line range for consistency
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0, // Always start at beginning of line
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(len(lines[endIdx])), // Go to end of last line
		},
	}, nil
}
