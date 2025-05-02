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

// TestReadDefinition tests the ReadDefinition tool with various Go type definitions
func TestReadDefinition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{
			name:         "Function",
			symbolName:   "FooBar",
			expectedText: "func FooBar()",
			snapshotName: "foobar",
		},
		{
			name:         "Struct",
			symbolName:   "TestStruct",
			expectedText: "type TestStruct struct",
			snapshotName: "struct",
		},
		{
			name:         "Method",
			symbolName:   "TestStruct.Method",
			expectedText: "func (t *TestStruct) Method()",
			snapshotName: "method",
		},
		{
			name:         "Interface",
			symbolName:   "TestInterface",
			expectedText: "type TestInterface interface",
			snapshotName: "interface",
		},
		{
			name:         "Type",
			symbolName:   "TestType",
			expectedText: "type TestType string",
			snapshotName: "type",
		},
		{
			name:         "Constant",
			symbolName:   "TestConstant",
			expectedText: "const TestConstant",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			symbolName:   "TestVariable",
			expectedText: "var TestVariable",
			snapshotName: "variable",
		},
		{
			name:         "Function",
			symbolName:   "TestFunction",
			expectedText: "func TestFunction()",
			snapshotName: "function",
		},
		{
			name:         "NotFound",
			symbolName:   "NotFound",
			expectedText: "not found",
			snapshotName: "not-found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "go", "definition", tc.snapshotName, result)
		})
	}
}
