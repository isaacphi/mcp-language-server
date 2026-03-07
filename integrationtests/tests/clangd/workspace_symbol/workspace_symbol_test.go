package workspace_symbol_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestWorkspaceSymbol tests the workspace/symbol MCP tool with clangd.
// Files are opened so clangd can index the project before querying symbols.
func TestWorkspaceSymbol(t *testing.T) {
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
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

	openAllFilesAndWait(suite, ctx)

	tests := []struct {
		name         string
		query        string
		snapshotName string
	}{
		{
			name:         "Query foo_bar",
			query:        "foo_bar",
			snapshotName: "foo_bar",
		},
		{
			name:         "Query helper",
			query:        "helper",
			snapshotName: "helper",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tools.WorkspaceSymbol(ctx, suite.Client, tc.query)
			if err != nil {
				t.Fatalf("WorkspaceSymbol failed: %v", err)
			}
			common.SnapshotTest(t, "clangd", "workspace_symbol", tc.snapshotName, result)
		})
	}
}
