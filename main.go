package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
	"github.com/mark3labs/mcp-go/server"
)

// Create a logger for the core component
var coreLogger = logging.NewLogger(logging.Core)

type config struct {
	// Legacy mode fields
	workspaceDir string
	lspCommand   string
	lspArgs      []string

	// Config mode fields
	configFile string
	lspConfig  *Config

	// Session mode fields
	sessionFile string

	// Mode flags
	isConfigMode  bool
	isSessionMode bool
}

type mcpServer struct {
	config config

	// Legacy mode: single LSP client
	lspClient        *lsp.Client
	workspaceWatcher *watcher.WorkspaceWatcher

	// Config mode: LSP manager
	lspManager *LSPManager

	mcpServer  *server.MCPServer
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func parseConfig() (*config, error) {
	cfg := &config{}
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory (legacy mode)")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (legacy mode, args should be passed after --)")
	flag.StringVar(&cfg.configFile, "config", "", "Path to config file for multi-LSP mode")
	flag.StringVar(&cfg.sessionFile, "session", "", "Path to session file (disables runtime LSP add/remove)")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	// Determine mode based on provided flags
	hasWorkspace := cfg.workspaceDir != ""
	hasLSP := cfg.lspCommand != ""
	hasConfig := cfg.configFile != ""
	hasSession := cfg.sessionFile != ""

	// Validate mode combinations
	if hasConfig && (hasWorkspace || hasLSP) {
		return nil, fmt.Errorf("cannot use --config with --workspace or --lsp flags")
	}

	if hasSession && (hasWorkspace || hasLSP) {
		return nil, fmt.Errorf("cannot use --session with --workspace or --lsp flags")
	}

	if hasSession && !hasConfig {
		return nil, fmt.Errorf("--session requires --config flag")
	}

	if hasConfig {
		// Config mode (with optional session mode)
		cfg.isConfigMode = true

		lspConfig, err := LoadConfigFile(cfg.configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %v", err)
		}
		cfg.lspConfig = lspConfig

		if hasSession {
			// Session mode
			cfg.isSessionMode = true

			// Validate session file exists
			if _, err := os.Stat(cfg.sessionFile); err != nil {
				if os.IsNotExist(err) {
					coreLogger.Warn("Session file does not exist yet: %s (will be created on save)", cfg.sessionFile)
				} else {
					return nil, fmt.Errorf("failed to access session file: %v", err)
				}
			}

			coreLogger.Info("Running in session mode (LSP add/remove disabled)")
		} else {
			coreLogger.Info("Running in config mode with multi-LSP support")
		}

		return cfg, nil
	}

	// Legacy mode - require both workspace and lsp
	if !hasWorkspace {
		return nil, fmt.Errorf("workspace directory is required (use --workspace, --config, or --config with --session)")
	}

	if !hasLSP {
		return nil, fmt.Errorf("LSP command is required (use --lsp, --config, or --config with --session)")
	}

	// Validate workspace directory
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

	coreLogger.Info("Running in legacy mode with single LSP")
	return cfg, nil
}

func newServer(config *config) (*mcpServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &mcpServer{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// Initialize LSP manager in config mode
	if config.isConfigMode {
		s.lspManager = NewLSPManager(config.lspConfig)
	}

	return s, nil
}

func (s *mcpServer) initializeLSP() error {
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

	coreLogger.Debug("Server capabilities: %+v", initResult.Capabilities)

	go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.workspaceDir)
	return client.WaitForServerReady(s.ctx)
}

func (s *mcpServer) start() error {
	// Only initialize LSP in legacy mode
	// In config mode, LSPs are started dynamically via lsp_start tool or session file
	if !s.config.isConfigMode {
		if err := s.initializeLSP(); err != nil {
			return err
		}
	} else if s.config.isSessionMode {
		// Auto-load session in session mode
		if _, err := os.Stat(s.config.sessionFile); err == nil {
			coreLogger.Info("Auto-loading session from %s", s.config.sessionFile)
			if err := s.lspManager.LoadSession(s.ctx, s.config.sessionFile); err != nil {
				coreLogger.Error("Failed to load session: %v", err)
				// Don't fail startup if session load fails
			}
		} else {
			coreLogger.Info("No existing session file to load")
		}
	}

	s.mcpServer = server.NewMCPServer(
		"MCP Language Server",
		"v0.0.2",
		server.WithLogging(),
		server.WithRecovery(),
	)

	err := s.registerTools()
	if err != nil {
		return fmt.Errorf("tool registration failed: %v", err)
	}

	return server.ServeStdio(s.mcpServer)
}

func main() {
	coreLogger.Info("MCP Language Server starting")

	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config, err := parseConfig()
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	server, err := newServer(config)
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination
	// Claude desktop does not properly kill child processes for MCP servers
	go func() {
		ppid := os.Getppid()
		coreLogger.Debug("Monitoring parent process: %d", ppid)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentPpid := os.Getppid()
				if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
					coreLogger.Info("Parent process %d terminated (current ppid: %d), initiating shutdown", ppid, currentPpid)
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
			coreLogger.Info("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(server, done)
		case <-parentDeath:
			coreLogger.Info("Parent death detected, initiating shutdown")
			cleanup(server, done)
		}
	}()

	if err := server.start(); err != nil {
		coreLogger.Error("Server error: %v", err)
		cleanup(server, done)
		os.Exit(1)
	}

	<-done
	coreLogger.Info("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(0)
}

func cleanup(s *mcpServer, done chan struct{}) {
	coreLogger.Info("Cleanup initiated for PID: %d", os.Getpid())

	if s.config.isConfigMode {
		// Config mode: stop all LSP instances via manager
		if s.lspManager != nil {
			coreLogger.Info("Stopping all LSP instances")
			s.lspManager.StopAll()
		}
	} else {
		// Legacy mode: stop single LSP client
		// Create a context with timeout for shutdown operations
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if s.lspClient != nil {
			coreLogger.Info("Closing open files")
			s.lspClient.CloseAllFiles(ctx)

			// Create a shorter timeout context for the shutdown request
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer shutdownCancel()

			// Run shutdown in a goroutine with timeout to avoid blocking if LSP doesn't respond
			shutdownDone := make(chan struct{})
			go func() {
				coreLogger.Info("Sending shutdown request")
				if err := s.lspClient.Shutdown(shutdownCtx); err != nil {
					coreLogger.Error("Shutdown request failed: %v", err)
				}
				close(shutdownDone)
			}()

			// Wait for shutdown with timeout
			select {
			case <-shutdownDone:
				coreLogger.Info("Shutdown request completed")
			case <-time.After(1 * time.Second):
				coreLogger.Warn("Shutdown request timed out, proceeding with exit")
			}

			coreLogger.Info("Sending exit notification")
			if err := s.lspClient.Exit(ctx); err != nil {
				coreLogger.Error("Exit notification failed: %v", err)
			}

			coreLogger.Info("Closing LSP client")
			if err := s.lspClient.Close(); err != nil {
				coreLogger.Error("Failed to close LSP client: %v", err)
			}
		}
	}

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
}
