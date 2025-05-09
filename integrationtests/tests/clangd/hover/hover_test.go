package hover_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestHover tests hover functionality with the Clangd language server
func TestHover(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open one file so that clangd loads compiles commands and begins indexing
		filesToOpen := []string{
			"src/main.cpp",
		}

		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				// Don't fail the test, some files might not exist in certain tests
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}
		// Wait for indexing to complete. clangd won't index files until they are opened.
		time.Sleep(5 * time.Second)
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
		// Tests using types.cpp file
		{
			name:         "Class",
			file:         "src/consumer.cpp",
			line:         7, // Assuming TestClass definition
			column:       7, // "TestClass"
			expectedText: "class TestClass",
			snapshotName: "class-type",
		},
		{
			name:         "Method in Class",
			file:         "src/consumer.cpp",
			line:         14, // Assuming method definition within TestClass
			column:       10, // "method"
			expectedText: "public: void method(int param)",
			snapshotName: "class-method",
		},
		{
			name:         "Variable", // Global variable in helper.cpp with inline comment
			file:         "src/helper.cpp",
			line:         5, // Assuming TEST_VARIABLE definition
			column:       5, // "TEST_VARIABLE"
			expectedText: "int TEST_VARIABLE",
			snapshotName: "variable",
		},
		{
			name:         "Function in main.cpp",
			file:         "src/main.cpp",
			line:         14, // Assuming foo_bar use
			column:       6,  // "foo_bar"
			expectedText: "function foo_bar",
			snapshotName: "function-main",
		},
		{
			name:         "Function definition in helper.cpp",
			file:         "src/main.cpp",
			line:         11, // Assuming helperFunction use
			column:       7,  // "helperFunction"
			expectedText: "function helperFunction",
			snapshotName: "function-definition-cpp",
		},
		// Test for a location without hover info (empty space or comment)
		{
			name:           "NoHoverInfoComment",
			file:           "src/main.cpp",
			line:           4, // Comment line
			column:         1,
			unexpectedText: "void", // Should not find any specific code hover
			snapshotName:   "no-hover-info-comment",
		},
		// Test for a location outside the file - expect an error or no result
		{
			name:           "OutsideFile",
			file:           "src/main.cpp",
			line:           1000, // Line number beyond file length
			column:         1,
			unexpectedText: "void", // Should not find any specific code hover
			snapshotName:   "outside-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get a test suite
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
			defer cancel()

			// Open all files and wait for clangd to index them
			openAllFilesAndWait(suite, ctx)

			filePath := filepath.Join(suite.WorkspaceDir, tt.file)

			// Get hover info
			result, err := tools.GetHoverInfo(ctx, suite.Client, filePath, tt.line, tt.column)
			if err != nil {
				// For the "OutsideFile" test or "NoHoverInfo" we might expect an error or empty result
				if tt.name == "OutsideFile" || strings.HasPrefix(tt.name, "NoHoverInfo") {
					// Create a snapshot even for error case or empty result
					snapshotContent := "No hover information expected or error occurred."
					if err != nil {
						snapshotContent = err.Error()
					} else if result != "" {
						snapshotContent = result
					}
					common.SnapshotTest(t, "clangd", "hover", tt.snapshotName, snapshotContent)
					return
				}
				t.Fatalf("GetHoverInfo failed for %s: %v. Result: %s", tt.name, err, result)
			}

			// Verify expected content
			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Test %s: Expected hover info to contain %q but got: %s", tt.name, tt.expectedText, result)
			}

			// Verify unexpected content is absent
			if tt.unexpectedText != "" && strings.Contains(result, tt.unexpectedText) {
				t.Errorf("Test %s: Expected hover info NOT to contain %q but it was found: %s", tt.name, tt.unexpectedText, result)
			}

			common.SnapshotTest(t, "clangd", "hover", tt.snapshotName, result)
		})
	}
}
