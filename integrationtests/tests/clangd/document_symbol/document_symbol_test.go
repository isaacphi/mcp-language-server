package document_symbol_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDocumentSymbol tests the document_symbol (outline) tool with clangd.
// Opens the file so clangd can index it, then requests document symbols and compares to snapshot.
func TestDocumentSymbol(t *testing.T) {
	openFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		filesToOpen := []string{
			"src/main.cpp",
			"src/consumer.cpp",
		}
		for _, file := range filesToOpen {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			err := suite.Client.OpenFile(ctx, filePath)
			if err != nil {
				t.Logf("Note: Failed to open %s: %v", file, err)
			}
		}
		time.Sleep(10 * time.Second)
	}

	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	openFilesAndWait(suite, ctx)

	filePath := filepath.Join(suite.WorkspaceDir, "src/consumer.cpp")
	result, err := tools.DocumentSymbolOutline(ctx, suite.Client, filePath)
	if err != nil {
		t.Fatalf("DocumentSymbolOutline failed: %v", err)
	}

	common.SnapshotTest(t, "clangd", "document_symbol", "consumer_cpp", result)
}
