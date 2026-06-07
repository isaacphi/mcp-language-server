package rename_symbol_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/rust/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestRenameSymbolStaleBuffer reproduces silent file corruption: when a file is
// edited on disk by a non-LSP path after the server opened it, and a rename is
// issued before the server's buffer is re-synced, the server plans the
// WorkspaceEdit against its stale (pre-edit) buffer. Those ranges point at where
// references used to be; applied to the now-larger on-disk file they overwrite
// unrelated text and miss the real references, while the tool reports success.
//
// It runs the real configuration (watcher ON). rust-analyzer is warmed first so
// the rename completes in ~1ms, far inside the watcher's 300ms debounce, and wins
// the race exactly as in production. The ~200x margin keeps it deterministic: if
// the rename ever lost the race it fails loudly with ContentModified rather than
// flake. A correct bridge must sync open files to disk before renaming.
func TestRenameSymbolStaleBuffer(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	for _, f := range []string{
		"src/main.rs", "src/types.rs", "src/helper.rs",
		"src/consumer.rs", "src/another_consumer.rs", "src/clean.rs",
	} {
		if err := suite.Client.OpenFile(ctx, filepath.Join(suite.WorkspaceDir, f)); err != nil {
			t.Logf("open %s: %v", f, err)
		}
	}
	time.Sleep(3 * time.Second)

	// Force rust-analyzer's first (slow) analysis now so the timed rename is fast.
	for i := 0; i < 3; i++ {
		if _, err := tools.FindReferences(ctx, suite.Client, "SHARED_CONSTANT"); err != nil {
			t.Logf("warm-up references: %v", err)
		}
	}

	consumerPath := filepath.Join(suite.WorkspaceDir, "src/consumer.rs")
	original, err := os.ReadFile(consumerPath)
	if err != nil {
		t.Fatalf("read consumer.rs: %v", err)
	}

	// Edit consumer.rs on disk without notifying the server: prepend marker lines
	// so every SHARED_CONSTANT reference shifts down, off the server's stale view.
	const markerCount = 8
	var markers strings.Builder
	for i := 0; i < markerCount; i++ {
		fmt.Fprintf(&markers, "// REPRO MARKER %d - must be preserved verbatim\n", i)
	}
	if err := os.WriteFile(consumerPath, append([]byte(markers.String()), original...), 0644); err != nil {
		t.Fatalf("write consumer.rs: %v", err)
	}

	// Rename the constant at its definition in types.rs (line 78, col 11).
	typesPath := filepath.Join(suite.WorkspaceDir, "src/types.rs")
	t0 := time.Now()
	result, err := tools.RenameSymbol(ctx, suite.Client, typesPath, 78, 11, "RENAMED_CONSTANT")
	t.Logf("rename took %v (watcher debounce is 300ms)", time.Since(t0))
	if err != nil {
		t.Fatalf("RenameSymbol returned error (likely ContentModified, watcher beat the rename): %v", err)
	}
	t.Logf("rename result: %s", result)

	got, err := os.ReadFile(consumerPath)
	if err != nil {
		t.Fatalf("re-read consumer.rs: %v", err)
	}
	gotStr := string(got)

	// A correct rename replaces both SHARED_CONSTANT references and touches nothing
	// else; any other bytes are the stale-position corruption.
	want := markers.String() + strings.ReplaceAll(string(original), "SHARED_CONSTANT", "RENAMED_CONSTANT")
	if gotStr != want {
		t.Errorf("consumer.rs corrupted by a stale-position rename.\n--- got ---\n%s\n--- want ---\n%s", gotStr, want)
	}
}
