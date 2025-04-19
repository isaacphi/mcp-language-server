package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/python/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the Python language server
func TestHover(t *testing.T) {
	tests := []struct {
		name           string
		file           string
		line           int
		column         int
		expectedText   string // Text that should be in the hover result
		unexpectedText string // Text that should NOT be in the hover result (optional)
		snapshotName   string
	}{
		// Tests using main.py file
		{
			name:         "Function",
			file:         "main.py",
			line:         6,
			column:       5,
			expectedText: "test_function",
			snapshotName: "function",
		},
		{
			name:         "Class",
			file:         "main.py",
			line:         18,
			column:       7,
			expectedText: "TestClass",
			snapshotName: "class",
		},
		{
			name:         "ClassMethod",
			file:         "main.py",
			line:         31,
			column:       10,
			expectedText: "test_method",
			snapshotName: "class-method",
		},
		{
			name:         "StaticMethod",
			file:         "main.py",
			line:         44,
			column:       15,
			expectedText: "static_method",
			snapshotName: "static-method",
		},
		{
			name:         "Constant",
			file:         "main.py",
			line:         79,
			column:       5,
			expectedText: "TEST_CONSTANT",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			file:         "main.py",
			line:         83,
			column:       5,
			expectedText: "test_variable",
			snapshotName: "variable",
		},
		{
			name:         "DerivedClass",
			file:         "main.py",
			line:         70,
			column:       7,
			expectedText: "DerivedClass",
			snapshotName: "derived-class",
		},
		// Test for a location without hover info (empty space)
		{
			name:           "NoHoverInfo",
			file:           "main.py",
			line:           2, // Blank line
			column:         1, // First column
			unexpectedText: "class",
			snapshotName:   "no-hover-info",
		},
		// Test for a location outside the file
		{
			name:           "OutsideFile",
			file:           "main.py",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "def",
			snapshotName:   "outside-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get a test suite
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
			defer cancel()

			filePath := filepath.Join(suite.WorkspaceDir, tt.file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tt.file, err)
			}

			// Get hover info
			result, err := tools.GetHoverInfo(ctx, suite.Client, filePath, tt.line, tt.column)
			if err != nil {
				// For the "OutsideFile" test, we expect an error
				if tt.name == "OutsideFile" {
					// Create a snapshot even for error case
					common.SnapshotTest(t, "python", "hover", tt.snapshotName, err.Error())
					return
				}
				t.Fatalf("GetHoverInfo failed: %v", err)
			}

			// Verify expected content
			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Expected hover info to contain %q but got: %s", tt.expectedText, result)
			}

			// Verify unexpected content is absent
			if tt.unexpectedText != "" && strings.Contains(result, tt.unexpectedText) {
				t.Errorf("Expected hover info NOT to contain %q but it was found: %s", tt.unexpectedText, result)
			}

			common.SnapshotTest(t, "python", "hover", tt.snapshotName, result)
		})
	}
}
