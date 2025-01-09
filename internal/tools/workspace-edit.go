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

// applyTextEdits applies a sequence of TextEdits to a file
func applyTextEdits(uri protocol.DocumentUri, edits []protocol.TextEdit) error {
	path := strings.TrimPrefix(string(uri), "file://")

	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

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
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// applyTextEdit applies a single TextEdit to the given lines
func applyTextEdit(lines []string, edit protocol.TextEdit) ([]string, error) {
	startLine := int(edit.Range.Start.Line)
	endLine := int(edit.Range.End.Line)
	startChar := int(edit.Range.Start.Character)
	endChar := int(edit.Range.End.Character)

	if startLine < 0 || startLine >= len(lines) || endLine < 0 || endLine >= len(lines) {
		return nil, fmt.Errorf("invalid line range: %v", edit.Range)
	}

	// Handle single-line edit
	if startLine == endLine {
		line := lines[startLine]
		if startChar < 0 || startChar > len(line) || endChar < 0 || endChar > len(line) {
			return nil, fmt.Errorf("invalid character range: %v", edit.Range)
		}

		newLine := line[:startChar] + edit.NewText + line[endChar:]
		lines[startLine] = newLine
		return lines, nil
	}

	// Handle multi-line edit
	newLines := make([]string, 0, len(lines))

	// Add lines before edit
	newLines = append(newLines, lines[:startLine]...)

	// Add first line of edit
	firstLine := lines[startLine]
	if startChar < 0 || startChar > len(firstLine) {
		return nil, fmt.Errorf("invalid start character: %v", edit.Range.Start)
	}
	newLineContent := firstLine[:startChar] + edit.NewText

	// Split new content into lines if it contains newlines
	if strings.Contains(edit.NewText, "\n") {
		splitNewContent := strings.Split(newLineContent, "\n")
		newLines = append(newLines, splitNewContent...)
	} else {
		newLines = append(newLines, newLineContent)
	}

	// Add remaining lines after edit
	if endLine+1 < len(lines) {
		lastLine := lines[endLine]
		if endChar >= 0 && endChar <= len(lastLine) {
			newLines[len(newLines)-1] += lastLine[endChar:]
		}
		newLines = append(newLines, lines[endLine+1:]...)
	}

	return newLines, nil
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
