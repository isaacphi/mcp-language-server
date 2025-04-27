package utilities

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

var (
	osReadFile  = os.ReadFile
	osWriteFile = os.WriteFile
	osStat      = os.Stat
	osRemove    = os.Remove
	osRemoveAll = os.RemoveAll
	osRename    = os.Rename
)

// ApplyTextEdits applies a sequence of text edits to a file specified by URI
func ApplyTextEdits(uri protocol.DocumentUri, edits []protocol.TextEdit) error {
	path := strings.TrimPrefix(string(uri), "file://")

	// Read the file content
	content, err := osReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Track if file ends with a newline
	endsWithNewline := len(content) > 0 && bytes.HasSuffix(content, []byte(lineEnding))

	// Split into lines without the endings
	lines := strings.Split(string(content), lineEnding)

	// Check for overlapping edits
	for i, edit1 := range edits {
		for j := i + 1; j < len(edits); j++ {
			if RangesOverlap(edit1.Range, edits[j].Range) {
				return fmt.Errorf("overlapping edits detected between edit %d and %d", i, j)
			}
		}
	}

	// Sort edits in reverse order
	sortedEdits := make([]protocol.TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		if sortedEdits[i].Range.Start.Line != sortedEdits[j].Range.Start.Line {
			return sortedEdits[i].Range.Start.Line > sortedEdits[j].Range.Start.Line
		}
		return sortedEdits[i].Range.Start.Character > sortedEdits[j].Range.Start.Character
	})

	// Apply each edit
	for _, edit := range sortedEdits {
		newLines, err := ApplyTextEdit(lines, edit, lineEnding)
		if err != nil {
			return fmt.Errorf("failed to apply edit: %w", err)
		}
		lines = newLines
	}

	// Join lines with proper line endings
	var newContent strings.Builder
	for i, line := range lines {
		if i > 0 {
			newContent.WriteString(lineEnding)
		}
		newContent.WriteString(line)
	}

	// Only add a newline if the original file had one and we haven't already added it
	if endsWithNewline && !strings.HasSuffix(newContent.String(), lineEnding) {
		newContent.WriteString(lineEnding)
	}

	if err := osWriteFile(path, []byte(newContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ApplyTextEdit applies a single text edit to a set of lines
func ApplyTextEdit(lines []string, edit protocol.TextEdit, lineEnding string) ([]string, error) {
	startLine := int(edit.Range.Start.Line)
	endLine := int(edit.Range.End.Line)
	startChar := int(edit.Range.Start.Character)
	endChar := int(edit.Range.End.Character)

	// Validate positions
	if startLine < 0 || startLine >= len(lines) {
		return nil, fmt.Errorf("invalid start line: %d", startLine)
	}
	if endLine < 0 || endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	// Create result slice with initial capacity
	result := make([]string, 0, len(lines))

	// Copy lines before edit
	result = append(result, lines[:startLine]...)

	// Get the prefix of the start line
	startLineContent := lines[startLine]
	if startChar < 0 || startChar > len(startLineContent) {
		startChar = len(startLineContent)
	}
	prefix := startLineContent[:startChar]

	// Get the suffix of the end line
	endLineContent := lines[endLine]
	if endChar < 0 || endChar > len(endLineContent) {
		endChar = len(endLineContent)
	}
	suffix := endLineContent[endChar:]

	// Handle the edit
	if edit.NewText == "" {
		if prefix+suffix != "" {
			result = append(result, prefix+suffix)
		}
	} else {
		// Split new text into lines
		newLines := strings.Split(edit.NewText, "\n")

		if len(newLines) == 1 {
			// Single line change
			result = append(result, prefix+newLines[0]+suffix)
		} else if endLine == startLine {
			// Multi-line insertion within the same line
			result = append(result, prefix+newLines[0])
			if len(newLines) > 2 {
				result = append(result, newLines[1:len(newLines)-1]...)
			}
			result = append(result, newLines[len(newLines)-1]+suffix)
		} else {
			// Multi-line change across different lines
			result = append(result, prefix+newLines[0])
			if len(newLines) > 2 {
				result = append(result, newLines[1:len(newLines)-1]...)
			}
			// Only append the final line with suffix if we're not replacing the entire content
			if len(suffix) > 0 || endLine < len(lines)-1 {
				result = append(result, newLines[len(newLines)-1]+suffix)
			} else {
				result = append(result, newLines[len(newLines)-1])
			}
		}
	}

	// Add remaining lines
	if endLine+1 < len(lines) {
		result = append(result, lines[endLine+1:]...)
	}

	return result, nil
}

// ApplyDocumentChange applies a DocumentChange (create/rename/delete operations)
func ApplyDocumentChange(change protocol.DocumentChange) error {
	if change.CreateFile != nil {
		path := strings.TrimPrefix(string(change.CreateFile.URI), "file://")
		if change.CreateFile.Options != nil {
			if change.CreateFile.Options.Overwrite {
				// Proceed with overwrite
			} else if change.CreateFile.Options.IgnoreIfExists {
				if _, err := osStat(path); err == nil {
					return nil // File exists and we're ignoring it
				}
			}
		}
		if err := osWriteFile(path, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	if change.DeleteFile != nil {
		path := strings.TrimPrefix(string(change.DeleteFile.URI), "file://")
		if change.DeleteFile.Options != nil && change.DeleteFile.Options.Recursive {
			if err := osRemoveAll(path); err != nil {
				return fmt.Errorf("failed to delete directory recursively: %w", err)
			}
		} else {
			if err := osRemove(path); err != nil {
				return fmt.Errorf("failed to delete file: %w", err)
			}
		}
	}

	if change.RenameFile != nil {
		oldPath := strings.TrimPrefix(string(change.RenameFile.OldURI), "file://")
		newPath := strings.TrimPrefix(string(change.RenameFile.NewURI), "file://")
		if change.RenameFile.Options != nil {
			if !change.RenameFile.Options.Overwrite {
				if _, err := osStat(newPath); err == nil {
					return fmt.Errorf("target file already exists and overwrite is not allowed: %s", newPath)
				}
			}
		}
		if err := osRename(oldPath, newPath); err != nil {
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
		return ApplyTextEdits(change.TextDocumentEdit.TextDocument.URI, textEdits)
	}

	return nil
}

// ApplyWorkspaceEdit applies the given WorkspaceEdit to the filesystem
func ApplyWorkspaceEdit(edit protocol.WorkspaceEdit) error {
	// Handle Changes field
	for uri, textEdits := range edit.Changes {
		if err := ApplyTextEdits(uri, textEdits); err != nil {
			return fmt.Errorf("failed to apply text edits: %w", err)
		}
	}

	// Handle DocumentChanges field
	for _, change := range edit.DocumentChanges {
		coreLogger.Warn("Document change: %v", spew.Sdump(change))
		if err := ApplyDocumentChange(change); err != nil {
			return fmt.Errorf("failed to apply document change: %w", err)
		}
	}

	return nil
}

// RangesOverlap checks if two ranges overlap in position
func RangesOverlap(r1, r2 protocol.Range) bool {
	if r1.Start.Line > r2.End.Line || r2.Start.Line > r1.End.Line {
		return false
	}
	if r1.Start.Line == r2.End.Line && r1.Start.Character > r2.End.Character {
		return false
	}
	if r2.Start.Line == r1.End.Line && r2.Start.Character > r1.End.Character {
		return false
	}
	return true
}
