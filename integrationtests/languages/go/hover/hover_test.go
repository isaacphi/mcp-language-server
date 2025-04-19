package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the Go language server
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
		// Tests using types.go file
		{
			name:         "StructType",
			file:         "types.go",
			line:         6,
			column:       6,
			expectedText: "SharedStruct",
			snapshotName: "struct-type",
		},
		{
			name:         "StructMethod",
			file:         "types.go",
			line:         14,
			column:       18,
			expectedText: "Method",
			snapshotName: "struct-method",
		},
		{
			name:         "InterfaceType",
			file:         "types.go",
			line:         19,
			column:       6,
			expectedText: "SharedInterface",
			snapshotName: "interface-type",
		},
		{
			name:         "Constant",
			file:         "types.go",
			line:         25,
			column:       7,
			expectedText: "SharedConstant",
			snapshotName: "constant",
		},
		{
			name:         "InterfaceMethodImplementation",
			file:         "types.go",
			line:         31,
			column:       18,
			expectedText: "Process",
			snapshotName: "interface-method-impl",
		},
		// Test for a location without hover info (empty space)
		{
			name:           "NoHoverInfo",
			file:           "types.go",
			line:           3, // Line with just "import" statement
			column:         1, // First column (whitespace)
			unexpectedText: "func",
			snapshotName:   "no-hover-info",
		},
		// Test for a location outside the file
		{
			name:           "OutsideFile",
			file:           "types.go",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "func",
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
					common.SnapshotTest(t, "go", "hover", tt.snapshotName, err.Error())
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

			common.SnapshotTest(t, "go", "hover", tt.snapshotName, result)
		})
	}
}
