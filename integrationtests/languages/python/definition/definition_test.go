package definition_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/languages/python/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestReadDefinition tests the ReadDefinition tool with various Python type definitions
func TestReadDefinition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		symbolName   string
		expectedText string
		snapshotName string
		mayFail      bool // Some symbols like methods might not be found by pyright
	}{
		{
			name:         "Function",
			symbolName:   "test_function",
			expectedText: "def test_function",
			snapshotName: "function",
		},
		{
			name:         "Class",
			symbolName:   "TestClass",
			expectedText: "class TestClass",
			snapshotName: "class",
		},
		{
			name:         "Method",
			symbolName:   "test_method", // Try just the method name
			expectedText: "def test_method",
			snapshotName: "method",
			mayFail:      true, // This may fail as pyright might not find methods directly
		},
		{
			name:         "StaticMethod",
			symbolName:   "static_method", // Try just the method name
			expectedText: "def static_method",
			snapshotName: "static-method",
			mayFail:      true, // This may fail as pyright might not find static methods directly
		},
		{
			name:         "Constant",
			symbolName:   "TEST_CONSTANT",
			expectedText: "TEST_CONSTANT",
			snapshotName: "constant",
		},
		{
			name:         "Variable",
			symbolName:   "test_variable",
			expectedText: "test_variable",
			snapshotName: "variable",
		},
		{
			name:         "DerivedClass",
			symbolName:   "DerivedClass",
			expectedText: "class DerivedClass",
			snapshotName: "derived-class",
		},
		{
			name:         "MultipleFiles",
			symbolName:   "SameName",
			expectedText: "class SameName",
			snapshotName: "same-name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the ReadDefinition tool
			result, err := tools.ReadDefinition(ctx, suite.Client, tc.symbolName, true)
			if err != nil {
				t.Fatalf("Failed to read definition: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) && !tc.mayFail {
				t.Errorf("Definition does not contain expected text: %s", tc.expectedText)
			}

			// Skip further validation if we know this test might fail but
			// continue with snapshot testing for future reference
			if tc.mayFail && strings.Contains(result, "not found") {
				t.Logf("Symbol might not be directly findable by pyright: %s", tc.symbolName)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "python", "definition", tc.snapshotName, result)
		})
	}
}
