package diagnostics_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/typescript/internal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics tests diagnostics functionality with the TypeScript language server
func TestDiagnostics(t *testing.T) {
	// Helper function to open all files and wait for indexing
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
	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Open all files and wait for TypeScript server to index them
		openAllFilesAndWait(suite, ctx)

		// Target the clean file
		filePath := filepath.Join(suite.WorkspaceDir, "clean.ts")

		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "typescript", "diagnostics", "clean", result)
	})

	// Test with a file containing an error
	t.Run("FileWithError", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Open all files and wait for TypeScript server to index them
		openAllFilesAndWait(suite, ctx)

		// Create a file with an error
		fileContent := `
// File with a type error
function errorFunction(x: number): string {
  return x; // Error: Type 'number' is not assignable to type 'string'
}

const result = errorFunction(42);
console.log(result);
`
		testFilePath := filepath.Join(suite.WorkspaceDir, "error.ts")
		err := suite.WriteFile("error.ts", fileContent)
		if err != nil {
			t.Fatalf("Failed to create error.ts: %v", err)
		}

		// Open the file to trigger diagnostics
		err = suite.Client.OpenFile(ctx, testFilePath)
		if err != nil {
			t.Fatalf("Failed to open error.ts: %v", err)
		}

		// Wait for diagnostics to be generated
		time.Sleep(3 * time.Second)

		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, testFilePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have diagnostics about the type error
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected diagnostics but got none")
		}

		if !strings.Contains(result, "Type 'number' is not assignable to type 'string'") {
			t.Errorf("Expected type error but got: %s", result)
		}

		common.SnapshotTest(t, "typescript", "diagnostics", "type-error", result)
	})

	// Test file dependency: file A (helper.ts) provides a function,
	// file B (consumer.ts) uses it, then modify A to break B
	t.Run("FileDependency", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
		defer cancel()

		// Open all files and wait for TypeScript server to index them
		openAllFilesAndWait(suite, ctx)

		// Ensure the relevant paths are accessible
		helperPath := filepath.Join(suite.WorkspaceDir, "helper.ts")
		consumerPath := filepath.Join(suite.WorkspaceDir, "consumer.ts")

		// Get initial diagnostics for consumer.ts
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, consumerPath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Should have no diagnostics initially
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics initially but got: %s", result)
		}

		// Now modify the helper function to cause an error in the consumer
		modifiedHelperContent := `// Helper functions and types that are used across files

// SharedFunction with references across files - now requires a parameter
export function SharedFunction(required: string): string {
  return "helper function: " + required;
}

// SharedInterface with methods
export interface SharedInterface {
  getName(): string;
  getValue(): number;
}

// SharedClass implementing the interface
export class SharedClass implements SharedInterface {
  private name: string;

  constructor(name: string) {
    this.name = name;
  }
  
  getName(): string {
    return this.name;
  }
  
  getValue(): number {
    return 42;
  }
  
  helperMethod(): void {
    console.log("Helper method called");
  }
}

// SharedType referenced across files
export type SharedType = string | number;

// SharedConstant referenced across files
export const SharedConstant = "SHARED_VALUE";

// SharedEnum referenced across files
export enum SharedEnum {
  ONE = "one",
  TWO = "two",
  THREE = "three"
}`

		// Write the modified content to the file
		err = suite.WriteFile("helper.ts", modifiedHelperContent)
		if err != nil {
			t.Fatalf("Failed to update helper.ts: %v", err)
		}

		// Explicitly notify the LSP server about the change
		helperURI := fmt.Sprintf("file://%s", helperPath)

		// Notify the LSP server about the file change
		err = suite.Client.NotifyChange(ctx, helperPath)
		if err != nil {
			t.Fatalf("Failed to notify change to helper.ts: %v", err)
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
			t.Fatalf("Failed to close consumer.ts: %v", err)
		}

		err = suite.Client.OpenFile(ctx, consumerPath)
		if err != nil {
			t.Fatalf("Failed to reopen consumer.ts: %v", err)
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

		// Should contain an error about function arguments or expected parameters
		expectedErrorPhrases := []string{
			"argument", "parameter", "expected", "required", "call",
		}

		found := false
		for _, phrase := range expectedErrorPhrases {
			if strings.Contains(strings.ToLower(result), phrase) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected error about arguments/parameters but got: %s", result)
		}

		common.SnapshotTest(t, "typescript", "diagnostics", "dependency", result)
	})
}
