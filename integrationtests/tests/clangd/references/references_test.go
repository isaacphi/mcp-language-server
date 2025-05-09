package references_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestFindReferences tests the FindReferences tool with C++ symbols
// that have references across different files
func TestFindReferences(t *testing.T) {
	// Helper function to open all files and wait for indexing
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		// Open one file so that clangd loads compiles commands and begins indexing
		filesToOpen := []string{
			"src/main.cpp",
		}

		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				// Don't fail the test, some files might not exist in certain tests
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}
		// Wait for indexing to complete. clangd won't index files until they are opened.
		time.Sleep(30 * time.Second)
	}

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second) // Increased timeout for clangd references
	defer cancel()

	// Open all files and wait for clangd to index them
	openAllFilesAndWait(suite, ctx)

	tests := []struct {
		name          string
		symbolName    string // The symbol to find references for. For methods, use Class::Method.
		fileHint      string // File where the symbol definition is likely located (optional, can speed up).
		lineHint      int    // Line number where the symbol is used/defined (optional, for focusing search).
		colHint       int    // Column number (optional).
		expectedText  string // Text expected in one of the reference locations.
		expectedFiles int    // Minimum number of files where references should be found.
		snapshotName  string
	}{
		{
			name:          "Function with references across files",
			symbolName:    "helperFunction", // used in main.cpp, consumer.cpp. Clangd seems to treat definitations as declarations, so the definition in helper.cpp is not included.
			fileHint:      "src/helper.cpp",
			lineHint:      7, // Definition line
			colHint:       6,
			expectedText:  "main.cpp", // Expect a reference in main.cpp
			expectedFiles: 2,          // main.cpp, consumer.cpp
			snapshotName:  "helper-function-references",
		},
		{
			name:          "Function with reference in same file",
			symbolName:    "foo_bar", // Defined and used in main.cpp
			fileHint:      "src/main.cpp",
			lineHint:      5, // Definition line
			colHint:       6,
			expectedText:  "main.cpp",
			expectedFiles: 1, // main.cpp (definition and usage)
			snapshotName:  "foobar-function-references",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the FindReferences tool
			result, err := tools.FindReferences(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to find references for %s: %v. Result: %s", tc.symbolName, err, result)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("References for %s do not contain expected text %q in result: %s", tc.symbolName, tc.expectedText, result)
			}

			// Count how many different files are mentioned in the result
			fileCount := countFilesInResult(result, suite.WorkspaceDir)
			if fileCount < tc.expectedFiles {
				t.Errorf("Expected references for %s in at least %d files, but found in %d files. Result:\n%s",
					tc.symbolName, tc.expectedFiles, fileCount, result)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "clangd", "references", tc.snapshotName, result)
		})
	}
}

// countFilesInResult counts the number of unique files mentioned in the result
// It now normalizes paths relative to the workspace for more robust counting.
func countFilesInResult(result string, workspaceDir string) int {
	fileMap := make(map[string]bool)

	// Normalize workspaceDir path for string matching
	normalizedWorkspaceDir := filepath.ToSlash(workspaceDir)

	lines := strings.Split(result, "\n")
	for _, line := range lines {
		// A line representing a file path typically contains ".cpp" or ".hpp"
		// and is an absolute path or relative to the workspace view.
		if strings.Contains(line, ".cpp") || strings.Contains(line, ".hpp") {
			// Attempt to extract a clean, relative path or a consistent absolute path part
			var pathKey string
			if strings.Contains(line, normalizedWorkspaceDir) {
				relPath, err := filepath.Rel(normalizedWorkspaceDir, strings.Fields(line)[0]) // Assuming path is the first part
				if err == nil {
					pathKey = filepath.ToSlash(relPath)
				} else {
					// Fallback if Rel fails, use a snippet after a known part
					pathKey = extractPathSegment(line)
				}
			} else {
				// If not a full workspace path, it might be a relative path already or just a filename
				pathKey = extractPathSegment(line) // A more generic extraction
			}
			if pathKey != "" {
				fileMap[pathKey] = true
			}
		}
	}
	return len(fileMap)
}

// extractPathSegment tries to get a consistent file path identifier from a line of text.
func extractPathSegment(line string) string {
	// Look for common C++ file extensions
	var ext string
	if strings.Contains(line, ".cpp") {
		ext = ".cpp"
	} else if strings.Contains(line, ".hpp") {
		ext = ".hpp"
	} else {
		return "" // Not a C++ source/header file line we can easily parse
	}

	fields := strings.Fields(line)
	for _, field := range fields {
		if strings.HasSuffix(field, ext) {
			// Attempt to clean common prefixes like "uri: file://" or line numbers
			cleanedPath := strings.TrimPrefix(field, "uri:")
			cleanedPath = strings.TrimPrefix(cleanedPath, "file://")
			// Remove trailing colons or line/char numbers like ":10:5"
			parts := strings.Split(cleanedPath, ":")
			if len(parts) > 0 {
				return parts[0] // Return the part before the first colon, assuming it's the path
			}
			return cleanedPath
		}
	}
	return ""
}
