package rename_symbol_test

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

// TestRenameSymbol tests the RenameSymbol functionality with the TypeScript language server
func TestRenameSymbol(t *testing.T) {
	// Helper function to open all files and wait for indexing (copied from diagnostics_test.go)
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open all files to ensure TypeScript server indexes everything
		filesToOpen := []string{
			"main.ts",
			"helper.ts",
			"consumer.ts",
			"another_consumer.ts",
			"clean.ts",
		}

		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				// Don't fail the test, some files might not exist in certain tests
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}

		// Give TypeScript server time to process files
		time.Sleep(3 * time.Second)
	}

	// Test with a successful rename of a symbol that exists
	t.Run("SuccessfulRename", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Open all files and wait for TypeScript server to index them
		openAllFilesAndWait(suite, ctx)

		// Request to rename SharedConstant to UpdatedConstant at its definition
		// The constant is defined at line 39, column 14 of helper.ts
		helperPath := filepath.Join(suite.WorkspaceDir, "helper.ts")
		result, err := tools.RenameSymbol(ctx, suite.Client, helperPath, 39, 14, "UpdatedConstant")
		if err != nil {
			t.Fatalf("RenameSymbol failed: %v", err)
		}

		// Verify the constant was renamed
		if !strings.Contains(result, "Successfully renamed symbol") {
			t.Errorf("Expected success message but got: %s", result)
		}

		// Verify it's mentioned that it renamed multiple occurrences
		if !strings.Contains(result, "occurrences") {
			t.Errorf("Expected multiple occurrences to be renamed but got: %s", result)
		}

		common.SnapshotTest(t, "typescript", "rename_symbol", "successful", result)

		// Verify that the rename worked by checking for the updated constant name in the file
		fileContent, err := suite.ReadFile("helper.ts")
		if err != nil {
			t.Fatalf("Failed to read helper.ts: %v", err)
		}

		if !strings.Contains(fileContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in helper.ts")
		}

		// Also check that it was renamed in the consumer file
		consumerContent, err := suite.ReadFile("consumer.ts")
		if err != nil {
			t.Fatalf("Failed to read consumer.ts: %v", err)
		}

		if !strings.Contains(consumerContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in consumer.ts")
		}
	})

	// Test with a symbol that doesn't exist
	t.Run("SymbolNotFound", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Open all files and wait for TypeScript server to index them
		openAllFilesAndWait(suite, ctx)

		// Create a simple file with known content first
		simpleContent := `// A simple TypeScript file for testing

function dummyFunction(): void {
    // This is a dummy function
}
`
		err := suite.WriteFile("position_test.ts", simpleContent)
		if err != nil {
			t.Fatalf("Failed to create position_test.ts: %v", err)
		}

		testFilePath := filepath.Join(suite.WorkspaceDir, "position_test.ts")
		err = suite.Client.OpenFile(ctx, testFilePath)
		if err != nil {
			t.Fatalf("Failed to open position_test.ts: %v", err)
		}

		time.Sleep(1 * time.Second) // Give time for the file to be processed

		// Request to rename a symbol at a position where no symbol exists (in whitespace)
		result, err := tools.RenameSymbol(ctx, suite.Client, testFilePath, 4, 1, "NewName")

		// The language server might actually succeed with no rename operations
		// In this case, we check if it reports no occurrences
		if err == nil {
			// Check if result indicates nothing was renamed
			if !strings.Contains(result, "0 occurrences") {
				t.Errorf("Expected 0 occurrences or error for non-existent symbol, but got: %s", result)
			}
			common.SnapshotTest(t, "typescript", "rename_symbol", "not_found", result)
		} else {
			// If there was an error, check it and snapshot that instead
			errorMessage := err.Error()
			if !strings.Contains(errorMessage, "failed to rename") &&
				!strings.Contains(errorMessage, "not found") &&
				!strings.Contains(errorMessage, "cannot rename") {
				t.Errorf("Expected error message about failed rename but got: %s", errorMessage)
			}
			common.SnapshotTest(t, "typescript", "rename_symbol", "not_found", errorMessage)
		}
	})
}
