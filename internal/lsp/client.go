package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	// stdinMu serializes writes to stdin. A single LSP message is framed as a
	// Content-Length header followed by the body in two separate Write calls, so
	// concurrent senders (request/response, notifications, the workspace watcher)
	// must not interleave or the wire framing is corrupted.
	stdinMu sync.Mutex

	// Request ID counter
	nextID atomic.Int32

	// Response handlers
	handlers   map[string]chan *Message
	handlersMu sync.RWMutex

	// Server request handlers
	serverRequestHandlers map[string]ServerRequestHandler
	serverHandlersMu      sync.RWMutex

	// Notification handlers
	notificationHandlers map[string]NotificationHandler
	notificationMu       sync.RWMutex

	// Diagnostic cache
	diagnostics   map[protocol.DocumentUri][]protocol.Diagnostic
	diagnosticsMu sync.RWMutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex

	// closed is closed exactly once, the moment the transport starts going away —
	// either because Close() began tearing it down, or because handleMessages
	// stopped reading (LSP server died: stdout EOF or crash). An in-flight Call
	// selects on it so a request whose response can no longer arrive does not
	// block forever, whether the shutdown was initiated by us or forced on us by a
	// dead server. It is a best-effort fast-fail for new Call/Notify, not a write
	// barrier: stdinMu serializes the actual writes, and a write that loses the
	// race with stdin.Close() simply returns an error.
	//
	// closedOnce guards the channel close so the Close() path and the
	// handleMessages-exit path can both signal it without a double-close panic.
	// It is distinct from closeOnce, which serializes the full teardown (stdin
	// close + Cmd.Wait) and must run only from Close(), never from the read
	// goroutine.
	closeOnce  sync.Once
	closedOnce sync.Once
	closed     chan struct{}
}

// signalClosed marks the transport as going away, unblocking any in-flight Call.
// Safe to call from multiple goroutines and more than once.
func (c *Client) signalClosed() {
	c.closedOnce.Do(func() {
		close(c.closed)
	})
}

func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	// Copy env
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &Client{
		Cmd:                   cmd,
		stdin:                 stdin,
		stdout:                bufio.NewReader(stdout),
		stderr:                stderr,
		handlers:              make(map[string]chan *Message),
		notificationHandlers:  make(map[string]NotificationHandler),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		diagnostics:           make(map[protocol.DocumentUri][]protocol.Diagnostic),
		openFiles:             make(map[string]*OpenFileInfo),
		closed:                make(chan struct{}),
	}

	// Start the LSP server process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Handle stderr in a separate goroutine with proper logging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			processLogger.Info("%s", line)
		}
		if err := scanner.Err(); err != nil {
			lspLogger.Error("Error reading LSP server stderr: %v", err)
		}
	}()

	// Start message handling loop
	go client.handleMessages()

	return client, nil
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	c.notificationHandlers[method] = handler
}

func (c *Client) RegisterServerRequestHandler(method string, handler ServerRequestHandler) {
	c.serverHandlersMu.Lock()
	defer c.serverHandlersMu.Unlock()
	c.serverRequestHandlers[method] = handler
}

func (c *Client) InitializeLSPClient(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	initParams := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: []protocol.WorkspaceFolder{
				{
					URI:  protocol.URI("file://" + workspaceDir),
					Name: workspaceDir,
				},
			},
		},

		XInitializeParams: protocol.XInitializeParams{
			ProcessID: int32(os.Getpid()),
			ClientInfo: &protocol.ClientInfo{
				Name:    "mcp-language-server",
				Version: "0.1.0",
			},
			RootPath: workspaceDir,
			RootURI:  protocol.DocumentUri("file://" + workspaceDir),
			Capabilities: protocol.ClientCapabilities{
				Workspace: protocol.WorkspaceClientCapabilities{
					Configuration: true,
					DidChangeConfiguration: protocol.DidChangeConfigurationClientCapabilities{
						DynamicRegistration: true,
					},
					DidChangeWatchedFiles: protocol.DidChangeWatchedFilesClientCapabilities{
						DynamicRegistration:    true,
						RelativePatternSupport: true,
					},
				},
				TextDocument: protocol.TextDocumentClientCapabilities{
					Synchronization: &protocol.TextDocumentSyncClientCapabilities{
						DynamicRegistration: true,
						DidSave:             true,
					},
					Completion: protocol.CompletionClientCapabilities{
						CompletionItem: protocol.ClientCompletionItemOptions{},
					},
					CodeLens: &protocol.CodeLensClientCapabilities{
						DynamicRegistration: true,
					},
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{},
					CodeAction: protocol.CodeActionClientCapabilities{
						CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
							CodeActionKind: protocol.ClientCodeActionKindOptions{
								ValueSet: []protocol.CodeActionKind{},
							},
						},
					},
					PublishDiagnostics: protocol.PublishDiagnosticsClientCapabilities{
						VersionSupport: true,
					},
					SemanticTokens: protocol.SemanticTokensClientCapabilities{
						Requests: protocol.ClientSemanticTokensRequestOptions{
							Range: &protocol.Or_ClientSemanticTokensRequestOptions_range{},
							Full:  &protocol.Or_ClientSemanticTokensRequestOptions_full{},
						},
						TokenTypes:     []string{},
						TokenModifiers: []string{},
						Formats:        []protocol.TokenFormat{},
					},
				},
				Window: protocol.WindowClientCapabilities{},
			},
			InitializationOptions: map[string]any{
				"codelenses": map[string]bool{
					"generate":           true,
					"regenerate_cgo":     true,
					"test":               true,
					"tidy":               true,
					"upgrade_dependency": true,
					"vendor":             true,
					"vulncheck":          false,
				},
			},
		},
	}

	var result protocol.InitializeResult
	if err := c.Call(ctx, "initialize", initParams, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	if err := c.Notify(ctx, "initialized", struct{}{}); err != nil {
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}

	// Register handlers
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics",
		func(params json.RawMessage) { HandleDiagnostics(c, params) })

	// Notify the LSP server
	err := c.Initialized(ctx, protocol.InitializedParams{})
	if err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	// LSP sepecific Initialization
	path := strings.ToLower(c.Cmd.Path)
	switch {
	case strings.Contains(path, "typescript-language-server"):
		err := initializeTypescriptLanguageServer(ctx, c, workspaceDir)
		if err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func (c *Client) Close() error {
	var err error
	// Guard against a double Close: closing c.closed (and stdin) twice would
	// panic, and Close can race in from more than one shutdown trigger.
	c.closeOnce.Do(func() {
		err = c.closeLocked()
	})
	return err
}

func (c *Client) closeLocked() error {
	// Try to close all open files first (graceful didClose notifications still
	// need the transport, so do this before signalling c.closed below).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to close files but continue shutdown regardless
	c.CloseAllFiles(ctx)

	// From here the transport is going away. Unblock any in-flight Call waiting
	// on a response and make new Call/Notify fail fast instead of writing to a
	// pipe we are about to close. handleMessages may have already signalled this
	// if the server died first, so go through the once-guarded helper.
	c.signalClosed()

	// Force kill the LSP process if it doesn't exit within timeout.
	// killDone is closed exactly once, by this function, after Cmd.Wait
	// returns. The kill goroutine only ever *reads* it (never closes it), so
	// the two paths can never double-close the same channel: when the process
	// exits on its own we close killDone and the goroutine takes its <-killDone
	// arm; when it hangs the goroutine kills it and then observes killDone
	// closing once Wait returns.
	killDone := make(chan struct{})
	go func() {
		select {
		case <-time.After(2 * time.Second):
			lspLogger.Warn("LSP process did not exit within timeout, forcing kill")
			if c.Cmd.Process != nil {
				if err := c.Cmd.Process.Kill(); err != nil {
					lspLogger.Error("Failed to kill process: %v", err)
				} else {
					lspLogger.Info("Process killed successfully")
				}
			}
		case <-killDone:
			// Process exited on its own before the timeout; nothing to do.
		}
	}()

	// Close stdin to signal the server. Deliberately NOT under stdinMu: a writer
	// blocked on a full pipe holds that lock, and closing the fd here is what
	// unblocks it (the in-flight write then returns an error). os.File guards
	// concurrent Write/Close internally, so this is a fast-fail, not a race.
	if err := c.stdin.Close(); err != nil {
		lspLogger.Error("Failed to close stdin: %v", err)
	}

	// Wait for process to exit
	err := c.Cmd.Wait()
	close(killDone) // Stop the force kill goroutine

	return err
}

// isClosed reports whether the transport is going away — either Close() began
// tearing it down or the read loop exited because the server died. It mirrors
// the broadened semantics of the closed channel (see field comment above).
func (c *Client) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
)

func (c *Client) WaitForServerReady(ctx context.Context) error {
	// TODO: wait for specific messages or poll workspace/symbol
	time.Sleep(time.Second * 1)
	return nil
}

type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentUri
}

func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	uri := fmt.Sprintf("file://%s", filepath)

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; exists {
		c.openFilesMu.Unlock()
		return nil // Already open
	}
	c.openFilesMu.Unlock()

	// Skip files that do not exist or cannot be read
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentUri(uri),
			LanguageID: DetectLanguageID(uri),
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.Notify(ctx, "textDocument/didOpen", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	c.openFiles[uri] = &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentUri(uri),
	}
	c.openFilesMu.Unlock()

	lspLogger.Debug("Opened file: %s", filepath)

	return nil
}

func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	uri := fmt.Sprintf("file://%s", filepath)

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	c.openFilesMu.Lock()
	fileInfo, isOpen := c.openFiles[uri]
	if !isOpen {
		c.openFilesMu.Unlock()
		return fmt.Errorf("cannot notify change for unopened file: %s", filepath)
	}

	// Increment version
	fileInfo.Version++
	version := fileInfo.Version
	c.openFilesMu.Unlock()

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentUri(uri),
			},
			Version: version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		},
	}

	return c.Notify(ctx, "textDocument/didChange", params)
}

func (c *Client) CloseFile(ctx context.Context, filepath string) error {
	uri := fmt.Sprintf("file://%s", filepath)

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; !exists {
		c.openFilesMu.Unlock()
		return nil // Already closed
	}
	c.openFilesMu.Unlock()

	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri(uri),
		},
	}
	lspLogger.Debug("Closing file: %s", params.TextDocument.URI.Dir())
	if err := c.Notify(ctx, "textDocument/didClose", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	delete(c.openFiles, uri)
	c.openFilesMu.Unlock()

	return nil
}

func (c *Client) IsFileOpen(filepath string) bool {
	uri := fmt.Sprintf("file://%s", filepath)
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// CloseAllFiles closes all currently open files
func (c *Client) CloseAllFiles(ctx context.Context) {
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))

	// First collect all URIs that need to be closed
	for uri := range c.openFiles {
		// Convert URI back to file path by trimming "file://" prefix
		filePath := strings.TrimPrefix(uri, "file://")
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock()

	// Then close them all
	for _, filePath := range filesToClose {
		err := c.CloseFile(ctx, filePath)
		if err != nil {
			lspLogger.Error("Error closing file %s: %v", filePath, err)
		}
	}

	lspLogger.Debug("Closed %d files", len(filesToClose))
}

func (c *Client) GetFileDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}
