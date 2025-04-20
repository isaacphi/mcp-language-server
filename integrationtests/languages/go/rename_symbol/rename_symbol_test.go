package rename_symbol_test

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

// TestRenameSymbol tests the RenameSymbol functionality with the Go language server
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
		filePath := filepath.Join(suite.WorkspaceDir, "types.go")
		err := suite.Client.OpenFile(ctx, filePath)
		if err != nil {
			t.Fatalf("Failed to open types.go: %v", err)
		}

		// Request to rename SharedConstant to UpdatedConstant at its definition
		// The constant is defined at line 25, column 7 of types.go
		result, err := tools.RenameSymbol(ctx, suite.Client, filePath, 25, 7, "UpdatedConstant")
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

		common.SnapshotTest(t, "go", "rename_symbol", "successful", result)

		// Verify that the rename worked by checking for the updated constant name in the file
		fileContent, err := suite.ReadFile("types.go")
		if err != nil {
			t.Fatalf("Failed to read types.go: %v", err)
		}

		if !strings.Contains(fileContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in types.go")
		}

		// Also check that it was renamed in the consumer file
		consumerContent, err := suite.ReadFile("consumer.go")
		if err != nil {
			t.Fatalf("Failed to read consumer.go: %v", err)
		}

		if !strings.Contains(consumerContent, "UpdatedConstant") {
			t.Errorf("Expected to find renamed constant 'UpdatedConstant' in consumer.go")
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
		filePath := filepath.Join(suite.WorkspaceDir, "clean.go")
		err := suite.Client.OpenFile(ctx, filePath)
		if err != nil {
			t.Fatalf("Failed to open clean.go: %v", err)
		}

		// Request to rename a symbol at a position where no symbol exists
		// The clean.go file doesn't have content at this position
		_, err = tools.RenameSymbol(ctx, suite.Client, filePath, 10, 10, "NewName")

		// Expect an error because there's no symbol at that position
		if err == nil {
			t.Errorf("Expected an error when renaming non-existent symbol, but got success")
		}

		// Save the error message for the snapshot
		errorMessage := err.Error()

		// Verify it mentions failing to rename
		if !strings.Contains(errorMessage, "failed to rename") {
			t.Errorf("Expected error message about failed rename but got: %s", errorMessage)
		}

		common.SnapshotTest(t, "go", "rename_symbol", "not_found", errorMessage)
	})
}
