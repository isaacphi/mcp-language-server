package definition_test

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

// TestReadDefinition tests the ReadDefinition tool with various Rust type definitions
func TestReadDefinition(t *testing.T) {
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

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	// Open all files and wait for rust-analyzer to index them
	openAllFilesAndWait(suite, ctx)

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{
			name:         "Function",
			symbolName:   "foo_bar",
			expectedText: "fn foo_bar()",
			snapshotName: "foobar",
		},
		{
			name:         "Struct",
			symbolName:   "TestStruct",
			expectedText: "struct TestStruct",
			snapshotName: "struct",
		},
		{
			name:         "Method",
			symbolName:   "method",
			expectedText: "fn method(&self)",
			snapshotName: "method",
		},
		{
			name:         "Trait",
			symbolName:   "TestInterface",
			expectedText: "trait TestInterface",
			snapshotName: "interface",
		},
		{
			name:         "Type",
			symbolName:   "TestType",
			expectedText: "type TestType",
			snapshotName: "type",
		},
		{
			name:         "Constant",
			symbolName:   "TEST_CONSTANT",
			expectedText: "const TEST_CONSTANT",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			symbolName:   "TEST_VARIABLE",
			expectedText: "static TEST_VARIABLE",
			snapshotName: "variable",
		},
		{
			name:         "Function",
			symbolName:   "test_function",
			expectedText: "fn test_function()",
			snapshotName: "function",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName, true)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "rust", "definition", tc.snapshotName, result)
		})
	}
}
