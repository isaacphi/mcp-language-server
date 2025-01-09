package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

type LSPCommand struct {
	WorkspaceDir string
	Command      string
	Args         []string
}

// Parse a command string into command and arguments
func parseLSPCommand(cmdStr string) LSPCommand {
	parts := strings.Fields(cmdStr)
	return LSPCommand{
		Command: parts[0],
		Args:    parts[1:],
	}
}

func main() {
	// Define command line flags
	var (
		workspaceDir  string
		lspCommandStr string
		keyword       string
	)

	flag.StringVar(&keyword, "keyword", "main", "keyword")
	flag.StringVar(&workspaceDir, "workspace", ".", "Path to workspace directory (optional)")
	flag.StringVar(&lspCommandStr, "lsp", "gopls", "LSP command to run (e.g., 'gopls -remote=auto')")
	flag.Parse()

	// Handle workspace directory
	workspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for workspace: %v", err)
	}

	// Validate workspace directory
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		log.Fatalf("Workspace directory does not exist: %s", workspaceDir)
	}

	// Parse LSP command
	lspCmd := parseLSPCommand(lspCommandStr)

	// Verify the LSP command exists
	if _, err := exec.LookPath(lspCmd.Command); err != nil {
		log.Fatalf("LSP command not found: %s", lspCmd.Command)
	}

	// Change to the workspace directory
	if err := os.Chdir(workspaceDir); err != nil {
		log.Fatalf("Failed to change to workspace directory: %v", err)
	}

	fmt.Printf("Using workspace: %s\n", workspaceDir)

	// Create a new LSP client
	fmt.Printf("Starting %s...\n", lspCmd.Command)
	client, err := lsp.NewClient(lspCmd.Command, lspCmd.Args...)
	if err != nil {
		log.Fatalf("Failed to create LSP client: %v", err)
	}
	defer client.Close()
	// Initialize
	ctx := context.Background()
	initResult, err := client.InitializeLSPClient(ctx, workspaceDir)
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	fmt.Printf("Server capabilities: %+v\n\n", initResult.Capabilities)

	// Send initialized notification
	err = client.Initialized(ctx, protocol.InitializedParams{})
	if err != nil {
		log.Fatalf("Initialized notification failed: %v", err)
	}

	if err := client.WaitForServerReady(ctx); err != nil {
		log.Fatalf("Server failed to become ready: %v", err)
	}

	// Test Tools
	text, err := tools.ReadDefinition(ctx, client, keyword)
	if err != nil {
		log.Fatalf("GetDefinition failed: %v", err)
	}

	fmt.Println(text)

	// Cleanup
	fmt.Println("\nShutting down...")
	err = client.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Shutdown failed: %v", err)
	}

	err = client.Exit(ctx)
	if err != nil {
		log.Fatalf("Exit failed: %v", err)
	}
	fmt.Println("Server shut down successfully")
}
