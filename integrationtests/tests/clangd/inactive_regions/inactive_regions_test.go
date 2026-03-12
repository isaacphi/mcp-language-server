package inactive_regions_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestInactiveRegions verifies clangd's textDocument/inactiveRegions extension via the tools helper.
func TestInactiveRegions(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
	defer cancel()

	// Open a file to ensure clangd loads compile commands and starts indexing.
	indexFile := filepath.Join(suite.WorkspaceDir, "src/main.cpp")
	if err := suite.Client.OpenFile(ctx, indexFile); err != nil {
		t.Logf("Note: failed to open %s for indexing: %v", indexFile, err)
	}

	// Give clangd some time to index and compute inactive regions.
	time.Sleep(5 * time.Second)

	targetFile := filepath.Join(suite.WorkspaceDir, "src/another_consumer.cpp")

	result, err := tools.GetInactiveRegions(ctx, suite.Client, targetFile)
	if err != nil {
		t.Fatalf("GetInactiveRegions failed: %v", err)
	}

	// Snapshot under clangd/inactive_regions.
	common.SnapshotTest(t, "clangd", "inactive_regions", "another-consumer", result)
}

