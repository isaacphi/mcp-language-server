package definition_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/typescript/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestReadDefinition tests the ReadDefinition tool with various TypeScript type definitions
func TestReadDefinition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	// Open the main.ts file to help the TypeScript server recognize the project
	err := suite.Client.OpenFile(ctx, suite.WorkspaceDir+"/main.ts")
	if err != nil {
		t.Fatalf("Failed to open main.ts: %v", err)
	}

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
	}{
		{
			name:         "Function",
			symbolName:   "TestFunction",
			expectedText: "function TestFunction()",
			snapshotName: "function",
		},
		{
			name:         "Interface",
			symbolName:   "TestInterface",
			expectedText: "interface TestInterface",
			snapshotName: "interface",
		},
		{
			name:         "Class",
			symbolName:   "TestClass",
			expectedText: "class TestClass",
			snapshotName: "class",
		},
		{
			name:         "Type",
			symbolName:   "TestType",
			expectedText: "type TestType",
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
			expectedText: "const TestVariable",
			snapshotName: "variable",
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
			common.SnapshotTest(t, "typescript", "definition", tc.snapshotName, result)
		})
	}
}
