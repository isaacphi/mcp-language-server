// Package internal contains shared helpers for Go tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for Go language server tests (either starts gopls as subprocess, or connects to an LSP in headless mode)
func GetTestSuite(t *testing.T, headless bool) *common.TestSuite {
	// Configure Go LSP
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "go",
		Command:          "gopls",
		Args:             []string{},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/go"),
		InitializeTimeMs: 2000,
	}
	if headless {
		config.HeadlessListenArg = "-listen=127.0.0.1:%d" // Port will be decided at test run time
	}

	// Create a test suite
	suite := common.NewTestSuite(t, config)

	// Set up the suite
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	// Register cleanup
	t.Cleanup(func() { suite.Cleanup() })
	return suite
}
