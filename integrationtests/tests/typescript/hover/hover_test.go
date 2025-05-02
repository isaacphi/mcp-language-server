package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/typescript/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the TypeScript language server
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
		// Tests using main.ts file
		{
			name:         "Function",
			file:         "main.ts",
			line:         2,
			column:       17,
			expectedText: "TestFunction",
			snapshotName: "function",
		},
		{
			name:         "Interface",
			file:         "main.ts",
			line:         8,
			column:       18,
			expectedText: "TestInterface",
			snapshotName: "interface-type",
		},
		{
			name:         "Class",
			file:         "main.ts",
			line:         14,
			column:       14,
			expectedText: "TestClass",
			snapshotName: "class",
		},
		{
			name:         "ClassMethod",
			file:         "main.ts",
			line:         21,
			column:       9,
			expectedText: "method",
			snapshotName: "class-method",
		},
		{
			name:         "Type",
			file:         "main.ts",
			line:         27,
			column:       13,
			expectedText: "TestType",
			snapshotName: "type",
		},
		{
			name:         "Variable",
			file:         "main.ts",
			line:         30,
			column:       20,
			expectedText: "TestVariable",
			snapshotName: "variable",
		},
		{
			name:         "Constant",
			file:         "main.ts",
			line:         33,
			column:       20,
			expectedText: "TestConstant",
			snapshotName: "constant",
		},
		// Test for a location without hover info (comment)
		{
			name:           "NoHoverInfo",
			file:           "main.ts",
			line:           7, // Comment line
			column:         1, // First column (whitespace)
			unexpectedText: "function",
			snapshotName:   "no-hover-info",
		},
		// Test for a location outside the file
		{
			name:           "OutsideFile",
			file:           "main.ts",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "function",
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
					if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
						t.Errorf("Expected hover info to contain %q but got: %s", tt.expectedText, result)
					}

					// Verify unexpected content is absent
					if tt.unexpectedText != "" && strings.Contains(result, tt.unexpectedText) {
						t.Errorf("Expected hover info NOT to contain %q but it was found: %s", tt.unexpectedText, result)
					}
					// Skip snapshot because CI contains unique paths in output
					//common.SnapshotTest(t, "typescript", "hover", tt.snapshotName, err.Error())
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

			common.SnapshotTest(t, "typescript", "hover", tt.snapshotName, result)
		})
	}
}
