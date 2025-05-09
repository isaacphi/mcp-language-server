// Package internal contains shared helpers for Clangd tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for Clangd language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	// Configure Clangd LSP
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "clangd",
		Command:          "clangd",
		Args:             []string{"--compile-commands-dir=" + filepath.Join(repoRoot, "integrationtests/workspaces/clangd")},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/clangd"),
		InitializeTimeMs: 2000,
	}

	// Create a test suite
	suite := common.NewTestSuite(t, config)

	// Set up the suite
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		suite.Cleanup()
	})

	return suite
}
