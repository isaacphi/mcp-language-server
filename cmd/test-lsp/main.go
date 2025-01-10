package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

type config struct {
	workspaceDir string
	lspCommand   string
	lspArgs      []string
	keyword      string
}

func parseConfig() (*config, error) {
	cfg := &config{}

	flag.StringVar(&cfg.keyword, "keyword", "main", "keyword to look up definition for")
	flag.StringVar(&cfg.workspaceDir, "workspace", ".", "Path to workspace directory (optional)")
	flag.StringVar(&cfg.lspCommand, "lsp", "gopls", "LSP command to run")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	// Validate and resolve workspace directory
	workspaceDir, err := filepath.Abs(cfg.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	cfg.workspaceDir = workspaceDir

	if _, err := os.Stat(cfg.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", cfg.workspaceDir)
	}

	// Validate LSP command
	if _, err := exec.LookPath(cfg.lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", cfg.lspCommand)
	}

	return cfg, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Change to the workspace directory
	if err := os.Chdir(cfg.workspaceDir); err != nil {
		log.Fatalf("Failed to change to workspace directory: %v", err)
	}

	fmt.Printf("Using workspace: %s\n", cfg.workspaceDir)
	fmt.Printf("Starting %s %v...\n", cfg.lspCommand, cfg.lspArgs)

	// Create a new LSP client
	client, err := lsp.NewClient(cfg.lspCommand, cfg.lspArgs...)
	if err != nil {
		log.Fatalf("Failed to create LSP client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	initResult, err := client.InitializeLSPClient(ctx, cfg.workspaceDir)
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	fmt.Printf("Server capabilities: %+v\n\n", initResult.Capabilities)

	err = client.Initialized(ctx, protocol.InitializedParams{})
	if err != nil {
		log.Fatalf("Initialized notification failed: %v", err)
	}

	if err := client.WaitForServerReady(ctx); err != nil {
		log.Fatalf("Server failed to become ready: %v", err)
	}

	// Test Tools
	text, err := tools.ReadDefinition(ctx, client, cfg.keyword, false)
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
