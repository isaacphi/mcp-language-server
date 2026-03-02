package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Create a logger for the core component
var coreLogger = logging.NewLogger(logging.Core)

type config struct {
	workspaceDir             string
	lspCommand               string
	lspArgs                  []string
	watcherPreopenOnRegister bool
	watcherPreopenMaxFiles   int
	idleTimeout              time.Duration
}

type mcpServer struct {
	config           config
	lspClient        *lsp.Client
	mcpServer        *server.MCPServer
	ctx              context.Context
	cancelFunc       context.CancelFunc
	workspaceWatcher *watcher.WorkspaceWatcher
	done             chan struct{}
	shutdownOnce     sync.Once
	lastActivityNs   atomic.Int64
}

func parseConfig() (*config, error) {
	preopenDefault := getEnvBool(true, "WATCHER_PREOPEN_ON_REGISTER", "MCP_WATCHER_PREOPEN_ON_REGISTER")
	preopenMaxDefault := getEnvInt(0, "WATCHER_PREOPEN_MAX_FILES", "MCP_WATCHER_PREOPEN_MAX_FILES")
	idleTimeoutDefault := getEnvDuration(0, "IDLE_TIMEOUT", "MCP_IDLE_TIMEOUT")

	cfg := &config{}
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (args should be passed after --)")
	flag.BoolVar(&cfg.watcherPreopenOnRegister, "watcher-preopen-on-register", preopenDefault, "Whether watcher registration should pre-open matching files")
	flag.IntVar(&cfg.watcherPreopenMaxFiles, "watcher-preopen-max-files", preopenMaxDefault, "Maximum files to pre-open on watcher registration (0 = unlimited)")
	flag.DurationVar(&cfg.idleTimeout, "idle-timeout", idleTimeoutDefault, "Idle timeout for automatic shutdown (0 = disabled)")
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

	if cfg.watcherPreopenMaxFiles < 0 {
		return nil, fmt.Errorf("watcher-preopen-max-files must be >= 0")
	}

	if cfg.idleTimeout < 0 {
		return nil, fmt.Errorf("idle-timeout must be >= 0")
	}

	return cfg, nil
}

func newServer(config *config) (*mcpServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &mcpServer{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
		done:       make(chan struct{}),
	}
	s.touchActivity()
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

	watcherConfig := watcher.DefaultWatcherConfig()
	watcherConfig.PreopenOnRegistration = s.config.watcherPreopenOnRegister
	watcherConfig.PreopenMaxFiles = s.config.watcherPreopenMaxFiles
	s.workspaceWatcher = watcher.NewWorkspaceWatcherWithConfig(client, watcherConfig)

	initResult, err := client.InitializeLSPClient(s.ctx, s.config.workspaceDir)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	coreLogger.Debug("Server capabilities: %+v", initResult.Capabilities)

	go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.workspaceDir)
	return client.WaitForServerReady(s.ctx)
}

func (s *mcpServer) start() error {
	if err := s.initializeLSP(); err != nil {
		return err
	}

	hooks := &server.Hooks{}
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		s.touchActivity()
	})

	s.mcpServer = server.NewMCPServer(
		"MCP Language Server",
		"v0.0.2",
		server.WithLogging(),
		server.WithRecovery(),
		server.WithHooks(hooks),
	)

	err := s.registerTools()
	if err != nil {
		return fmt.Errorf("tool registration failed: %v", err)
	}

	if s.config.idleTimeout > 0 {
		coreLogger.Info("Idle timeout enabled: %s", s.config.idleTimeout)
		go s.monitorIdleTimeout()
	}

	stdioServer := server.NewStdioServer(s.mcpServer)
	stdioServer.SetErrorLogger(log.New(os.Stderr, "", log.LstdFlags))
	return stdioServer.Listen(s.ctx, os.Stdin, os.Stdout)
}

func main() {
	coreLogger.Info("MCP Language Server starting")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cfg, err := parseConfig()
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	srv, err := newServer(cfg)
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
		if ppid == 1 {
			coreLogger.Warn("Server started with parent PID 1; parent-death monitoring may be ineffective")
		}

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
			case <-srv.done:
				return
			}
		}
	}()

	// Handle shutdown triggers
	go func() {
		select {
		case sig := <-sigChan:
			coreLogger.Info("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(srv)
		case <-parentDeath:
			coreLogger.Info("Parent death detected, initiating shutdown")
			cleanup(srv)
		case <-srv.done:
			return
		}
	}()

	exitCode := 0
	if err := srv.start(); err != nil {
		if isClosed(srv.done) || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || srv.ctx.Err() != nil {
			coreLogger.Info("Server exited during shutdown: %v", err)
		} else {
			coreLogger.Error("Server error: %v", err)
			exitCode = 1
		}
	}

	// Always run cleanup once the stdio server loop exits. This handles normal EOF
	// (client disconnected) and prevents lingering processes waiting on srv.done.
	cleanup(srv)

	<-srv.done
	coreLogger.Info("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(exitCode)
}

func (s *mcpServer) touchActivity() {
	s.lastActivityNs.Store(time.Now().UnixNano())
}

func (s *mcpServer) monitorIdleTimeout() {
	checkInterval := s.config.idleTimeout / 4
	if checkInterval < 5*time.Second {
		checkInterval = 5 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			last := time.Unix(0, s.lastActivityNs.Load())
			idleFor := time.Since(last)
			if idleFor >= s.config.idleTimeout {
				coreLogger.Warn("Idle timeout reached (%s >= %s), initiating shutdown", idleFor.Round(time.Second), s.config.idleTimeout)
				cleanup(s)
				return
			}
		case <-s.done:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func cleanup(s *mcpServer) {
	s.shutdownOnce.Do(func() {
		coreLogger.Info("Cleanup initiated for PID: %d", os.Getpid())

		// Stop background goroutines tied to server context.
		s.cancelFunc()

		// Close stdin so mcp-go's stdio loop unblocks and exits.
		if err := os.Stdin.Close(); err != nil {
			coreLogger.Debug("Failed to close stdin during cleanup: %v", err)
		}

		// Create a context with timeout for shutdown operations.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if s.lspClient != nil {
			coreLogger.Info("Closing open files")
			s.lspClient.CloseAllFiles(ctx)

			// Create a shorter timeout context for the shutdown request.
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer shutdownCancel()

			// Run shutdown in a goroutine with timeout to avoid blocking if LSP doesn't respond.
			shutdownDone := make(chan struct{})
			go func() {
				coreLogger.Info("Sending shutdown request")
				if err := s.lspClient.Shutdown(shutdownCtx); err != nil {
					coreLogger.Error("Shutdown request failed: %v", err)
				}
				close(shutdownDone)
			}()

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

		close(s.done)
		coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
	})
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func getEnvBool(defaultValue bool, keys ...string) bool {
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "t", "yes", "y", "on":
			return true
		case "0", "false", "f", "no", "n", "off":
			return false
		default:
			coreLogger.Warn("Invalid boolean value %q for %s, using default %v", value, key, defaultValue)
			return defaultValue
		}
	}

	return defaultValue
}

func getEnvInt(defaultValue int, keys ...string) int {
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			coreLogger.Warn("Invalid integer value %q for %s, using default %d", value, key, defaultValue)
			return defaultValue
		}
		return parsed
	}

	return defaultValue
}

func getEnvDuration(defaultValue time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}

		parsed, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			coreLogger.Warn("Invalid duration value %q for %s, using default %s", value, key, defaultValue)
			return defaultValue
		}
		return parsed
	}

	return defaultValue
}
