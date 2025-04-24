package diagnostics_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics tests diagnostics functionality with the Go language server
func TestDiagnostics(t *testing.T) {
	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "clean.go")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "go", "diagnostics", "clean", result)
	})

	// Test with a file containing an error
	t.Run("FileWithError", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		// Wait for diagnostics to be generated
		time.Sleep(2 * time.Second)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "main.go")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
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

		common.SnapshotTest(t, "go", "diagnostics", "unreachable", result)
	})

	// Test file dependency: file A (helper.go) provides a function,
	// file B (consumer.go) uses it, then modify A to break B
	t.Run("FileDependency", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		// Wait for initial diagnostics to be generated
		time.Sleep(2 * time.Second)

		// Verify consumer.go is clean initially
		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Ensure both helper.go and consumer.go are open in the LSP
		helperPath := filepath.Join(suite.WorkspaceDir, "helper.go")
		consumerPath := filepath.Join(suite.WorkspaceDir, "consumer.go")

		err := suite.Client.OpenFile(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to open helper.go: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to open consumer.go: %v", err)
		}

		// Wait for files to be processed
		time.Sleep(2 * time.Second)

		// Get initial diagnostics for consumer.go
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Should have no diagnostics initially
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics initially but got: %s", result)
		}

		// Now modify the helper function to cause an error in the consumer
		modifiedHelperContent := `package main

// HelperFunction now requires an int parameter
func HelperFunction(value int) string {
	return "hello world"
}
`
		// Write the modified content to the file
		err = suite.WriteFile("helper.go", modifiedHelperContent)
		if err != nil {
			t.Fatalf("Failed to update helper.go: %v", err)
		}

		// Explicitly notify the LSP server about the change
		helperURI := fmt.Sprintf("file://%s", helperPath)

		// Notify the LSP server about the file change
		err = suite.Client.NotifyChange(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to notify change to helper.go: %v", err)
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
			t.Fatalf("Failed to close consumer.go: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to reopen consumer.go: %v", err)
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
		if !strings.Contains(result, "argument") && !strings.Contains(result, "parameter") {
			t.Errorf("Expected error about wrong arguments but got: %s", result)
		}

		common.SnapshotTest(t, "go", "diagnostics", "dependency", result)
	})
}
