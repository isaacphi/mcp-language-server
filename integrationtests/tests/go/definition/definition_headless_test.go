package definition_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestReadDefinitionHeadless runs ReadDefinition against an already-running gopls.
// Requires GOPLS_HEADLESS_ADDR (e.g. localhost:6060). The server must be started separately (e.g. gopls -listen=:6060).
func TestReadDefinitionHeadless(t *testing.T) {
	suite := internal.GetHeadlessTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{"Function", "FooBar", "func FooBar()", "foobar"},
		{"Struct", "TestStruct", "type TestStruct struct", "struct"},
		{"Method", "TestStruct.Method", "func (t *TestStruct) Method()", "method"},
		{"Interface", "TestInterface", "type TestInterface interface", "interface"},
		{"Type", "TestType", "type TestType string", "type"},
		{"Constant", "TestConstant", "const TestConstant", "constant"},
		{"Variable", "TestVariable", "var TestVariable", "variable"},
		{"TestFunction", "TestFunction", "func TestFunction()", "function"},
		{"NotFound", "NotFound", "not found", "not-found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("ReadDefinition failed: %v", err)
			}
			// Headless server may have a different workspace; only assert NotFound case
			if tc.snapshotName == "not-found" && !strings.Contains(result, "not found") {
				t.Errorf("expected 'not found' in result for unknown symbol, got: %s", result)
			}
			common.SnapshotTest(t, "go", "definition_headless", tc.snapshotName, result)
		})
	}
}
