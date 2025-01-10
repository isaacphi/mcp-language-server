package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTextEdits(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		edits       []protocol.TextEdit
		expected    string
		shouldError bool
	}{
		{
			name:    "simple replacement",
			initial: "line1\nline2\nline3\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 1, Character: 5},
					},
					NewText: "replaced",
				},
			},
			expected: "line1\nreplaced\nline3\n",
		},
		{
			name:    "multiple line deletion",
			initial: "line1\nline2\nline3\nline4\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 2, Character: 0},
					},
					NewText: "",
				},
			},
			expected: "line1\nline3\nline4\n",
		},
		{
			name:    "insert between lines",
			initial: "line1\nline2\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 1, Character: 0},
					},
					NewText: "new line\n",
				},
			},
			expected: "line1\nnew line\nline2\n",
		},
		{
			name:    "append to end of file",
			initial: "line1\nline2", // Note: no trailing newline
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 5},
						End:   protocol.Position{Line: 1, Character: 5},
					},
					NewText: "\nline3",
				},
			},
			expected: "line1\nline2\nline3",
		},
		{
			name:    "handle CRLF line endings",
			initial: "line1\r\nline2\r\nline3\r\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 1, Character: 5},
					},
					NewText: "replaced",
				},
			},
			expected: "line1\r\nreplaced\r\nline3\r\n",
		},
		{
			name:    "multiple edits in reverse order",
			initial: "line1\nline2\nline3\nline4\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 5},
					},
					NewText: "first",
				},
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 2, Character: 0},
						End:   protocol.Position{Line: 2, Character: 5},
					},
					NewText: "third",
				},
			},
			expected: "first\nline2\nthird\nline4\n",
		},
		{
			name:    "overlapping edits should error",
			initial: "line1\nline2\nline3\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 1, Character: 5},
					},
					NewText: "overlap1",
				},
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 2, Character: 0},
					},
					NewText: "overlap2",
				},
			},
			shouldError: true,
		},
		{
			name:    "preserve empty lines",
			initial: "line1\n\nline3\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 5},
					},
					NewText: "replaced",
				},
			},
			expected: "replaced\n\nline3\n",
		},
		{
			name:    "multi-line replacement with different line endings",
			initial: "start\nline1\nline2\nend\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 2, Character: 5},
					},
					NewText: "replacement\nwith\nmultiple\nlines",
				},
			},
			expected: "start\nreplacement\nwith\nmultiple\nlines\nend\n",
		},
		{
			name:    "delete last character",
			initial: "line1\nline2\nline3\n",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 4},
						End:   protocol.Position{Line: 1, Character: 5},
					},
					NewText: "",
				},
			},
			expected: "line1\nline\nline3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.txt")
			err := os.WriteFile(tmpFile, []byte(tt.initial), 0644)
			require.NoError(t, err)

			// Apply edits
			err = applyTextEdits(protocol.DocumentUri("file://"+tmpFile), tt.edits)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Read result
			content, err := os.ReadFile(tmpFile)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(content))
		})
	}
}

// Helper function to test if edits overlap
func TestCheckOverlappingEdits(t *testing.T) {
	tests := []struct {
		name     string
		edits    []protocol.TextEdit
		overlaps bool
	}{
		{
			name: "non-overlapping edits",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 5},
					},
				},
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 1, Character: 5},
					},
				},
			},
			overlaps: false,
		},
		{
			name: "overlapping edits",
			edits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 1, Character: 5},
					},
				},
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 0},
						End:   protocol.Position{Line: 2, Character: 0},
					},
				},
			},
			overlaps: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Implement overlap checking logic here
			// This would be a good addition to your code
			overlaps := false
			for i := 0; i < len(tt.edits); i++ {
				for j := i + 1; j < len(tt.edits); j++ {
					if rangesOverlap(tt.edits[i].Range, tt.edits[j].Range) {
						overlaps = true
					}
				}
			}
			assert.Equal(t, tt.overlaps, overlaps)
		})
	}
}
