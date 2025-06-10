package definition_test

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

// TestReadDefinition tests the ReadDefinition tool with various C++ type definitions
func TestReadDefinition(t *testing.T) {
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
		time.Sleep(10 * time.Second)
	}

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	// Open all files and wait for clangd to index them
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
			expectedText: "void foo_bar()",
			snapshotName: "foobar",
		},
		{
			name:         "Class",
			symbolName:   "TestClass",
			expectedText: "class TestClass",
			snapshotName: "class",
		},
		{
			name:         "Method",
			symbolName:   "method",
			expectedText: "void method(int param)",
			snapshotName: "method",
		},
		{
			name:         "Namespace function",
			symbolName:   "nsFunction2",
			expectedText: "void nsFunction2()",
			snapshotName: "nsFunction",
		},
		{
			name:         "Struct",
			symbolName:   "TestStruct",
			expectedText: "struct TestStruct",
			snapshotName: "struct",
		},
		{
			name:         "Type",
			symbolName:   "TestType",
			expectedText: "using TestType",
			snapshotName: "type",
		},
		{
			name:         "Constant",
			symbolName:   "TEST_CONSTANT",
			expectedText: "const int TEST_CONSTANT",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			symbolName:   "TEST_VARIABLE",
			expectedText: "int TEST_VARIABLE",
			snapshotName: "variable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "clangd", "definition", tc.snapshotName, result)
		})
	}
}

func TestReadDefinitionInAnotherFile(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open all files to ensure clangd indexes everything
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
		time.Sleep(5 * time.Second)
	}

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	// Open all files and wait for clangd to index them
	openAllFilesAndWait(suite, ctx)

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{
			name:         "Function",
			symbolName:   "helperFunction",
			expectedText: "void helperFunction()",
			snapshotName: "helperFunction",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "clangd", "definition", tc.snapshotName, result)
		})
	}
}
