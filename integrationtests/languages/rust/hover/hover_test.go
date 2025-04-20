package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/rust/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the Rust language server
func TestHover(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open all files to ensure rust-analyzer indexes everything
		filesToOpen := []string{
			"src/main.rs",
			"src/types.rs",
			"src/helper.rs",
			"src/consumer.rs",
			"src/another_consumer.rs",
			"src/clean.rs",
		}

		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				// Don't fail the test, some files might not exist in certain tests
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}
	}

	tests := []struct {
		name           string
		file           string
		line           int
		column         int
		expectedText   string // Text that should be in the hover result
		unexpectedText string // Text that should NOT be in the hover result (optional)
		snapshotName   string
	}{
		// Tests using types.rs file
		{
			name:         "Struct",
			file:         "src/types.rs",
			line:         13,
			column:       12,
			expectedText: "TestStruct",
			snapshotName: "struct-type",
		},
		{
			name:         "StructMethod",
			file:         "src/types.rs",
			line:         27,
			column:       15,
			expectedText: "method",
			snapshotName: "struct-method",
		},
		{
			name:         "Trait",
			file:         "src/types.rs",
			line:         33,
			column:       12,
			expectedText: "TestInterface",
			snapshotName: "trait-type",
		},
		{
			name:         "Constant",
			file:         "src/types.rs",
			line:         4,
			column:       12,
			expectedText: "TEST_CONSTANT",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			file:         "src/types.rs",
			line:         7,
			column:       12,
			expectedText: "TEST_VARIABLE",
			snapshotName: "variable",
		},
		{
			name:         "Function",
			file:         "src/types.rs",
			line:         81,
			column:       8,
			expectedText: "test_function",
			snapshotName: "function",
		},
		// Test for a location without hover info (empty space)
		{
			name:           "NoHoverInfo",
			file:           "src/types.rs",
			line:           1, // Comment line
			column:         1, // First column
			unexpectedText: "fn",
			snapshotName:   "no-hover-info",
		},
		// Test for a location outside the file
		{
			name:           "OutsideFile",
			file:           "src/types.rs",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "fn",
			snapshotName:   "outside-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get a test suite
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
			defer cancel()

			// Open all files and wait for rust-analyzer to index them
			openAllFilesAndWait(suite, ctx)

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
					common.SnapshotTest(t, "rust", "hover", tt.snapshotName, err.Error())
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

			common.SnapshotTest(t, "rust", "hover", tt.snapshotName, result)
		})
	}
}
