package ast_test

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/clangd/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestAST_Present verifies clangd's textDocument/ast extension for a range
// that covers the TestClass declaration in consumer.cpp.
func TestAST_Present(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
	defer cancel()

	// Open a file to ensure clangd loads compile commands and starts indexing.
	indexFile := filepath.Join(suite.WorkspaceDir, "src/main.cpp")
	if err := suite.Client.OpenFile(ctx, indexFile); err != nil {
		t.Logf("Note: failed to open %s for indexing: %v", indexFile, err)
	}

	// Give clangd some time to index the workspace.
	time.Sleep(5 * time.Second)

	// Query AST for the TestClass declaration range in consumer.cpp.
	targetFile := filepath.Join(suite.WorkspaceDir, "src/consumer.cpp")
	result, err := tools.GetAST(ctx, suite.Client, targetFile, 7, 1, 15, 2)
	if err != nil {
		t.Fatalf("GetAST failed: %v", err)
	}

	// Normalize unstable details such as heap-based pointer IDs in QualType dumps,
	// which change between runs but do not affect behavior.
	result = normalizeASTOutput(result)

	common.SnapshotTest(t, "clangd", "ast", "consumer-class", result)
}

// TestAST_Absent verifies behavior when querying an out-of-bounds range
// (no meaningful AST should be available).
func TestAST_Absent(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
	defer cancel()

	// Open a file to ensure clangd loads compile commands and starts indexing.
	indexFile := filepath.Join(suite.WorkspaceDir, "src/main.cpp")
	if err := suite.Client.OpenFile(ctx, indexFile); err != nil {
		t.Logf("Note: failed to open %s for indexing: %v", indexFile, err)
	}

	time.Sleep(5 * time.Second)

	targetFile := filepath.Join(suite.WorkspaceDir, "src/consumer.cpp")
	// Use an empty range at the very start of the file; depending on clangd,
	// this may yield no AST or a very small one, which we snapshot separately.
	result, err := tools.GetAST(ctx, suite.Client, targetFile, 1, 1, 1, 1)
	if err != nil {
		t.Fatalf("GetAST failed: %v", err)
	}

	result = normalizeASTOutput(result)

	common.SnapshotTest(t, "clangd", "ast", "consumer-class-out-of-range", result)
}

// normalizeASTOutput removes volatile pointer addresses (e.g. 0x7ffeabcd1234) from clangd AST dumps
// so that snapshots remain stable across runs and machines.
func normalizeASTOutput(input string) string {
	pointerRe := regexp.MustCompile(`0x[0-9a-fA-F]+`)
	return pointerRe.ReplaceAllString(input, "0xPOINTER")
}

