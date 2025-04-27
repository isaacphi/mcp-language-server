package references_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestFindReferences tests the FindReferences tool with Go symbols
// that have references across different files
func TestFindReferences(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name          string
		symbolName    string
		expectedText  string
		expectedFiles int // Number of files where references should be found
		snapshotName  string
	}{
		{
			name:          "Function with references across files",
			symbolName:    "HelperFunction",
			expectedText:  "ConsumerFunction",
			expectedFiles: 2, // consumer.go and another_consumer.go
			snapshotName:  "helper-function",
		},
		{
			name:          "Function with reference in same file",
			symbolName:    "FooBar",
			expectedText:  "main()",
			expectedFiles: 1, // main.go
			snapshotName:  "foobar-function",
		},
		{
			name:          "Struct with references across files",
			symbolName:    "SharedStruct",
			expectedText:  "ConsumerFunction",
			expectedFiles: 2, // consumer.go and another_consumer.go
			snapshotName:  "shared-struct",
		},
		{
			name:          "Method with references across files",
			symbolName:    "SharedStruct.Method",
			expectedText:  "s.Method()",
			expectedFiles: 1, // consumer.go
			snapshotName:  "struct-method",
		},
		{
			name:          "Interface with references across files",
			symbolName:    "SharedInterface",
			expectedText:  "var iface SharedInterface",
			expectedFiles: 2, // consumer.go and another_consumer.go
			snapshotName:  "shared-interface",
		},
		{
			name:          "Interface method with references",
			symbolName:    "SharedInterface.GetName",
			expectedText:  "iface.GetName()",
			expectedFiles: 1, // consumer.go
			snapshotName:  "interface-method",
		},
		{
			name:          "Constant with references across files",
			symbolName:    "SharedConstant",
			expectedText:  "SharedConstant",
			expectedFiles: 2, // consumer.go and another_consumer.go
			snapshotName:  "shared-constant",
		},
		{
			name:          "Type with references across files",
			symbolName:    "SharedType",
			expectedText:  "SharedType",
			expectedFiles: 2, // consumer.go and another_consumer.go
			snapshotName:  "shared-type",
		},
		{
			name:          "References not found",
			symbolName:    "NotFound",
			expectedText:  "No references found for symbol:",
			expectedFiles: 0,
			snapshotName:  "not-found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the FindReferences tool
			result, err := tools.FindReferences(ctx, suite.Client, tc.symbolName, true)
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
			common.SnapshotTest(t, "go", "references", tc.snapshotName, result)
		})
	}
}

// countFilesInResult counts the number of unique files mentioned in the result
func countFilesInResult(result string) int {
	fileMap := make(map[string]bool)

	// Any line containing "workspace" and ".go" is a file path
	for line := range strings.SplitSeq(result, "\n") {
		if strings.Contains(line, "workspace") && strings.Contains(line, ".go") {
			if !strings.Contains(line, "References in File") {
				fileMap[line] = true
			}
		}
	}

	return len(fileMap)
}
