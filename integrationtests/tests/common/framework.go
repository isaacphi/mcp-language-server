package common

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
)

// LSPTestConfig defines configuration for a language server test
type LSPTestConfig struct {
	Name              string   // Name of the language server
	Command           string   // Command to run (ignored if ConnectAddr is set)
	Args              []string // Arguments (ignored if ConnectAddr is set)
	ConnectAddr       string   // If set, connect to existing LSP at this address (headless) instead of starting Command
	HeadlessListenArg string   // If set, start LSP with this listen arg (e.g. "-listen=127.0.0.1:6061") and connect via NewClientHeadless
	WorkspaceDir      string   // Template workspace directory
	InitializeTimeMs  int      // Time to wait after initialization in ms
}

// TestSuite contains everything needed for running integration tests
type TestSuite struct {
	Config       LSPTestConfig
	Client       *lsp.Client
	WorkspaceDir string
	TempDir      string
	Context      context.Context
	Cancel       context.CancelFunc
	Watcher      *watcher.WorkspaceWatcher
	initialized  bool
	cleanupOnce  sync.Once
	logFile      string
	t            *testing.T
	LanguageName string
	headless     bool      // true when using ConnectAddr or HeadlessListenArg (affects cleanup)
	headlessCmd  *exec.Cmd // when we start LSP in listen mode, the process we started (for cleanup)
}

// NewTestSuite creates a new test suite for the given language server
func NewTestSuite(t *testing.T, config LSPTestConfig) *TestSuite {
	ctx, cancel := context.WithCancel(context.Background())
	return &TestSuite{
		Config:       config,
		Context:      ctx,
		Cancel:       cancel,
		initialized:  false,
		t:            t,
		LanguageName: config.Name,
	}
}

// startLSPInListenMode reserves a port, starts the LSP with the same Command/Args plus
// HeadlessListenArg (with %d replaced by the port), and waits until the server accepts connections.
// Caller must connect with NewClientHeadless(addr) and is responsible for killing the process on cleanup.
func (ts *TestSuite) startLSPInListenMode() (addr string, cmd *exec.Cmd, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("failed to reserve port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return "", nil, fmt.Errorf("failed to close listener: %w", err)
	}
	addr = "127.0.0.1:" + strconv.Itoa(port)
	listenArg := fmt.Sprintf(ts.Config.HeadlessListenArg, port)
	fullArgs := append(append([]string{}, ts.Config.Args...), listenArg)
	cmd = exec.Command(ts.Config.Command, fullArgs...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start LSP: %w", err)
	}
	// Wait for server to accept connections (retry with backoff)
	const maxWait = 15 * time.Second
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return addr, cmd, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return "", nil, fmt.Errorf("LSP at %s did not accept connections within %v", addr, maxWait)
}

// Setup initializes the test suite, copies the workspace, and starts the LSP
func (ts *TestSuite) Setup() error {
	if ts.initialized {
		return fmt.Errorf("test suite already initialized")
	}

	// Create test output directory in the repo
	// Create a log file named after the test
	testName := ts.t.Name()
	// Clean the test name for use in a filename
	testName = strings.ReplaceAll(testName, "/", "_")
	testName = strings.ReplaceAll(testName, " ", "_")

	// Navigate to the repo root (assuming tests run from within the repo)
	// The executable is in a temporary directory, so find the repo root based on the package path
	pkgDir, err := filepath.Abs("../../../")
	if err != nil {
		return fmt.Errorf("failed to get absolute path to repo root: %w", err)
	}

	testOutputDir := filepath.Join(pkgDir, "test-output")
	if err := os.MkdirAll(testOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create test-output directory: %w", err)
	}

	// Create a consistent directory for this language server
	// Extract the language name from the config
	langName := ts.Config.Name
	if langName == "" {
		langName = "unknown"
	}

	// Use a consistent directory name based on the language
	tempDir := filepath.Join(testOutputDir, langName, testName)
	logsDir := filepath.Join(tempDir, "logs")
	workspaceDir := filepath.Join(tempDir, "workspace")

	// Clean up previous test output
	if _, err := os.Stat(tempDir); err == nil {
		ts.t.Logf("Cleaning up previous test directory: %s", tempDir)
		if err := os.RemoveAll(workspaceDir); err != nil {
			ts.t.Logf("Warning: Failed to clean up previous test directory: %v", err)
		}
	}

	// Create a fresh directory
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create test directory: %w", err)
	}
	ts.TempDir = tempDir
	ts.t.Logf("Created test directory: %s", tempDir)

	// Set up logging
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFileName := fmt.Sprintf("%s.log", testName)
	ts.logFile = filepath.Join(logsDir, logFileName)

	// Clear file if it already existed
	if err := os.Remove(ts.logFile); err != nil {
		log.Printf("failed to remove old log file: %s", ts.logFile)
	}

	// Configure logging to write to the file
	if err := logging.SetupFileLogging(ts.logFile); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}

	// Set log level based on environment variable or default to Info
	logLevel := logging.LevelInfo
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		switch strings.ToUpper(envLevel) {
		case "DEBUG":
			logLevel = logging.LevelDebug
		case "INFO":
			logLevel = logging.LevelInfo
		case "WARN":
			logLevel = logging.LevelWarn
		case "ERROR":
			logLevel = logging.LevelError
		case "FATAL":
			logLevel = logging.LevelFatal
		}
	}
	logging.SetGlobalLevel(logLevel)

	ts.t.Logf("Logs will be written to: %s (log level: %s)", ts.logFile, logLevel.String())

	// Copy workspace template
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	if err := CopyDir(ts.Config.WorkspaceDir, workspaceDir); err != nil {
		return fmt.Errorf("failed to copy workspace template: %w", err)
	}
	ts.WorkspaceDir = workspaceDir
	ts.t.Logf("Copied workspace from %s to %s", ts.Config.WorkspaceDir, workspaceDir)

	// Create and initialize LSP client
	var client *lsp.Client
	if ts.Config.HeadlessListenArg != "" {
		// Start LSP in listen mode (same Command/Args as NewClient), then connect via NewClientHeadless
		addr, cmd, err := ts.startLSPInListenMode()
		if err != nil {
			return err
		}
		ts.headlessCmd = cmd
		ts.headless = true
		client, err = lsp.NewClientHeadless(addr)
		if err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return fmt.Errorf("failed to connect to LSP at %s: %w", addr, err)
		}
		ts.Client = client
		ts.t.Logf("Started LSP in listen mode and connected at %s", addr)
	} else if ts.Config.ConnectAddr != "" {
		return fmt.Errorf("headless via ConnectAddr is disabled; use HeadlessListenArg to run headless (start LSP in listen mode and connect)")
	} else {
		var err error
		client, err = lsp.NewClient(ts.Config.Command, ts.Config.Args...)
		if err != nil {
			return fmt.Errorf("failed to create LSP client: %w", err)
		}
		ts.Client = client
		ts.t.Logf("Started LSP: %s %v", ts.Config.Command, ts.Config.Args)
	}

	// Initialize LSP and set up file watcher
	initResult, err := client.InitializeLSPClient(ts.Context, workspaceDir)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}
	ts.t.Logf("LSP initialized with capabilities: %+v", initResult.Capabilities)

	ts.Watcher = watcher.NewWorkspaceWatcher(client)
	go ts.Watcher.WatchWorkspace(ts.Context, workspaceDir)

	if err := client.WaitForServerReady(ts.Context); err != nil {
		return fmt.Errorf("server failed to become ready: %w", err)
	}

	// Give watcher time to set up and scan workspace
	initializeTime := 1000 // Default 1 second
	if ts.Config.InitializeTimeMs > 0 {
		initializeTime = ts.Config.InitializeTimeMs
	}
	ts.t.Logf("Waiting %d ms for LSP to initialize", initializeTime)
	time.Sleep(time.Duration(initializeTime) * time.Millisecond)

	ts.initialized = true
	return nil
}

// Cleanup stops the LSP and cleans up resources
func (ts *TestSuite) Cleanup() {
	ts.cleanupOnce.Do(func() {
		ts.t.Logf("Cleaning up test suite")

		// Cancel context to stop watchers
		ts.Cancel()

		// Shutdown LSP: for subprocess we shutdown+exit+close; for headless we only close unless we started the process
		if ts.Client != nil {
			if !ts.headless {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				ts.t.Logf("Shutting down LSP client")
				if err := ts.Client.Shutdown(shutdownCtx); err != nil {
					ts.t.Logf("Shutdown failed: %v", err)
				}
				if err := ts.Client.Exit(shutdownCtx); err != nil {
					ts.t.Logf("Exit failed: %v", err)
				}
			} else if ts.headlessCmd != nil {
				// We started the LSP in listen mode; send shutdown/exit so it exits gracefully
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				ts.t.Logf("Shutting down LSP client (headless subprocess)")
				if err := ts.Client.Shutdown(shutdownCtx); err != nil {
					ts.t.Logf("Shutdown failed: %v", err)
				}
				if err := ts.Client.Exit(shutdownCtx); err != nil {
					ts.t.Logf("Exit failed: %v", err)
				}
			}
			if err := ts.Client.Close(); err != nil {
				ts.t.Logf("Close failed: %v", err)
			}
		}
		if ts.headlessCmd != nil {
			done := make(chan struct{})
			go func() {
				_ = ts.headlessCmd.Wait()
				close(done)
			}()
			select {
			case <-done:
				// process exited
			case <-time.After(3 * time.Second):
				if ts.headlessCmd.Process != nil {
					ts.t.Logf("Killing LSP process after timeout")
					_ = ts.headlessCmd.Process.Kill()
					_ = ts.headlessCmd.Wait()
				}
			}
		}

		// No need to close log files explicitly, logging package handles that

		ts.t.Logf("Test artifacts are in: %s", ts.TempDir)
		ts.t.Logf("Log file: %s", ts.logFile)
		ts.t.Logf("To clean up, run: rm -rf %s", ts.TempDir)
	})
}

// ReadFile reads a file from the workspace
func (ts *TestSuite) ReadFile(relPath string) (string, error) {
	path := filepath.Join(ts.WorkspaceDir, relPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return string(data), nil
}

// WriteFile writes content to a file in the workspace
func (ts *TestSuite) WriteFile(relPath, content string) error {
	path := filepath.Join(ts.WorkspaceDir, relPath)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	// Give the watcher time to detect the file change
	time.Sleep(500 * time.Millisecond)
	return nil
}
