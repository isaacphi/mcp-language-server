package references_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestFindReferencesHeadless runs FindReferences against an already-running gopls.
// Requires GOPLS_HEADLESS_ADDR (e.g. localhost:6060). The server must be started separately (e.g. gopls -listen=:6060).
func TestFindReferencesHeadless(t *testing.T) {
	suite := internal.GetHeadlessTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name          string
		symbolName    string
		expectedText  string
		expectedFiles int
		snapshotName  string
	}{
		{"Function across files", "HelperFunction", "ConsumerFunction", 2, "helper-function"},
		{"Function same file", "FooBar", "main()", 1, "foobar-function"},
		{"Struct across files", "SharedStruct", "ConsumerFunction", 2, "shared-struct"},
		{"Method", "SharedStruct.Method", "s.Method()", 1, "struct-method"},
		{"Interface", "SharedInterface", "var iface SharedInterface", 2, "shared-interface"},
		{"Interface method", "SharedInterface.GetName", "iface.GetName()", 1, "interface-method"},
		{"Constant", "SharedConstant", "SharedConstant", 2, "shared-constant"},
		{"Type", "SharedType", "SharedType", 2, "shared-type"},
		{"NotFound", "NotFound", "No references found for symbol:", 0, "not-found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tools.FindReferences(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("FindReferences failed: %v", err)
			}
			// Headless server may have a different workspace; only assert NotFound case
			if tc.snapshotName == "not-found" && !strings.Contains(result, "No references found") {
				t.Errorf("expected 'No references found' for unknown symbol, got: %s", result)
			}
			common.SnapshotTest(t, "go", "references_headless", tc.snapshotName, result)
		})
	}
}
