// Package internal contains shared helpers for TypeScript tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
)

// GetTestSuite returns a test suite for TypeScript language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	// Configure TypeScript LSP
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "typescript",
		Command:          "typescript-language-server",
		Args:             []string{"--stdio"},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/typescript"),
		InitializeTimeMs: 2000, // 2 seconds
	}

	// Create a test suite
	suite := common.NewTestSuite(t, config)

	// Set up the suite
	err = suite.Setup()
	if err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		suite.Cleanup()
	})

	return suite
}
