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
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type config struct {
	workspaceDir string
	lspCommand   string
	lspArgs      []string
}

type server struct {
	config     config
	lspClient  *lsp.Client
	mcpServer  *mcp_golang.Server
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func parseConfig() (*config, error) {
	cfg := &config{}
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (args should be passed after --)")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	// Validate workspace directory
	if cfg.workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory is required")
	}

	workspaceDir, err := filepath.Abs(cfg.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	cfg.workspaceDir = workspaceDir

	if _, err := os.Stat(cfg.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", cfg.workspaceDir)
	}

	// Validate LSP command
	if cfg.lspCommand == "" {
		return nil, fmt.Errorf("LSP command is required")
	}

	if _, err := exec.LookPath(cfg.lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", cfg.lspCommand)
	}

	return cfg, nil
}

func newServer(config *config) (*server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &server{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

func (s *server) initializeLSP() error {
	if err := os.Chdir(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	client, err := lsp.NewClient(s.config.lspCommand, s.config.lspArgs...)
	if err != nil {
		return fmt.Errorf("failed to create LSP client: %v", err)
	}
	s.lspClient = client

	initResult, err := client.InitializeLSPClient(s.ctx, s.config.workspaceDir)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	log.Printf("Server capabilities: %+v\n\n", initResult.Capabilities)

	err = client.Initialized(s.ctx, protocol.InitializedParams{})
	if err != nil {
		return fmt.Errorf("initialized notification failed: %v", err)
	}

	return client.WaitForServerReady(s.ctx)
}

func (s *server) start() error {
	if err := s.initializeLSP(); err != nil {
		return err
	}

	s.mcpServer = mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := s.registerTools()
	if err != nil {
		return fmt.Errorf("tool registration failed: %v", err)
	}

	return s.mcpServer.Serve()
}

func (s *server) stop() {
	if s.lspClient != nil {
		err := s.lspClient.Shutdown(s.ctx)
		if err != nil {
			log.Printf("shutdown failed: %v", err)
		}

		err = s.lspClient.Exit(s.ctx)
		if err != nil {
			log.Printf("exit failed: %v", err)
		}

		s.lspClient.Close()
	}
	s.cancelFunc()
}

func main() {
	done := make(chan struct{})

	config, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}

	server, err := newServer(config)
	if err != nil {
		log.Fatal(err)
	}
	defer server.stop()

	log.Printf("Using workspace: %s\n", config.workspaceDir)
	log.Printf("Starting %s %v...\n", config.lspCommand, config.lspArgs)

	if err := server.start(); err != nil {
		log.Fatal(err)
	}

	// Wait forever
	<-done
}
