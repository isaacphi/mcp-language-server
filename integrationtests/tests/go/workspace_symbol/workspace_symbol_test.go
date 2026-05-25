package workspace_symbol_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestWorkspaceSymbol exercises the new workspace_symbol tool against
// gopls. The query is forwarded to LSP `workspace/symbol`, which
// gopls answers with a fuzzy-matched symbol list scoped to the
// current workspace.
func TestWorkspaceSymbol(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedText string // substring that MUST appear in the result
		snapshotName string
	}{
		{
			name:         "ExactType",
			query:        "SharedStruct",
			expectedText: "SharedStruct",
			snapshotName: "exact-type",
		},
		{
			name:         "ExactInterface",
			query:        "SharedInterface",
			expectedText: "SharedInterface",
			snapshotName: "exact-interface",
		},
		{
			name:         "ExactConstant",
			query:        "SharedConstant",
			expectedText: "SharedConstant",
			snapshotName: "exact-constant",
		},
		{
			// `AnotherConsumer` exists only in the test workspace
			// fixture and has no substring overlap with any Go stdlib
			// symbol. A `TestStruct` or `Method` query would fuzzy-
			// match `stringStruct` / `Method` in runtime/reflect/abi,
			// so the snapshot would include Go-version-specific paths
			// like `/opt/homebrew/Cellar/go/1.26.2/...` that break on
			// any other machine.
			name:         "WorkspaceUniqueType",
			query:        "AnotherConsumer",
			expectedText: "AnotherConsumer",
			snapshotName: "workspace-unique-type",
		},
		{
			name:         "NoMatch",
			query:        "ThisSymbolDefinitelyDoesNotExistAnywhere",
			expectedText: "No symbols found",
			snapshotName: "no-match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
			defer cancel()

			result, err := tools.WorkspaceSymbol(ctx, suite.Client, tt.query)
			if err != nil {
				t.Fatalf("WorkspaceSymbol failed: %v", err)
			}

			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Expected workspace_symbol result to contain %q but got:\n%s",
					tt.expectedText, result)
			}

			common.SnapshotTest(t, "go", "workspace_symbol", tt.snapshotName, result)
		})
	}
}

// TestWorkspaceSymbol_EmptyQuery asserts the validation gate fires
// BEFORE the LSP call — empty / whitespace queries get rejected
// with a clear error instead of being forwarded to gopls (which
// would return every symbol in the workspace, swamping the result).
func TestWorkspaceSymbol_EmptyQuery(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	cases := []string{"", "   ", "\t\n"}
	for _, q := range cases {
		_, err := tools.WorkspaceSymbol(ctx, suite.Client, q)
		if err == nil {
			t.Errorf("Expected error for empty query %q, got nil", q)
		}
	}
}
