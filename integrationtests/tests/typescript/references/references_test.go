package references_test

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

// TestFindReferences tests the FindReferences tool with TypeScript symbols
// that have references across different files
func TestFindReferences(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	// First open all files to ensure TypeScript server indexes everything
	filesToOpen := []string{
		"main.ts",
		"helper.ts",
		"consumer.ts",
		"another_consumer.ts",
	}

	for _, file := range filesToOpen {
		filePath := filepath.Join(suite.WorkspaceDir, file)
		err := suite.Client.OpenFile(ctx, filePath)
		if err != nil {
			// Don't fail the test, just log it
			t.Logf("Note: Failed to open %s: %v", file, err)
		}
	}

	// Give TypeScript server time to process files
	time.Sleep(3 * time.Second)

	tests := []struct {
		name          string
		symbolName    string
		expectedText  string
		expectedFiles int // Number of files where references should be found
		snapshotName  string
	}{
		{
			name:          "Function with references across files",
			symbolName:    "SharedFunction",
			expectedText:  "ConsumerFunction",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-function",
		},
		{
			name:          "Function with reference in same file",
			symbolName:    "TestFunction",
			expectedText:  "main()",
			expectedFiles: 1, // main.ts
			snapshotName:  "test-function",
		},
		{
			name:          "Class with references across files",
			symbolName:    "SharedClass",
			expectedText:  "SharedClass",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-class",
		},
		{
			name:          "Method with references across files",
			symbolName:    "helperMethod", // Method name only
			expectedText:  "helperMethod",
			expectedFiles: 1, // consumer.ts
			snapshotName:  "class-method",
		},
		{
			name:          "Interface with references across files",
			symbolName:    "SharedInterface",
			expectedText:  "SharedInterface",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-interface",
		},
		{
			name:          "Interface method with references",
			symbolName:    "getName", // Method name only
			expectedText:  "getName",
			expectedFiles: 2, // Helper file defines it, consumer uses it
			snapshotName:  "interface-method",
		},
		{
			name:          "Constant with references across files",
			symbolName:    "SharedConstant",
			expectedText:  "SharedConstant",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-constant",
		},
		{
			name:          "Type with references across files",
			symbolName:    "SharedType",
			expectedText:  "SharedType",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-type",
		},
		{
			name:          "Enum with references across files",
			symbolName:    "SharedEnum",
			expectedText:  "SharedEnum",
			expectedFiles: 2, // consumer.ts and another_consumer.ts
			snapshotName:  "shared-enum",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the FindReferences tool
			result, err := tools.FindReferences(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to find references: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("References do not contain expected text: %s", tc.expectedText)
			}

			// Count how many different files are mentioned in the result
			fileCount := countFilesInResult(result)
			if fileCount < tc.expectedFiles {
				t.Errorf("Expected references in at least %d files, but found in %d files",
					tc.expectedFiles, fileCount)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "typescript", "references", tc.snapshotName, result)
		})
	}
}

// countFilesInResult counts the number of unique files mentioned in the result
func countFilesInResult(result string) int {
	fileMap := make(map[string]bool)

	// Any line containing "workspace" and ".ts" is a file path
	// but filter out lines that are just headers
	for line := range strings.SplitSeq(result, "\n") {
		if strings.Contains(line, "workspace") && strings.Contains(line, ".ts") {
			// Avoid counting section headers and focus on actual file paths
			if !strings.Contains(line, "References in File") && !strings.Contains(line, "Symbol:") {
				fileMap[line] = true
			}
		}
	}

	return len(fileMap)
}
