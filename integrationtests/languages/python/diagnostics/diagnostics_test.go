package diagnostics_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/python/internal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics tests diagnostics functionality with the Python language server
func TestDiagnostics(t *testing.T) {
	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Check diagnostics for clean.py, which shouldn't have any errors
		filePath := filepath.Join(suite.WorkspaceDir, "clean.py")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "python", "diagnostics", "clean", result)
	})

	// Test with a file containing errors
	t.Run("FileWithErrors", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		// Wait for diagnostics to be generated
		time.Sleep(2 * time.Second)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Check diagnostics for error_file.py, which contains deliberate errors
		filePath := filepath.Join(suite.WorkspaceDir, "error_file.py")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have diagnostics
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics but got none")
		}

		// Check for specific error types that should be detected
		if !strings.Contains(result, "Type") && !strings.Contains(result, "type") &&
			!strings.Contains(result, "undefined") && !strings.Contains(result, "Undefined") {
			t.Errorf("Expected type errors or undefined variable errors but got: %s", result)
		}

		common.SnapshotTest(t, "python", "diagnostics", "errors", result)
	})

	// Test file dependency: helper.py provides a function,
	// consumer_clean.py uses it, then modify helper.py to break consumer_clean.py
	t.Run("FileDependency", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		// Wait for initial diagnostics to be generated
		time.Sleep(2 * time.Second)

		// Create context
		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Ensure both helper.py and consumer_clean.py are open in the LSP
		helperPath := filepath.Join(suite.WorkspaceDir, "helper.py")
		consumerPath := filepath.Join(suite.WorkspaceDir, "consumer_clean.py")

		err := suite.Client.OpenFile(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to open helper.py: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to open consumer_clean.py: %v", err)
		}

		// Wait for files to be processed
		time.Sleep(2 * time.Second)

		// Get initial diagnostics for consumer_clean.py
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Should have no diagnostics initially
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics initially but got: %s", result)
		}

		// Now modify the helper function to cause an error in the consumer
		modifiedHelperContent := `"""Helper module that provides utility functions."""

from typing import List, Dict


def helper_function(name: str, age: int) -> str:
    """A helper function that formats a greeting message.
    
    Args:
        name: The name to greet
        age: The age of the person
        
    Returns:
        A formatted greeting message
    """
    return f"Hello, {name}! You are {age} years old."


def get_items() -> List[str]:
    """Get a list of sample items.
    
    Returns:
        A list of sample strings
    """
    return ["apple", "banana", "orange", "grape"]`

		// Write the modified content to the file
		err = suite.WriteFile("helper.py", modifiedHelperContent)
		if err != nil {
			t.Fatalf("Failed to update helper.py: %v", err)
		}

		// Explicitly notify the LSP server about the change
		helperURI := fmt.Sprintf("file://%s", helperPath)

		// Notify the LSP server about the file change
		err = suite.Client.NotifyChange(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to notify change to helper.py: %v", err)
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
			t.Fatalf("Failed to close consumer_clean.py: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to reopen consumer_clean.py: %v", err)
		}

		// Wait for diagnostics to be generated
		time.Sleep(3 * time.Second)

		// Check diagnostics again on consumer file - should now have an error
		result, err = tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed after dependency change: %v", err)
		}

		// Should have diagnostics now
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics after dependency change but got none")
		}

		// Should contain an error about function arguments
		if !strings.Contains(result, "argument") && !strings.Contains(result, "parameter") &&
			!strings.Contains(result, "missing") && !strings.Contains(result, "required") {
			t.Errorf("Expected error about wrong arguments but got: %s", result)
		}

		common.SnapshotTest(t, "python", "diagnostics", "dependency", result)
	})
}
