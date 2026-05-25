package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Logger is an interface for logging in tests
type Logger interface {
	Printf(format string, v ...any)
}

// Helper to copy directories recursively
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper to copy a single file
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close source file: %v\n", err)
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close destination file: %v\n", err)
		}
	}()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// CleanupTestSuites is a helper to clean up all test suites in a test
func CleanupTestSuites(suites ...*TestSuite) {
	for _, suite := range suites {
		if suite != nil {
			suite.Cleanup()
		}
	}
}

// normalizePaths replaces absolute paths in the result with placeholder paths for consistent snapshots.
//
// Handles multi-occurrence lines correctly — call_hierarchy and
// some clangd outputs embed the same workspace path twice (once
// per LSP location field), and the previous Split-based approach
// silently dropped every occurrence after the first while leaving
// the runner's filesystem layout between them in plain text. This
// rewrites each "<prefix>/workspace/<rest>" occurrence to
// "/TEST_OUTPUT/workspace/<rest>" by scanning forward through the
// line, so a snapshot built on one developer's machine is byte-
// equal to one built on a CI runner regardless of where the test
// output directory lives on either filesystem.
func normalizePaths(_ *testing.T, input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = normalizeWorkspaceMarker(normalizeWorkspaceMarker(line, "/workspaces/"), "/workspace/")
	}
	return strings.Join(lines, "\n")
}

// normalizeWorkspaceMarker walks a single line and replaces every
// `<absolute path>/workspace/<rest-of-path>` segment with
// `/TEST_OUTPUT/workspace/<rest-of-path>`. The marker arg is
// either `/workspace/` or `/workspaces/` (clangd's convention).
//
// The replacement starts at each marker occurrence and walks
// backwards to the nearest path-segment boundary (a space, tab,
// open paren, comma, or start-of-line) to determine where the
// absolute-path prefix begins. That keeps the line's non-path
// content intact while stripping the per-machine prefix. We
// advance past the freshly-written placeholder before scanning
// for the next occurrence so an idempotent re-match (the
// placeholder ITSELF ends in `/workspace/`) doesn't infinite-loop.
func normalizeWorkspaceMarker(line, marker string) string {
	const placeholder = "/TEST_OUTPUT/workspace/"
	out := line
	cursor := 0 // resume scanning from here on each iteration
	for cursor < len(out) {
		rel := strings.Index(out[cursor:], marker)
		if rel < 0 {
			return out
		}
		idx := cursor + rel
		// Walk backwards from the marker until we hit a path-segment
		// boundary (or the start of the cursor — we don't rewind past
		// previously-normalized text).
		start := idx
		for start > cursor {
			c := out[start-1]
			if c == ' ' || c == '\t' || c == '(' || c == '[' || c == ',' {
				break
			}
			start--
		}
		out = out[:start] + placeholder + out[idx+len(marker):]
		// Resume past the freshly-written placeholder so the
		// trailing `/workspace/` doesn't re-match.
		cursor = start + len(placeholder)
	}
	return out
}

// FindRepoRoot locates the repository root by looking for specific indicators
// Exported so it can be used by other packages
func FindRepoRoot() (string, error) {
	// Start from the current directory and walk up until we find the main.go file
	// which is at the repository root
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if this is the repo root (has a go.mod file)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found the repo root
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the filesystem root without finding repo root
			return "", fmt.Errorf("repository root not found")
		}
		dir = parent
	}
}

// SnapshotTest compares the actual result against an expected result file
// If the file doesn't exist or UPDATE_SNAPSHOTS=true env var is set, it will update the snapshot
func SnapshotTest(t *testing.T, languageName, toolName, testName, actualResult string) {
	// Normalize paths in the result to avoid system-specific paths in snapshots
	actualResult = normalizePaths(t, actualResult)

	// Get the absolute path to the snapshots directory
	repoRoot, err := FindRepoRoot()
	if err != nil {
		t.Fatalf("Failed to find repo root: %v", err)
	}

	// Build path based on language/tool/testName hierarchy
	snapshotDir := filepath.Join(repoRoot, "integrationtests", "snapshots", languageName, toolName)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("Failed to create snapshots directory: %v", err)
	}

	snapshotFile := filepath.Join(snapshotDir, testName+".snap")

	// Use a package-level flag to control snapshot updates
	updateFlag := os.Getenv("UPDATE_SNAPSHOTS") == "true"

	// If snapshot doesn't exist or update flag is set, write the snapshot
	_, err = os.Stat(snapshotFile)
	if os.IsNotExist(err) || updateFlag {
		if err := os.WriteFile(snapshotFile, []byte(actualResult), 0644); err != nil {
			t.Fatalf("Failed to write snapshot: %v", err)
		}
		if os.IsNotExist(err) {
			t.Logf("Created new snapshot: %s", snapshotFile)
		} else {
			t.Logf("Updated snapshot: %s", snapshotFile)
		}
		return
	}

	// Read the expected result
	expectedBytes, err := os.ReadFile(snapshotFile)
	if err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}
	expected := string(expectedBytes)

	// Compare the results
	if expected != actualResult {
		t.Errorf("Result doesn't match snapshot.\nExpected:\n%s\n\nActual:\n%s", expected, actualResult)

		// Create a diff file for debugging
		diffFile := snapshotFile + ".diff"
		diffContent := fmt.Sprintf("=== Expected ===\n%s\n\n=== Actual ===\n%s", expected, actualResult)
		if err := os.WriteFile(diffFile, []byte(diffContent), 0644); err != nil {
			t.Logf("Failed to write diff file: %v", err)
		} else {
			t.Logf("Wrote diff to: %s", diffFile)
		}
	}
}
