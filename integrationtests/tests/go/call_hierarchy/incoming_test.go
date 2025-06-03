package callhierarchy_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

func TestIncomingCalls(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		symbolName   string
		depth        int
		expectedText string
		snapshotName string
	}{
		{
			name:         "Function with calls in same file",
			symbolName:   "FooBar",
			expectedText: ": main",
			depth:        5,
			snapshotName: "incoming-same-file",
		},
		{
			name:         "Function with calls in other file",
			symbolName:   "HelperFunction",
			depth:        5,
			expectedText: "ConsumerFunction",
			snapshotName: "incoming-other-file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the GetIncomingCalls tool
			result, err := tools.GetIncomingCalls(ctx, suite.Client, tc.symbolName, tc.depth)
			if err != nil {
				t.Fatalf("Failed to find incoming calls: %v", err)
			}

			// Check that the result contains relevant information
			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Incoming calls do not contain expected text: %s", tc.expectedText)
			}

			// Use snapshot testing to verify exact output
			common.SnapshotTest(t, "go", "call_hierarchy", tc.snapshotName, result)
		})
	}

}
