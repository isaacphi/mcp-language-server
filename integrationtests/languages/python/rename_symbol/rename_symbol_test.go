package rename_symbol_test

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

// TestRenameSymbol tests the RenameSymbol functionality with the Python language server
func TestRenameSymbol(t *testing.T) {
	// Test with a successful rename of a symbol that exists
	t.Run("SuccessfulRename", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		// Wait for initialization
		time.Sleep(2 * time.Second)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Ensure the file is open
		filePath := filepath.Join(suite.WorkspaceDir, "helper.py")
		err := suite.Client.OpenFile(ctx, filePath)
		if err != nil {
			t.Fatalf("Failed to open helper.py: %v", err)
		}

		// Open the consumer file too to ensure references are indexed
		consumerPath := filepath.Join(suite.WorkspaceDir, "consumer.py")
		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to open consumer.py: %v", err)
		}

		// Give the language server time to process the files
		time.Sleep(2 * time.Second)

		// Request to rename SHARED_CONSTANT to UPDATED_CONSTANT at its definition
		// The constant is defined at line 8, column 1 of helper.py
		result, err := tools.RenameSymbol(ctx, suite.Client, filePath, 8, 1, "UPDATED_CONSTANT")
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

		common.SnapshotTest(t, "python", "rename_symbol", "successful", result)

		// Verify that the rename worked by checking for the updated constant name in the file
		fileContent, err := suite.ReadFile("helper.py")
		if err != nil {
			t.Fatalf("Failed to read helper.py: %v", err)
		}

		if !strings.Contains(fileContent, "UPDATED_CONSTANT") {
			t.Errorf("Expected to find renamed constant 'UPDATED_CONSTANT' in helper.py")
		}

		// Also check that it was renamed in the consumer file
		consumerContent, err := suite.ReadFile("consumer.py")
		if err != nil {
			t.Fatalf("Failed to read consumer.py: %v", err)
		}

		if !strings.Contains(consumerContent, "UPDATED_CONSTANT") {
			t.Errorf("Expected to find renamed constant 'UPDATED_CONSTANT' in consumer.py")
		}
	})

	// Test with a symbol that doesn't exist
	t.Run("SymbolNotFound", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		// Wait for initialization
		time.Sleep(2 * time.Second)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Ensure the file is open
		filePath := filepath.Join(suite.WorkspaceDir, "clean.py")
		err := suite.Client.OpenFile(ctx, filePath)
		if err != nil {
			t.Fatalf("Failed to open clean.py: %v", err)
		}

		// Create a simple file with known content first
		simpleContent := `"""A simple Python file for testing."""

def dummy_function():
    """This is a dummy function."""
    pass
`
		err = suite.WriteFile("position_test.py", simpleContent)
		if err != nil {
			t.Fatalf("Failed to create position_test.py: %v", err)
		}

		testFilePath := filepath.Join(suite.WorkspaceDir, "position_test.py")
		err = suite.Client.OpenFile(ctx, testFilePath)
		if err != nil {
			t.Fatalf("Failed to open position_test.py: %v", err)
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
			common.SnapshotTest(t, "python", "rename_symbol", "not_found", result)
		} else {
			// If there was an error, check it and snapshot that instead
			errorMessage := err.Error()
			if !strings.Contains(errorMessage, "failed to rename") &&
				!strings.Contains(errorMessage, "not found") &&
				!strings.Contains(errorMessage, "cannot rename") {
				t.Errorf("Expected error message about failed rename but got: %s", errorMessage)
			}
			common.SnapshotTest(t, "python", "rename_symbol", "not_found", errorMessage)
		}
	})
}
