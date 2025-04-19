// Package internal contains shared helpers for Rust tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/languages/common"
)

// GetTestSuite returns a test suite for Rust language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	// Configure Rust LSP (rust-analyzer)
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "rust",
		Command:          "rust-analyzer",
		Args:             []string{},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/rust"),
		InitializeTimeMs: 5000,
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
