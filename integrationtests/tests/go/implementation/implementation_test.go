package implementation_test

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

// TestImplementation exercises the new implementation tool against
// gopls. Go's structural typing means there's no `implements` keyword
// for grep to match — this tool is the only AST-aware path to find
// satisfaction sets.
//
// The test workspace has `SharedInterface` (types.go:19) satisfied by
// `*SharedStruct` (types.go:6 via Process+GetName methods). Pointing
// the cursor at either anchor should resolve the other end.
func TestImplementation(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		line         int
		column       int
		expectedText string // substring expected in the result
		snapshotName string
	}{
		{
			// Cursor on `SharedInterface` (the interface name itself).
			// gopls returns the concrete types that satisfy it.
			name:         "InterfaceToImpl",
			file:         "types.go",
			line:         19,
			column:       6,
			expectedText: "types.go",
			snapshotName: "interface-to-impl",
		},
		{
			// Cursor on a method declared in the interface. gopls
			// returns the concrete methods that satisfy it.
			name:         "InterfaceMethodToImpl",
			file:         "types.go",
			line:         20,
			column:       2, // `Process` declaration inside SharedInterface
			expectedText: "types.go",
			snapshotName: "interface-method-to-impl",
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

			result, err := tools.FindImplementations(ctx, suite.Client, filePath, tt.line, tt.column)
			if err != nil {
				t.Fatalf("FindImplementations failed: %v", err)
			}

			if tt.expectedText != "" && !strings.Contains(result, tt.expectedText) {
				t.Errorf("Expected implementation result to contain %q but got:\n%s",
					tt.expectedText, result)
			}

			common.SnapshotTest(t, "go", "implementation", tt.snapshotName, result)
		})
	}
}

// TestImplementation_NoImpls covers the cursor-on-non-type path.
// gopls returns an LSP-level error for textDocument/implementation
// when the cursor is on a non-type symbol (`SharedConstant is a
// const, not a type`); other servers may return an empty list per
// the LSP spec. Either shape is acceptable — this test asserts the
// tool surfaces the reason clearly in EITHER case.
func TestImplementation_NoImpls(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "types.go")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open types.go: %v", err)
	}

	// Cursor on `SharedConstant` — a const, not an interface.
	result, err := tools.FindImplementations(ctx, suite.Client, filePath, 25, 7)
	if err != nil {
		// gopls path: surfaces "is a const, not a type" / "is not an
		// interface" style errors. Agent gets the reason via the
		// error message — acceptable behavior.
		if !strings.Contains(err.Error(), "not a type") &&
			!strings.Contains(err.Error(), "not an interface") {
			t.Fatalf("Expected an 'is a const, not a type' style error, got: %v", err)
		}
		return
	}
	// Spec-compliant path: empty list → handler surfaces the fallback.
	if !strings.Contains(result, "No implementations") {
		t.Errorf("Expected 'No implementations' marker, got: %s", result)
	}
}
