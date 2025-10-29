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
	"github.com/mark3labs/mcp-go/server"
)

// Create a logger for the core component
var coreLogger = logging.NewLogger(logging.Core)

type config struct {
	// Config mode fields
	configFile string
	lspConfig  *Config

	// Session mode fields
	sessionFile string

	// Mode flags
	isConfigMode  bool
	isSessionMode bool
	isLegacyMode  bool
}

type mcpServer struct {
	config      config
	lspManager  *LSPManager
	mcpServer   *server.MCPServer
	ctx         context.Context
	cancelFunc  context.CancelFunc
}

func parseConfig() (*config, error) {
	cfg := &config{}
	workspaceDir := ""
	lspCommand := ""
	var lspArgs []string

	flag.StringVar(&workspaceDir, "workspace", "", "Path to workspace directory (legacy mode)")
	flag.StringVar(&lspCommand, "lsp", "", "LSP command to run (legacy mode, args should be passed after --)")
	flag.StringVar(&cfg.configFile, "config", "", "Path to config file for multi-LSP mode")
	flag.StringVar(&cfg.sessionFile, "session", "", "Path to session file (disables runtime LSP add/remove)")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	lspArgs = flag.Args()

	// Determine mode based on provided flags
	hasWorkspace := workspaceDir != ""
	hasLSP := lspCommand != ""
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
			// Session mode - configure LSPs from session file during parameter parsing
			cfg.isSessionMode = true

			// Load the session file if it exists
			session, err := LoadSessionFile(cfg.sessionFile)
			if err != nil {
				if os.IsNotExist(err) {
					coreLogger.Warn("Session file does not exist yet: %s (will be created on save)", cfg.sessionFile)
					// No session to load, keep LSPs empty but in session mode
				} else {
					return nil, fmt.Errorf("failed to access session file: %v", err)
				}
			} else {
				// Build LSP configs from session file, combining workspace from session with defaults from config
				for _, entry := range session.LSPs {
					defaultLSP, ok := cfg.lspConfig.Defaults[entry.Language]
					if !ok {
						return nil, fmt.Errorf("language %s in session file not found in config", entry.Language)
					}

					// Create LSPConfig by copying from LSPDefaultConfig and setting workspace
					cfg.lspConfig.LSPs[entry.Language] = LSPConfig{
						LSPDefaultConfig: defaultLSP,
						Workspace:        entry.Workspace,
					}
				}

				coreLogger.Info("Loaded session from %s with %d LSPs", cfg.sessionFile, len(session.LSPs))
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
	absWorkspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}

	if _, err := os.Stat(absWorkspace); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", absWorkspace)
	}

	// Validate LSP command
	if _, err := exec.LookPath(lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", lspCommand)
	}

	// Legacy mode: create a synthetic config with single LSP for manager
	cfg.isConfigMode = true
	cfg.isLegacyMode = true

	defaultConfig := LSPDefaultConfig{
		Command: lspCommand,
		Args:    lspArgs,
		Env:     map[string]string{},
	}

	cfg.lspConfig = &Config{
		Defaults: map[string]LSPDefaultConfig{
			"default": defaultConfig,
		},
		LSPs: map[string]LSPConfig{
			"default": {
				LSPDefaultConfig: defaultConfig,
				Workspace:        absWorkspace,
			},
		},
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
		lspManager: NewLSPManager(config.lspConfig),
	}

	return s, nil
}


func (s *mcpServer) start() error {
	// Auto-start all configured LSPs that have a workspace
	// This applies to all modes: legacy, config, and session (session LSPs are pre-configured in parseConfig)
	for language, lspConfig := range s.config.lspConfig.LSPs {
		if lspConfig.Workspace != "" {
			coreLogger.Debug("Auto-starting LSP for language: %s", language)
			_, err := s.lspManager.StartLSP(s.ctx, lspConfig.Workspace, language)
			if err != nil {
				coreLogger.Error("Failed to start LSP for language %s: %v", language, err)
				// Continue starting other LSPs even if one fails
			}
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

	// Stop all LSP instances via manager
	if s.lspManager != nil {
		coreLogger.Info("Stopping all LSP instances")
		s.lspManager.StopAll()
	}

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
}
