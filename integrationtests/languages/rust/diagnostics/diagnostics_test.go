package diagnostics_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/rust/internal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics tests diagnostics functionality with the Rust language server
func TestDiagnostics(t *testing.T) {
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
	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		// Open all files and wait for rust-analyzer to index them
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "src/clean.rs")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, true, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "rust", "diagnostics", "clean", result)
	})

	// Test with a file containing an error
	t.Run("FileWithError", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		// Open all files and wait for rust-analyzer to index them
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "src/main.rs")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, true, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have diagnostics about unreachable code
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics but got none")
		}

		if !strings.Contains(result, "unreachable") {
			t.Errorf("Expected unreachable code error but got: %s", result)
		}

		common.SnapshotTest(t, "rust", "diagnostics", "unreachable", result)
	})

	// Test file dependency: file A (helper.rs) provides a function,
	// file B (consumer.rs) uses it, then modify A to break B
	t.Run("FileDependency", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		// Open all files and wait for rust-analyzer to index them
		openAllFilesAndWait(suite, ctx)

		// Ensure the relevant paths are accessible
		helperPath := filepath.Join(suite.WorkspaceDir, "src/helper.rs")
		consumerPath := filepath.Join(suite.WorkspaceDir, "src/consumer.rs")

		// Get initial diagnostics for consumer.rs
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, true, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Should have no diagnostics initially
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics initially but got: %s", result)
		}

		// Now modify the helper function to cause an error in the consumer
		modifiedHelperContent := `// Helper functions for testing

// A function that will be referenced from other files
pub fn helper_function(value: i32) -> String {
    String::from("hello world")
}
`
		// Write the modified content to the file
		err = suite.WriteFile("src/helper.rs", modifiedHelperContent)
		if err != nil {
			t.Fatalf("Failed to update helper.rs: %v", err)
		}

		// Explicitly notify the LSP server about the change
		helperURI := fmt.Sprintf("file://%s", helperPath)

		// Notify the LSP server about the file change
		err = suite.Client.NotifyChange(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to notify change to helper.rs: %v", err)
		}

		// Also send a didChangeWatchedFiles notification for coverage
		// This simulates what the watcher would do
		fileChangeParams := protocol.DidChangeWatchedFilesParams{
			Changes: []protocol.FileEvent{
				{
					URI:  protocol.DocumentUri(helperURI),
					Type: protocol.FileChangeType(protocol.Changed),
				},
			},
		}

		err = suite.Client.DidChangeWatchedFiles(ctx, fileChangeParams)
		if err != nil {
			t.Fatalf("Failed to send DidChangeWatchedFiles: %v", err)
		}

		// Wait for LSP to process the change
		time.Sleep(3 * time.Second)

		// Force reopen the consumer file to ensure LSP reevaluates it
		err = suite.Client.CloseFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to close consumer.rs: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to reopen consumer.rs: %v", err)
		}

		// Wait for diagnostics to be generated
		time.Sleep(3 * time.Second)

		// Check diagnostics again on consumer file - should now have an error
		result, err = tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, true, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed after dependency change: %v", err)
		}

		// Should have diagnostics now
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics after dependency change but got none")
		}

		// Should contain an error about function arguments
		if !strings.Contains(result, "argument") && !strings.Contains(result, "parameter") && !strings.Contains(result, "expected") {
			t.Errorf("Expected error about wrong arguments but got: %s", result)
		}

		common.SnapshotTest(t, "rust", "diagnostics", "dependency", result)
	})
}
