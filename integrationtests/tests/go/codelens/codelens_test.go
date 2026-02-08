package codelens_test

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestCodeLens tests the codelens functionality with the Go language server.
// Runs in both subprocess and headless (listen-mode) modes.
func TestCodeLens(t *testing.T) {
	t.Skip("Remove this line to run codelens tool tests")

	for _, mode := range []struct {
		name     string
		headless bool
	}{{"Subprocess", false}, {"Headless", true}} {
		t.Run(mode.name, func(t *testing.T) {
			// Test GetCodeLens with a file that should have codelenses
			t.Run("GetCodeLens", func(t *testing.T) {
				suite := internal.GetTestSuite(t, mode.headless)

				ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
				defer cancel()

				// The go.mod fixture already has an unused dependency

				// Wait for LSP to process the file
				time.Sleep(2 * time.Second)

				// Test GetCodeLens
				filePath := filepath.Join(suite.WorkspaceDir, "go.mod")
				result, err := tools.GetCodeLens(ctx, suite.Client, filePath)
				if err != nil {
					t.Fatalf("GetCodeLens failed: %v", err)
				}

				// Verify we have at least one code lens
				if !strings.Contains(result, "Code Lens results") {
					t.Errorf("Expected code lens results but got: %s", result)
				}

				// Verify we have a "go mod tidy" code lens
				if !strings.Contains(strings.ToLower(result), "tidy") {
					t.Errorf("Expected 'tidy' code lens but got: %s", result)
				}

				common.SnapshotTest(t, "go", "codelens", "get", result)
			})

			// Test ExecuteCodeLens by running the tidy codelens command
			t.Run("ExecuteCodeLens", func(t *testing.T) {
				suite := internal.GetTestSuite(t, mode.headless)

				ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
				defer cancel()

				// The go.mod fixture already has an unused dependency
				// Wait for LSP to process the file
				time.Sleep(2 * time.Second)

				// First get the code lenses to find the right index
				filePath := filepath.Join(suite.WorkspaceDir, "go.mod")
				result, err := tools.GetCodeLens(ctx, suite.Client, filePath)
				if err != nil {
					t.Fatalf("GetCodeLens failed: %v", err)
				}

				// Make sure we have a code lens with "tidy" in it
				if !strings.Contains(strings.ToLower(result), "tidy") {
					t.Fatalf("Expected 'tidy' code lens but none found: %s", result)
				}
				// Use regex to find a number in square brackets followed by 'tidy' with no square brackets in between,
				// this tells us which codelens index is the `go mod tidy`.
				re := regexp.MustCompile(`\[(\d+)\][^\[\]]*tidy`)
				match := re.FindStringSubmatch(result)
				if len(match) < 2 {
					t.Fatalf("Could not find code lens index for 'tidy': %s", result)
				}
				tidyIndex, err := strconv.Atoi(match[1])
				if err != nil {
					t.Fatalf("Failed to parse code lens index for 'tidy': %v", err)
				}

				t.Logf("Code lenses: %s", result)

				// Execute the code lens using the index we found for the tidy lens
				execResult, err := tools.ExecuteCodeLens(ctx, suite.Client, filePath, tidyIndex)
				if err != nil {
					t.Fatalf("ExecuteCodeLens failed: %v", err)
				}

				t.Logf("ExecuteCodeLens result: %s", execResult)

				// Wait for LSP to update the file
				time.Sleep(3 * time.Second)

				// Check if the file was updated (dependency should be removed)
				updatedContent, err := suite.ReadFile("go.mod")
				if err != nil {
					t.Fatalf("Failed to read updated go.mod: %v", err)
				}

				// Verify the dependency is gone
				if strings.Contains(updatedContent, "github.com/stretchr/testify") {
					t.Errorf("Expected dependency to be removed, but it's still there:\n%s", updatedContent)
				}

				common.SnapshotTest(t, "go", "codelens", "execute", execResult)
			})
		})
	}
}
