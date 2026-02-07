// Package internal contains shared helpers for Go tests
package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for Go language server tests (starts gopls as subprocess)
func GetTestSuite(t *testing.T) *common.TestSuite {
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

	suite := common.NewTestSuite(t, config)
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	t.Cleanup(func() { suite.Cleanup() })
	return suite
}

// GetHeadlessTestSuite returns a test suite that connects to an existing gopls at GOPLS_HEADLESS_ADDR.
// Skips the test if GOPLS_HEADLESS_ADDR is not set. The server must be started separately (e.g. gopls -listen=:6060).
func GetHeadlessTestSuite(t *testing.T) *common.TestSuite {
	addr := os.Getenv("GOPLS_HEADLESS_ADDR")
	if addr == "" {
		t.Skip("GOPLS_HEADLESS_ADDR not set; set to e.g. localhost:6060 to run headless tests (gopls must be running with -listen=:6060)")
	}

	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "go",
		ConnectAddr:      addr,
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/go"),
		InitializeTimeMs: 2000,
	}

	suite := common.NewTestSuite(t, config)
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up headless test suite: %v", err)
	}
	t.Cleanup(func() { suite.Cleanup() })
	return suite
}
