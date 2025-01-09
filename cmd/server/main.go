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
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type config struct {
	workspaceDir  string
	lspCommandStr string
}

type server struct {
	config     config
	lspClient  *lsp.Client
	mcpServer  *mcp_golang.Server
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type lspCommand struct {
	command string
	args    []string
}

func parseLSPCommand(cmdStr string) lspCommand {
	parts := strings.Fields(cmdStr)
	return lspCommand{
		command: parts[0],
		args:    parts[1:],
	}
}

func parseConfig() (*config, error) {
	config := &config{}

	flag.StringVar(&config.workspaceDir, "workspace", "", "Path to workspace directory (optional)")
	flag.StringVar(&config.lspCommandStr, "lsp", "", "LSP command to run (e.g., 'gopls -remote=auto')")
	flag.Parse()

	workspaceDir, err := filepath.Abs(config.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	config.workspaceDir = workspaceDir

	if _, err := os.Stat(config.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", config.workspaceDir)
	}

	return config, nil
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
	lspCmd := parseLSPCommand(s.config.lspCommandStr)

	if _, err := exec.LookPath(lspCmd.command); err != nil {
		return fmt.Errorf("LSP command not found: %s", lspCmd.command)
	}

	if err := os.Chdir(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	client, err := lsp.NewClient(lspCmd.command, lspCmd.args...)
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
	log.Printf("Starting %s...\n", config.lspCommandStr)

	if err := server.start(); err != nil {
		log.Fatal(err)
	}

	// Wait forever
	<-done
}
