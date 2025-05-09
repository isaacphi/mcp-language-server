package diagnostics_test

// note: clangd doesn't support pull diagnostics (textdocument/diagnostic)
// see: https://github.com/clangd/clangd/issues/2108
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

// TestDiagnostics tests diagnostics functionality with the Clangd language server
func TestDiagnostics(t *testing.T) {
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
		time.Sleep(10 * time.Second)

	}

	// Test with a clean file
	t.Run("CleanFile", func(t *testing.T) {
		// Get a test suite with clean code
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		// Open all files and wait for clangd to index them
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "src/clean.cpp")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}

		// Verify we have no diagnostics
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("Expected no diagnostics but got: %s", result)
		}

		common.SnapshotTest(t, "clangd", "diagnostics", "clean", result)
	})

	// Test with a file containing an error
	t.Run("FileWithError", func(t *testing.T) {
		// Get a test suite with code that contains errors
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		// Open all files and wait for clangd to index them
		openAllFilesAndWait(suite, ctx)

		filePath := filepath.Join(suite.WorkspaceDir, "src/main.cpp")
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

		common.SnapshotTest(t, "clangd", "diagnostics", "unreachable", result)
	})
}
