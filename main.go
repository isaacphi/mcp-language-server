package main

import (
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
	// Loaded LSP configuration
	lspConfig *Config

	// Mode flags
	isSessionMode   bool
	isSingleLSPMode bool
}

type mcpServer struct {
	config     config
	lspManager *LSPManager
	mcpServer  *server.MCPServer
}

// detectConfigFile looks for a config file in standard locations:
// 1. ~/.mcp-language-server.json
// 2. ~/.config/mcp-language-server.json
// Returns the path to the first existing file or an error if none found
func detectConfigFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	locations := []string{
		filepath.Join(homeDir, ".mcp-language-server.json"),
		filepath.Join(homeDir, ".config", "mcp-language-server.json"),
	}

	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			return location, nil
		}
	}

	return "", fmt.Errorf("no config file found in standard locations: %v", locations)
}

func parseConfig() (*config, error) {
	cfg := &config{}
	workspaceDir := ""
	lspCommand := ""
	var lspArgs []string
	var configFile string
	var sessionFile string

	flag.StringVar(&workspaceDir, "workspace", "", "Path to workspace directory (single-mcp mode)")
	flag.StringVar(&lspCommand, "lsp", "", "LSP command to run (single-mcp mode, args should be passed after --)")
	flag.StringVar(&configFile, "config", "", "Path to config file for unbounded or session mode")
	flag.StringVar(&sessionFile, "session", "", "Path to session file (disables runtime LSP add/remove)")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	lspArgs = flag.Args()

	// Determine mode based on provided flags
	hasWorkspace := workspaceDir != ""
	hasLSP := lspCommand != ""
	hasConfig := configFile != ""
	hasSession := sessionFile != ""

	// Auto-detect config file if no parameters are provided or if --session is used without --config
	if !hasConfig && ((!hasWorkspace && !hasLSP && !hasSession) || hasSession) {
		detectedConfig, err := detectConfigFile()
		if err != nil {
			if !hasWorkspace && !hasLSP && !hasSession {
				// No parameters provided at all - suggest auto-detection locations
				return nil, fmt.Errorf("no configuration provided. Please use:\n" +
					"  - --workspace and --lsp for single-mcp mode, or\n" +
					"  - --config for unbounded or session mode, or\n" +
					"  - place config file at ~/.mcp-language-server.json or ~/.config/mcp-language-server.json")
			}
			// --session without --config
			return nil, fmt.Errorf("--session requires --config flag or a config file at ~/.mcp-language-server.json or ~/.config/mcp-language-server.json: %v", err)
		}
		configFile = detectedConfig
		hasConfig = true
		coreLogger.Info("Auto-detected config file: %s", configFile)
	}

	// Validate mode combinations
	if hasConfig && (hasWorkspace || hasLSP) {
		return nil, fmt.Errorf("cannot use --config with --workspace or --lsp flags")
	}

	if hasSession && (hasWorkspace || hasLSP) {
		return nil, fmt.Errorf("cannot use --session with --workspace or --lsp flags")
	}

	if hasConfig {
		// Free mode (with optional session mode)
		lspConfig, err := LoadConfigFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %v", err)
		}
		cfg.lspConfig = lspConfig

		if hasSession {
			// Session mode - configure LSPs from session file during parameter parsing
			cfg.isSessionMode = true

			// Load the session file if it exists
			session, err := LoadSessionFile(sessionFile)
			if err != nil {
				if os.IsNotExist(err) {
					coreLogger.Warn("Session file does not exist yet: %s (will be created on save)", sessionFile)
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

					// Create nested map for language if it doesn't exist
					if cfg.lspConfig.LSPs[entry.Language] == nil {
						cfg.lspConfig.LSPs[entry.Language] = make(map[string]LSPConfig)
					}

					// Store LSPConfig keyed by language and workspace
					cfg.lspConfig.LSPs[entry.Language][entry.Workspace] = LSPConfig{
						LSPDefaultConfig: defaultLSP,
						Workspace:        entry.Workspace,
					}
				}

				coreLogger.Info("Loaded session from %s with %d LSPs", sessionFile, len(session.LSPs))
			}

			coreLogger.Info("Running in session mode (LSP add/remove disabled)")
		} else {
			coreLogger.Info("Running in unbounded mode with multi-LSP support")
		}

		return cfg, nil
	}

	// Single-MCP mode - require both workspace and lsp
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

	// Single-MCP mode: create a synthetic config with single LSP for manager
	cfg.isSingleLSPMode = true
	cfg.lspConfig = &Config{
		Defaults: make(map[string]LSPDefaultConfig), // Not used in single-mcp mode (tools not registered)
		LSPs: map[string]map[string]LSPConfig{
			"default": {
				absWorkspace: {
					LSPDefaultConfig: LSPDefaultConfig{
						Command: lspCommand,
						Args:    lspArgs,
						Env:     map[string]string{},
					},
					Workspace: absWorkspace,
				},
			},
		},
	}

	coreLogger.Info("Running in single-mcp mode with single LSP")
	return cfg, nil
}

func newServer(config *config) (*mcpServer, error) {
	s := &mcpServer{
		config:     *config,
		lspManager: NewLSPManager(config.lspConfig),
	}

	return s, nil
}

func (s *mcpServer) start() error {
	// Auto-start all configured LSPs that have a workspace
	// This applies to all modes: single-mcp, free, and session (session LSPs are pre-configured in parseConfig)
	// Count how many LSPs are configured to start
	configuredCount := 0
	for _, workspaceConfigs := range s.config.lspConfig.LSPs {
		for _, lspConfig := range workspaceConfigs {
			if lspConfig.Workspace != "" {
				configuredCount++
			}
		}
	}

	// Auto-start all configured LSPs during startup (duringStartup=true to avoid non-determinism)
	for language, workspaceConfigs := range s.config.lspConfig.LSPs {
		for workspace, lspConfig := range workspaceConfigs {
			if lspConfig.Workspace != "" {
				coreLogger.Debug("Auto-starting LSP for language: %s, workspace: %s", language, workspace)
				_, err := s.lspManager.StartLSP(workspace, language, true)
				if err != nil {
					coreLogger.Error("Failed to start LSP for language %s: %v", language, err)
					// Continue starting other LSPs even if one fails
				}
			}
		}
	}

	// After startup, if exactly one LSP is running (either single LSP configured, or others failed),
	// auto-select it for convenience
	instances := s.lspManager.ListLSPs()
	if len(instances) == 1 && configuredCount >= 1 {
		if err := s.lspManager.SelectLSP(instances[0]["id"]); err != nil {
			coreLogger.Debug("Failed to auto-select single running LSP: %v", err)
		}
	}

	s.mcpServer = server.NewMCPServer(
		"MCP Language Server",
		"v0.0.3",
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
