package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
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
	config config
	// lspClientMu guards lspClient and shuttingDown: start() assigns lspClient
	// from the server goroutine while a shutdown trigger may have cleanup() read
	// it from another goroutine, so the two must not race on the bare pointer.
	lspClientMu sync.RWMutex
	lspClient   *lsp.Client
	// shuttingDown is set by cleanup() under lspClientMu so that initializeLSP,
	// if it is mid-flight, can see the shutdown and tear down the LSP child it
	// just spawned instead of leaking it as an orphan.
	shuttingDown     bool
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
	if err := os.Chdir(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	client, err := lsp.NewClient(s.config.lspCommand, s.config.lspArgs...)
	if err != nil {
		return fmt.Errorf("failed to create LSP client: %v", err)
	}
	// NewClient has already spawned the LSP child process. If a shutdown trigger
	// fired during that spawn, cleanup() ran before s.lspClient was published and
	// could not tear the child down — so close it here to avoid leaking an
	// orphan. The lock makes this race-free: cleanup() either reads the assigned
	// client (and closes it) or sets shuttingDown before we publish (and we close
	// it). Exactly one path owns the teardown.
	s.lspClientMu.Lock()
	if s.shuttingDown {
		s.lspClientMu.Unlock()
		if closeErr := client.Close(); closeErr != nil {
			coreLogger.Error("Failed to close LSP client spawned during shutdown: %v", closeErr)
		}
		return fmt.Errorf("shutdown initiated during LSP initialization")
	}
	s.lspClient = client
	s.lspClientMu.Unlock()
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

	// Ensure teardown runs at most once, regardless of which trigger fires
	// first (OS signal, parent death, or stdin EOF). Without this the racing
	// triggers could run cleanup() — and thus the LSP shutdown/kill — twice.
	var cleanupOnce sync.Once
	shutdown := func() {
		cleanupOnce.Do(func() {
			cleanup(server, done)
		})
	}

	// drainServerErr does a non-blocking read of any result start() has already
	// produced, used on the signal/parent-death paths to preserve a genuine
	// independent start() failure in the exit code. It normalizes context.Canceled
	// to nil: mcp-go's ServeStdio installs its own SIGTERM/SIGINT handler that
	// cancels its context, so on a signal start() returns context.Canceled — that
	// is our own intentional shutdown, not a failure, and must not become exit 1.
	drainServerErr := func(ch <-chan error) error {
		select {
		case e := <-ch:
			if errors.Is(e, context.Canceled) {
				return nil
			}
			return e
		default:
			return nil
		}
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination.
	// The reliable death signal is the stdio EOF handled below: when the host
	// exits, the kernel closes its end of our stdio pipe and server.start()
	// returns. That is the primary trigger and works on every platform. This
	// getppid() poll is only a best-effort backstop for the rare case where stdio
	// stays open (e.g. redirected to a pty or a pipe another process holds) after
	// the host is gone — macOS has no PR_SET_PDEATHSIG equivalent, so we cannot
	// get a kernel-delivered death signal there.
	//
	// We only act when the reparent involves PID 1 (reparented to, or originally
	// owned by, init): a bare "ppid changed" check would also fire when an
	// intermediate process in a wrapper/subreaper spawn chain exits while the real
	// host is still alive, killing the LSP child mid-session. The cost of that
	// conservatism is a known gap: under a PR_SET_CHILD_SUBREAPER ancestor on
	// Linux (systemd user scope, some multiplexers/containers) an orphaned child
	// reparents to the subreaper's PID, not 1, so this poll never fires there and
	// we fall back entirely on the stdio-EOF path. This poll is a backstop, not a
	// guarantee.
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

	// Run the blocking stdio server in the background so teardown can be driven
	// from a single place, no matter which trigger fires first. ServeStdio only
	// returns on a clean stdio EOF (host closed the pipe) or an error; it does
	// NOT return on parent death while stdio stays open, so the main goroutine
	// must not park on it — otherwise an orphaned process would never exit.
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.start()
	}()

	// Wait for the first teardown trigger: OS signal, parent death, or the
	// stdio server returning.
	var serverErr error
	select {
	case sig := <-sigChan:
		coreLogger.Info("Received signal %v in PID: %d", sig, os.Getpid())
		// If start() had already failed on its own before the signal arrived,
		// preserve that error for the exit code. We have not begun teardown yet,
		// so anything already queued here is a genuine independent failure, not a
		// side effect of us closing stdin during cleanup.
		serverErr = drainServerErr(serverErrCh)
	case <-parentDeath:
		coreLogger.Info("Parent death detected, initiating shutdown")
		serverErr = drainServerErr(serverErrCh)
	case serverErr = <-serverErrCh:
		if serverErr != nil {
			coreLogger.Error("Server error: %v", serverErr)
		} else {
			coreLogger.Info("MCP stdio closed (EOF), initiating shutdown for PID: %d", os.Getpid())
		}
	}

	// Run cleanup concurrently and bound how long we wait for it. Running it in
	// a goroutine is what lets the timeout below actually fire: cleanup()'s
	// s.lspClient.Close() is unbounded, so a wedged LSP must not be able to
	// block us inside cleanup(). It also lets us exit even while server.start()
	// is still parked in ServeStdio (the orphan/pty case where stdin never EOFs).
	go shutdown()

	select {
	case <-done:
		coreLogger.Info("Server shutdown complete for PID: %d", os.Getpid())
	case <-time.After(5 * time.Second):
		coreLogger.Warn("Shutdown wait timed out, forcing exit for PID: %d", os.Getpid())
	}
	if serverErr != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func cleanup(s *mcpServer, done chan struct{}) {
	coreLogger.Info("Cleanup initiated for PID: %d", os.Getpid())

	// Stop the workspace watcher first. It runs on s.ctx and otherwise keeps
	// issuing notifications (writes to the LSP stdin) throughout teardown, racing
	// the shutdown/exit/close below. Cancelling s.ctx makes its loop exit.
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Mark shutdown and snapshot the client under the lock. If start() is still
	// mid-init on another goroutine, either we read the client it already
	// published (and close it here) or it sees shuttingDown after we set it and
	// closes the freshly spawned child itself — so the child is never leaked.
	s.lspClientMu.Lock()
	s.shuttingDown = true
	lspClient := s.lspClient
	s.lspClientMu.Unlock()
	if lspClient != nil {
		coreLogger.Info("Closing open files")
		lspClient.CloseAllFiles(ctx)

		// Create a shorter timeout context for the shutdown request
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer shutdownCancel()

		// Run shutdown in a goroutine with timeout to avoid blocking if LSP doesn't respond
		shutdownDone := make(chan struct{})
		go func() {
			coreLogger.Info("Sending shutdown request")
			if err := lspClient.Shutdown(shutdownCtx); err != nil {
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
		if err := lspClient.Exit(ctx); err != nil {
			coreLogger.Error("Exit notification failed: %v", err)
		}

		coreLogger.Info("Closing LSP client")
		if err := lspClient.Close(); err != nil {
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
