package call_hierarchy_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestCallHierarchy exercises the new call_hierarchy tool against
// gopls. The cursor MUST be on the function/method name itself —
// gopls rejects prepareCallHierarchy at other positions.
//
// The test workspace has SharedStruct.Method() (types.go:14) which
// is called from elsewhere — gopls should return both incoming
// (callers) and outgoing (callees, if any) edges.
func TestCallHierarchy(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		line         int
		column       int
		expectedText string // substring expected in the result
		snapshotName string
	}{
		{
			// Cursor on `Method` (SharedStruct.Method name).
			name:         "OnMethodName",
			file:         "types.go",
			line:         14,
			column:       24, // `Method` token in `func (s *SharedStruct) Method() string`
			expectedText: "identifier:",
			snapshotName: "on-method-name",
		},
		{
			// Cursor on `GetName` (SharedStruct.GetName) — a method
			// that satisfies the SharedInterface contract. Picked
			// over `Process` because the latter calls fmt.Printf, and
			// gopls's call_hierarchy then surfaces a `callee[0]:` row
			// pointing at the Go stdlib (`/usr/local/go/src/fmt/...`
			// on Linux, `/opt/homebrew/Cellar/go/.../fmt/...` on
			// macOS). Either path would make the snapshot non-
			// portable across CI runners. GetName has no callees so
			// the snapshot only contains workspace paths.
			name:         "OnInterfaceImpl",
			file:         "types.go",
			line:         37,
			column:       24,
			expectedText: "identifier:",
			snapshotName: "on-interface-impl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite := internal.GetTestSuite(t)

			ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
			defer cancel()

			filePath := filepath.Join(suite.WorkspaceDir, tt.file)
			if err := suite.Client.OpenFile(ctx, filePath); err != nil {
				t.Fatalf("Failed to open %s: %v", tt.file, err)
			}

			result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, tt.line, tt.column)
			if err != nil {
				t.Fatalf("GetCallHierarchy failed: %v", err)
			}

			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Expected call_hierarchy result to contain %q but got:\n%s",
					tt.expectedText, result)
			}

			common.SnapshotTest(t, "go", "call_hierarchy", tt.snapshotName, result)
		})
	}
}

// TestCallHierarchy_NonFunction covers the cursor-on-non-function
// path. LSP spec says prepareCallHierarchy SHOULD return null/empty
// for invalid positions, but gopls instead returns a JSON-RPC error
// (`SharedConstant is not a function`). Either shape is acceptable
// from the agent's perspective; this test asserts the tool either
// surfaces the fallback marker OR returns an error containing the
// failure reason — both are valid, language-server-dependent
// behaviors.
func TestCallHierarchy_NonFunction(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "types.go")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open types.go: %v", err)
	}

	// Cursor on `SharedConstant` (a const, not a function).
	result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 25, 7)
	if err != nil {
		// gopls path: LSP-level error surfaces. The agent sees the
		// reason in the error message — acceptable behavior.
		if !strings.Contains(err.Error(), "not a function") &&
			!strings.Contains(err.Error(), "no symbol") {
			t.Fatalf("Expected an 'is not a function' / 'no symbol' style "+
				"error, got: %v", err)
		}
		return
	}
	// Spec-compliant path: empty list → handler surfaces the fallback.
	if !strings.Contains(result, "No call hierarchy") {
		t.Errorf("Expected 'No call hierarchy' fallback, got: %s", result)
	}
}
