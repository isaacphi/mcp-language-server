package references_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/python/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestFindReferences tests the FindReferences tool with Python symbols
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
			symbolName:    "helper_function",
			expectedText:  "helper_function",
			expectedFiles: 2, // consumer.py and another_consumer.py
			snapshotName:  "helper-function",
		},
		{
			name:          "Class with references across files",
			symbolName:    "SharedClass",
			expectedText:  "SharedClass",
			expectedFiles: 2, // consumer.py and another_consumer.py
			snapshotName:  "shared-class",
		},
		{
			name:          "Method with references across files",
			symbolName:    "get_name", // Use the unqualified method name for Python
			expectedText:  "get_name",
			expectedFiles: 2, // consumer.py and another_consumer.py
			snapshotName:  "class-method",
		},
		{
			name:          "Interface with references across files",
			symbolName:    "SharedInterface",
			expectedText:  "SharedInterface",
			expectedFiles: 1, // consumer.py
			snapshotName:  "shared-interface",
		},
		{
			name:          "Interface method with references",
			symbolName:    "process", // Use the unqualified method name for Python
			expectedText:  "process",
			expectedFiles: 1, // consumer.py
			snapshotName:  "interface-method",
		},
		{
			name:          "Constant with references across files",
			symbolName:    "SHARED_CONSTANT",
			expectedText:  "SHARED_CONSTANT",
			expectedFiles: 2, // consumer.py and another_consumer.py
			snapshotName:  "shared-constant",
		},
		{
			name:          "Enum-like class with references across files",
			symbolName:    "Color",
			expectedText:  "Color",
			expectedFiles: 2, // consumer.py and another_consumer.py
			snapshotName:  "color-enum",
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
			common.SnapshotTest(t, "python", "references", tc.snapshotName, result)
		})
	}
}

// countFilesInResult counts the number of unique files mentioned in the result
func countFilesInResult(result string) int {
	fileMap := make(map[string]bool)

	// Any line containing "workspace" and ".py" is a file path
	for line := range strings.SplitSeq(result, "\n") {
		if strings.Contains(line, "workspace") && strings.Contains(line, ".py") {
			if !strings.Contains(line, "References in File") {
				fileMap[line] = true
			}
		}
	}

	return len(fileMap)
}
