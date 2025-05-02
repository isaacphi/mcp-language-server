// Package internal contains shared helpers for Python tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for Python language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	// Configure Python LSP (pyright)
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "python",
		Command:          "pyright-langserver",
		Args:             []string{"--stdio"},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/python"),
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
