package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
	"github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

var debug = os.Getenv("DEBUG") != ""

type config struct {
	workspaceDir string
	lspCommand   string
	lspArgs      []string
}

type server struct {
	config           config
	lspClient        *lsp.Client
	mcpServer        *mcp_golang.Server
	ctx              context.Context
	cancelFunc       context.CancelFunc
	workspaceWatcher *watcher.WorkspaceWatcher
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
	s.workspaceWatcher = watcher.NewWorkspaceWatcher(client)

	initResult, err := client.InitializeLSPClient(s.ctx, s.config.workspaceDir)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	if debug {
		log.Printf("Server capabilities: %+v\n\n", initResult.Capabilities)
	}

	err = client.Initialized(s.ctx, protocol.InitializedParams{})
	if err != nil {
		return fmt.Errorf("initialized notification failed: %v", err)
	}

	go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.workspaceDir)
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

func main() {
	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}

	server, err := newServer(config)
	if err != nil {
		log.Fatal(err)
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination
	go func() {
		ppid := os.Getppid()
		if debug {
			log.Printf("Monitoring parent process: %d", ppid)
		}

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentPpid := os.Getppid()
				if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
					log.Printf("Parent process %d terminated (current ppid: %d), initiating shutdown", ppid, currentPpid)
					close(parentDeath)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Handle shutdown triggers
	go func() {
		select {
		case sig := <-sigChan:
			log.Printf("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(server, done)
		case <-parentDeath:
			log.Printf("Parent death detected, initiating shutdown")
			cleanup(server, done)
		}
	}()

	if err := server.start(); err != nil {
		log.Printf("Server error: %v", err)
		cleanup(server, done)
		os.Exit(1)
	}

	<-done
	log.Printf("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(0)
}

func cleanup(s *server, done chan struct{}) {
	log.Printf("Cleanup initiated for PID: %d", os.Getpid())

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.lspClient != nil {
		log.Printf("Sending shutdown request")
		if err := s.lspClient.Shutdown(ctx); err != nil {
			log.Printf("Shutdown request failed: %v", err)
		}

		log.Printf("Sending exit notification")
		if err := s.lspClient.Exit(ctx); err != nil {
			log.Printf("Exit notification failed: %v", err)
		}

		log.Printf("Closing LSP client")
		if err := s.lspClient.Close(); err != nil {
			log.Printf("Failed to close LSP client: %v", err)
		}
	}

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	log.Printf("Cleanup completed for PID: %d", os.Getpid())
}
