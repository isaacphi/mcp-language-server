package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ApplyWorkspaceEdit applies the given WorkspaceEdit to the filesystem
func ApplyWorkspaceEdit(edit protocol.WorkspaceEdit) error {
	// Handle Changes field
	for uri, textEdits := range edit.Changes {
		if err := applyTextEdits(uri, textEdits); err != nil {
			return fmt.Errorf("failed to apply text edits: %w", err)
		}
	}

	// Handle DocumentChanges field
	for _, change := range edit.DocumentChanges {
		if err := applyDocumentChange(change); err != nil {
			return fmt.Errorf("failed to apply document change: %w", err)
		}
	}

	return nil
}

func applyTextEdits(uri protocol.DocumentUri, edits []protocol.TextEdit) error {
	path := strings.TrimPrefix(string(uri), "file://")

	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Split into lines while preserving empty lines
	// Use strings.Split instead of SplitAfter since we'll handle newlines separately
	lines := strings.Split(string(content), "\n")
	endsWithNewline := len(content) > 0 && content[len(content)-1] == '\n'

	// Sort edits in reverse order to avoid position shifting
	sortedEdits := make([]protocol.TextEdit, len(edits))
	copy(sortedEdits, edits)
	sortTextEdits(sortedEdits)

	// Apply each edit
	for _, edit := range sortedEdits {
		lines, err = applyTextEdit(lines, edit)
		if err != nil {
			return fmt.Errorf("failed to apply edit: %w", err)
		}
	}

	// Write back to file
	newContent := strings.Join(lines, "\n")
	if endsWithNewline {
		newContent += "\n"
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
func applyTextEdit(lines []string, edit protocol.TextEdit) ([]string, error) {
	startLine := int(edit.Range.Start.Line)
	endLine := int(edit.Range.End.Line)
	startChar := int(edit.Range.Start.Character)
	endChar := int(edit.Range.End.Character)

	if startLine < 0 || startLine >= len(lines) {
		return nil, fmt.Errorf("invalid start line: %d", startLine)
	}
	if endLine < 0 || endLine >= len(lines) {
		return nil, fmt.Errorf("invalid end line: %d", endLine)
	}

	// Create result slice with initial capacity
	result := make([]string, 0, len(lines))

	// Copy lines before the edit
	result = append(result, lines[:startLine]...)

	// Get the prefix of the start line up to the edit start
	startLineContent := lines[startLine]
	if startChar < 0 || startChar > len(startLineContent) {
		return nil, fmt.Errorf("invalid start character: %d", startChar)
	}
	prefix := startLineContent[:startChar]

	// Get the suffix of the end line after the edit end
	endLineContent := lines[endLine]
	if endChar < 0 || endChar > len(endLineContent) {
		return nil, fmt.Errorf("invalid end character: %d", endChar)
	}
	suffix := endLineContent[endChar:]

	// Split the new text into lines
	newLines := strings.Split(edit.NewText, "\n")

	if len(newLines) == 0 {
		// Handle empty replacement
		if startLine == endLine {
			// Single line edit
			result = append(result, prefix+suffix)
		} else {
			// Multi-line removal
			result = append(result, prefix+suffix)
		}
	} else {
		// Handle the first and last lines of the new content specially
		firstNew := newLines[0]
		lastNew := newLines[len(newLines)-1]

		if len(newLines) == 1 {
			// Single line insertion/replacement
			result = append(result, prefix+firstNew+suffix)
		} else {
			// Multi-line insertion/replacement
			result = append(result, prefix+firstNew)
			result = append(result, newLines[1:len(newLines)-1]...)
			result = append(result, lastNew+suffix)
		}
	}

	// Add remaining lines after the edit
	if endLine+1 < len(lines) {
		result = append(result, lines[endLine+1:]...)
	}

	return result, nil
}

// applyDocumentChange applies a DocumentChange (create/rename/delete operations)
func applyDocumentChange(change protocol.DocumentChange) error {
	if change.CreateFile != nil {
		path := strings.TrimPrefix(string(change.CreateFile.URI), "file://")
		if change.CreateFile.Options != nil {
			if change.CreateFile.Options.Overwrite {
				// Proceed with overwrite
			} else if change.CreateFile.Options.IgnoreIfExists {
				if _, err := os.Stat(path); err == nil {
					return nil // File exists and we're ignoring it
				}
			}
		}
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	if change.DeleteFile != nil {
		path := strings.TrimPrefix(string(change.DeleteFile.URI), "file://")
		if change.DeleteFile.Options != nil && change.DeleteFile.Options.Recursive {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to delete directory recursively: %w", err)
			}
		} else {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to delete file: %w", err)
			}
		}
	}

	if change.RenameFile != nil {
		oldPath := strings.TrimPrefix(string(change.RenameFile.OldURI), "file://")
		newPath := strings.TrimPrefix(string(change.RenameFile.NewURI), "file://")
		if change.RenameFile.Options != nil {
			if !change.RenameFile.Options.Overwrite {
				if _, err := os.Stat(newPath); err == nil {
					return fmt.Errorf("target file already exists and overwrite is not allowed: %s", newPath)
				}
			}
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename file: %w", err)
		}
	}

	if change.TextDocumentEdit != nil {
		textEdits := make([]protocol.TextEdit, len(change.TextDocumentEdit.Edits))
		for i, edit := range change.TextDocumentEdit.Edits {
			var err error
			textEdits[i], err = edit.AsTextEdit()
			if err != nil {
				return fmt.Errorf("invalid edit type: %w", err)
			}
		}
		return applyTextEdits(change.TextDocumentEdit.TextDocument.URI, textEdits)
	}

	return nil
}

// sortTextEdits sorts TextEdits in reverse order (bottom to top, right to left)
// This ensures that earlier edits don't invalidate the positions of later edits
func sortTextEdits(edits []protocol.TextEdit) {
	for i := 0; i < len(edits)-1; i++ {
		for j := i + 1; j < len(edits); j++ {
			if compareRange(edits[i].Range, edits[j].Range) < 0 {
				edits[i], edits[j] = edits[j], edits[i]
			}
		}
	}
}

// compareRange compares two ranges for sorting
// Returns -1 if r1 comes before r2, 1 if r1 comes after r2, 0 if equal
func compareRange(r1, r2 protocol.Range) int {
	if r1.Start.Line != r2.Start.Line {
		return int(r2.Start.Line - r1.Start.Line)
	}
	if r1.Start.Character != r2.Start.Character {
		return int(r2.Start.Character - r1.Start.Character)
	}
	if r1.End.Line != r2.End.Line {
		return int(r2.End.Line - r1.End.Line)
	}
	return int(r2.End.Character - r1.End.Character)
}
