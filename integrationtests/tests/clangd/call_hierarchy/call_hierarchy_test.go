package call_hierarchy_test

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

// TestGetCallHierarchy tests the get_call_hierarchy tool: full incoming call tree
// with cycle detection. Uses position-based API. Tests helperFunction() which is called by main() and method().
func TestGetCallHierarchy(t *testing.T) {
	openAllFilesAndWait := func(suite *common.TestSuite, ctx context.Context) {
		for _, file := range []string{"src/main.cpp", "src/helper.cpp", "src/consumer.cpp"} {
			filePath := filepath.Join(suite.WorkspaceDir, file)
			_ = suite.Client.OpenFile(ctx, filePath)
		}
		time.Sleep(30 * time.Second)
	}

	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 60*time.Second)
	defer cancel()
	openAllFilesAndWait(suite, ctx)

	filePath := filepath.Join(suite.WorkspaceDir, "src/helper.cpp")
	result, err := tools.GetFullCallHierarchy(ctx, suite.Client, filePath, 7, 6)
	if err != nil {
		t.Fatalf("GetFullCallHierarchy failed: %v", err)
	}
	if !strings.Contains(result, "main") {
		t.Errorf("Result should contain main (caller of helperFunction). Result:\n%s", result)
	}
	common.SnapshotTest(t, "clangd", "call_hierarchy", "get-call-hierarchy-helper-incoming", result)
}
