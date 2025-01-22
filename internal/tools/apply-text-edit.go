package tools

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

type TextEditType string

const (
	Replace TextEditType = "replace"
	Insert  TextEditType = "insert"
	Delete  TextEditType = "delete"
)

type TextEdit struct {
	Type      TextEditType `json:"type" jsonschema:"required,enum=replace|insert|delete,description=Type of edit operation (replace, insert, delete)"`
	StartLine int          `json:"startLine" jsonschema:"required,description=Start line to replace, inclusive"`
	EndLine   int          `json:"endLine" jsonschema:"required,description=End line to replace, inclusive"`
	NewText   string       `json:"newText" jsonschema:"description=Replacement text. Leave blank to clear lines."`
}

func ApplyTextEdits(ctx context.Context, client *lsp.Client, filePath string, edits []TextEdit) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}
	defer func() {
		if err := client.CloseFile(ctx, filePath); err != nil {
			log.Printf("Could not close file: %v", err)
		}
	}()

	// Sort edits by line number in descending order to process from bottom to top
	// This way line numbers don't shift under us as we make edits
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].StartLine > edits[j].StartLine
	})

	// Convert from input format to protocol.TextEdit
	var textEdits []protocol.TextEdit
	for _, edit := range edits {
		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		switch edit.Type {
		case Insert:
			// For insert, make it a zero-width range at the start position
			rng.End = rng.Start
		case Delete:
			// For delete, ensure NewText is empty
			edit.NewText = ""
		case Replace:
			// Replace uses the full range and NewText as-is
		}

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

	return "Successfully applied text edits.\nWARNING: line numbers may have changed. Re-read code before applying additional edits.", nil
}

// getRange now handles EOF insertions and is more precise about character positions
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

	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0,
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(len(lines[endIdx])),
		},
	}, nil
}
