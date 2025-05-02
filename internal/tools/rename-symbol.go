package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

// RenameSymbol renames a symbol (variable, function, class, etc.) at the specified position
// It uses the LSP rename functionality to handle all references across files
func RenameSymbol(ctx context.Context, client *lsp.Client, filePath string, line, column int, newName string) (string, error) {
	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Convert 1-indexed line/column to 0-indexed for LSP protocol
	uri := protocol.DocumentUri("file://" + filePath)
	position := protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(column - 1),
	}

	// Create the rename parameters
	params := protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
		Position: position,
		NewName:  newName,
	}

	// Skip the PrepareRename check as it might not be supported by all language servers
	// Execute the rename directly

	// Execute the rename operation
	workspaceEdit, err := client.Rename(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to rename symbol: %v", err)
	}

	// Count the changes that will be made
	changeCount := 0
	fileCount := 0

	// Build output
	var locationsBuilder strings.Builder

	// Create a slice to store all changes before sorting and writing
	type FileChanges struct {
		URI       string
		Locations string
	}
	var allChanges []FileChanges

	// Count changes in Changes field
	if workspaceEdit.Changes != nil {
		fileCount = len(workspaceEdit.Changes)
		for uri, edits := range workspaceEdit.Changes {
			changeCount += len(edits)
			var locs strings.Builder
			for i, change := range edits {
				locs.WriteString(
					fmt.Sprintf("L%d:C%d", change.Range.Start.Line+1, change.Range.Start.Character+1),
				)
				if i != len(edits)-1 {
					locs.WriteString(", ")
				}
			}
			allChanges = append(allChanges, FileChanges{
				URI:       string(uri),
				Locations: locs.String(),
			})
		}
	}

	// Count changes in DocumentChanges field
	for _, change := range workspaceEdit.DocumentChanges {
		if change.TextDocumentEdit != nil {
			var locs strings.Builder
			for i, edit := range change.TextDocumentEdit.Edits {
				textEdit, err := edit.AsTextEdit()
				if err == nil {
					locs.WriteString(fmt.Sprintf("L%d:C%d", textEdit.Range.Start.Line+1, textEdit.Range.Start.Character+1))
					if i != len(change.TextDocumentEdit.Edits)-1 {
						locs.WriteString(", ")
					}
				}
			}
			allChanges = append(allChanges, FileChanges{
				URI:       string(change.TextDocumentEdit.TextDocument.URI),
				Locations: locs.String(),
			})
			fileCount++
			changeCount += len(change.TextDocumentEdit.Edits)
		}
	}

	// Sort changes by filename for consistent output
	sort.Slice(allChanges, func(i, j int) bool {
		return allChanges[i].URI < allChanges[j].URI
	})

	// Write sorted changes to the output builder
	for _, change := range allChanges {
		locationsBuilder.WriteString(fmt.Sprintf("%s: %s\n", change.URI, change.Locations))
	}

	// Apply the workspace edit to files:workspaceEdit
	if err := utilities.ApplyWorkspaceEdit(workspaceEdit); err != nil {
		return "", fmt.Errorf("failed to apply changes: %v", err)
	}

	if fileCount == 0 || changeCount == 0 {
		return "Failed to rename symbol. 0 occurrences found.", nil
	}

	// Generate a summary of changes made
	return fmt.Sprintf("Successfully renamed symbol to '%s'.\nUpdated %d occurrences across %d files:\n%s",
		newName, changeCount, fileCount, locationsBuilder.String()), nil
}
