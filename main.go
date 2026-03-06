package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
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
	workspaceDir string
	lspCommand   string
	lspArgs      []string
}

type mcpServer struct {
	config           config
	lspClient        *lsp.Client
	mcpServer        *server.MCPServer
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

func newServer(config *config) (*mcpServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &mcpServer{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

func (s *mcpServer) initializeLSP() error {
	// For clangd, prefer using the compile-commands-dir (if provided) as the
	// current working directory. This keeps relative paths consistent with the
	// compilation database and avoids cache conflicts with an editor instance.
	if isClangdExecutable(s.config.lspCommand) {
		baseDir := findCompileCommandsDir(s.config.lspArgs, s.config.workspaceDir)
		if err := os.Chdir(baseDir); err != nil {
			return fmt.Errorf("failed to change to clangd compile-commands-dir: %v", err)
		}

		// Ensure clangd uses a dedicated compile_commands.json symlink in
		// baseDir/llm_lsp so that its cache is separated from any editor
		// instance using the original compilation database.
		if err := s.prepareClangdCompileCommands(baseDir); err != nil {
			coreLogger.Warn("Failed to prepare clangd compile_commands.json: %v", err)
		}
	} else {
		if err := os.Chdir(s.config.workspaceDir); err != nil {
			return fmt.Errorf("failed to change to workspace directory: %v", err)
		}
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

// isClangdExecutable reports whether the given command path refers to clangd.
func isClangdExecutable(cmd string) bool {
	base := strings.ToLower(filepath.Base(cmd))
	return base == "clangd" || strings.HasPrefix(base, "clangd-")
}

// findCompileCommandsDir returns the directory specified by a
// --compile-commands-dir flag (if present), otherwise falls back to the
// workspace directory. Relative paths are resolved against the workspace.
func findCompileCommandsDir(args []string, workspaceDir string) string {
	var dir string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--compile-commands-dir=") {
			dir = strings.TrimPrefix(arg, "--compile-commands-dir=")
			break
		}
		if arg == "--compile-commands-dir" && i+1 < len(args) {
			dir = args[i+1]
			break
		}
	}

	if dir == "" {
		return workspaceDir
	}

	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspaceDir, dir)
	}

	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}

	return dir
}

// prepareClangdCompileCommands creates workspaceDir/llm_lsp/compile_commands.json
// as a symlink to the workspace's compile_commands.json (if present) and
// rewrites the LSP arguments so clangd uses that directory for its compilation
// database. This keeps the clangd cache used by the MCP server separate from
// any editor instance using the original compile_commands.json.
func (s *mcpServer) prepareClangdCompileCommands(baseDir string) error {
	// Locate compile_commands.json in the base directory (typically the
	// compile-commands-dir used by clangd).
	ccPath := filepath.Join(baseDir, "compile_commands.json")
	info, err := os.Stat(ccPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing to do if there is no compilation database.
			coreLogger.Debug("No compile_commands.json found in workspace, skipping clangd llm_lsp setup")
			return nil
		}
		return fmt.Errorf("failed to stat compile_commands.json: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("compile_commands.json exists but is a directory: %s", ccPath)
	}

	// Create the llm_lsp directory next to the original compile_commands.json.
	llmDir := filepath.Join(baseDir, "llm_lsp")
	if err := os.MkdirAll(llmDir, 0o755); err != nil {
		return fmt.Errorf("failed to create llm_lsp directory: %w", err)
	}

	// Create (or replace) a symlink to compile_commands.json inside llm_lsp.
	linkPath := filepath.Join(llmDir, "compile_commands.json")
	if _, err := os.Lstat(linkPath); err == nil || !os.IsNotExist(err) {
		// Remove any existing file or symlink at this path.
		if rmErr := os.Remove(linkPath); rmErr != nil {
			return fmt.Errorf("failed to remove existing llm_lsp compile_commands.json: %w", rmErr)
		}
	}
	if err := os.Symlink(ccPath, linkPath); err != nil {
		return fmt.Errorf("failed to create llm_lsp compile_commands.json symlink: %w", err)
	}

	// Ensure we use an absolute path for the compile-commands-dir argument.
	llmDirAbs, err := filepath.Abs(llmDir)
	if err != nil {
		return fmt.Errorf("failed to resolve llm_lsp directory: %w", err)
	}

	// Rewrite LSP args to force clangd to use llm_lsp as its compile-commands-dir.
	var newArgs []string
	skipNext := false
	for i := 0; i < len(s.config.lspArgs); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		arg := s.config.lspArgs[i]

		// Drop any existing --compile-commands-dir (with or without a value).
		if strings.HasPrefix(arg, "--compile-commands-dir=") {
			continue
		}
		if arg == "--compile-commands-dir" {
			// Skip this and its value (if present).
			skipNext = true
			continue
		}

		newArgs = append(newArgs, arg)
	}

	newArgs = append(newArgs, fmt.Sprintf("--compile-commands-dir=%s", llmDirAbs))
	s.config.lspArgs = newArgs

	coreLogger.Debug("Configured clangd to use compile_commands.json symlink at %s", linkPath)
	return nil
}

func (s *mcpServer) start() error {
	if err := s.initializeLSP(); err != nil {
		return err
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

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
}
