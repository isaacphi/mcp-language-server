package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
)

var lspManagerLogger = logging.NewLogger(logging.Core)

// LSPInstance represents a running LSP server instance
type LSPInstance struct {
	ID               string
	Language         string
	WorkspacePath    string
	Client           *lsp.Client
	WorkspaceWatcher *watcher.WorkspaceWatcher
	Ctx              context.Context
	CancelFunc       context.CancelFunc
	Status           string // "running", "stopped"
}

// LSPManager manages multiple LSP server instances
type LSPManager struct {
	instances   map[string]*LSPInstance
	mu          sync.RWMutex
	defaultLSP  string // ID of the default/selected LSP
	config      *Config
}

// NewLSPManager creates a new LSP manager
func NewLSPManager(config *Config) *LSPManager {
	return &LSPManager{
		instances: make(map[string]*LSPInstance),
		config:    config,
	}
}

// StartLSP starts a new LSP instance for the given language and workspace
func (m *LSPManager) StartLSP(ctx context.Context, workspacePath, language string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate workspace directory
	workspaceDir, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return "", fmt.Errorf("workspace directory does not exist: %s", workspaceDir)
	}

	// Get LSP configuration for the language
	lspConfig, ok := m.config.LSPs[language]
	if !ok {
		return "", fmt.Errorf("no LSP configuration found for language: %s", language)
	}

	// Validate LSP command
	if _, err := exec.LookPath(lspConfig.Command); err != nil {
		return "", fmt.Errorf("LSP command not found: %s", lspConfig.Command)
	}

	// Generate unique ID
	id := uuid.New().String()

	// Create context for this LSP instance
	instanceCtx, cancel := context.WithCancel(ctx)

	// Create LSP client
	client, err := lsp.NewClient(lspConfig.Command, lspConfig.Args...)
	if err != nil {
		cancel()
		return "", fmt.Errorf("failed to create LSP client: %v", err)
	}

	// Set environment variables if specified
	if len(lspConfig.Env) > 0 {
		env := os.Environ()
		for key, value := range lspConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		client.Cmd.Env = env
	}

	// Set as default LSP
	m.defaultLSP = id

	// Initialize the LSP client
	currentDir, err := os.Getwd()
	if err != nil {
		cancel()
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(workspaceDir); err != nil {
		cancel()
		return "", fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	initResult, err := client.InitializeLSPClient(instanceCtx, workspaceDir)
	if err != nil {
		os.Chdir(currentDir)
		cancel()
		return "", fmt.Errorf("initialize failed: %v", err)
	}

	// Restore original directory
	os.Chdir(currentDir)

	lspManagerLogger.Debug("LSP %s server capabilities: %+v", id, initResult.Capabilities)

	// Create workspace watcher
	workspaceWatcher := watcher.NewWorkspaceWatcher(client)
	go workspaceWatcher.WatchWorkspace(instanceCtx, workspaceDir)

	// Wait for server to be ready
	if err := client.WaitForServerReady(instanceCtx); err != nil {
		cancel()
		return "", fmt.Errorf("failed to wait for server ready: %v", err)
	}

	// Store the instance
	instance := &LSPInstance{
		ID:               id,
		Language:         language,
		WorkspacePath:    workspaceDir,
		Client:           client,
		WorkspaceWatcher: workspaceWatcher,
		Ctx:              instanceCtx,
		CancelFunc:       cancel,
		Status:           "running",
	}

	m.instances[id] = instance

	lspManagerLogger.Info("Started LSP instance %s for language %s in workspace %s", id, language, workspaceDir)

	return id, nil
}

// StopLSP stops an LSP instance by ID
func (m *LSPManager) StopLSP(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("LSP instance not found: %s", id)
	}

	if instance.Status == "stopped" {
		return fmt.Errorf("LSP instance %s is already stopped", id)
	}

	lspManagerLogger.Info("Stopping LSP instance %s", id)

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if instance.Client != nil {
		lspManagerLogger.Debug("Closing open files for instance %s", id)
		instance.Client.CloseAllFiles(ctx)

		// Create a shorter timeout context for the shutdown request
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer shutdownCancel()

		// Run shutdown in a goroutine with timeout
		shutdownDone := make(chan struct{})
		go func() {
			lspManagerLogger.Debug("Sending shutdown request to instance %s", id)
			if err := instance.Client.Shutdown(shutdownCtx); err != nil {
				lspManagerLogger.Error("Shutdown request failed for instance %s: %v", id, err)
			}
			close(shutdownDone)
		}()

		// Wait for shutdown with timeout
		select {
		case <-shutdownDone:
			lspManagerLogger.Debug("Shutdown request completed for instance %s", id)
		case <-time.After(1 * time.Second):
			lspManagerLogger.Warn("Shutdown request timed out for instance %s", id)
		}

		lspManagerLogger.Debug("Sending exit notification to instance %s", id)
		if err := instance.Client.Exit(ctx); err != nil {
			lspManagerLogger.Error("Exit notification failed for instance %s: %v", id, err)
		}

		lspManagerLogger.Debug("Closing LSP client for instance %s", id)
		if err := instance.Client.Close(); err != nil {
			lspManagerLogger.Error("Failed to close LSP client for instance %s: %v", id, err)
		}
	}

	// Cancel the instance context
	instance.CancelFunc()
	instance.Status = "stopped"

	// Update default LSP if this was the default instance
	if m.defaultLSP == id {
		m.defaultLSP = ""
		// Find another running instance to set as default
		for instanceID, inst := range m.instances {
			if inst.Status == "running" {
				m.defaultLSP = instanceID
				break
			}
		}
	}

	delete(m.instances, id)

	lspManagerLogger.Info("Stopped LSP instance %s", id)
	return nil
}

// GetLSP retrieves an LSP instance by ID
func (m *LSPManager) GetLSP(id string) (*LSPInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, ok := m.instances[id]
	if !ok {
		return nil, fmt.Errorf("LSP instance not found: %s", id)
	}

	if instance.Status != "running" {
		return nil, fmt.Errorf("LSP instance %s is not running", id)
	}

	return instance, nil
}

// GetDefaultLSP retrieves the default/selected LSP instance
func (m *LSPManager) GetDefaultLSP() (*LSPInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.defaultLSP == "" {
		return nil, fmt.Errorf("no LSP instance has been started. Use lsp_start tool first")
	}

	instance, ok := m.instances[m.defaultLSP]
	if !ok || instance.Status != "running" {
		return nil, fmt.Errorf("default LSP instance is no longer available")
	}

	return instance, nil
}

// ListLSPs returns information about all running LSP instances
func (m *LSPManager) ListLSPs() []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]map[string]string, 0, len(m.instances))
	for _, instance := range m.instances {
		result = append(result, map[string]string{
			"id":        instance.ID,
			"language":  instance.Language,
			"workspace": instance.WorkspacePath,
			"status":    instance.Status,
		})
	}

	return result
}

// StopAll stops all LSP instances
func (m *LSPManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.instances {
		m.mu.Unlock()
		m.StopLSP(id)
		m.mu.Lock()
	}
}

// ResolveLSPInstance returns the LSP instance to use based on the optional ID
// If id is empty, returns the default LSP
func (m *LSPManager) ResolveLSPInstance(id string) (*LSPInstance, error) {
	if id == "" {
		return m.GetDefaultLSP()
	}
	return m.GetLSP(id)
}

// SelectLSP sets the specified LSP instance as the default
func (m *LSPManager) SelectLSP(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("LSP instance with ID %s not found", id)
	}

	if instance.Status != "running" {
		return fmt.Errorf("LSP instance %s is not running (status: %s)", id, instance.Status)
	}

	m.defaultLSP = id
	return nil
}

// SaveSession saves the current LSP instances to a session file
func (m *LSPManager) SaveSession(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session := &LSPSession{
		LSPs: make([]LSPSessionEntry, 0, len(m.instances)),
	}

	for _, instance := range m.instances {
		if instance.Status == "running" {
			session.LSPs = append(session.LSPs, LSPSessionEntry{
				Workspace: instance.WorkspacePath,
				Language:  instance.Language,
			})
		}
	}

	lspManagerLogger.Info("Saving session with %d LSP instances to %s", len(session.LSPs), filepath)
	return SaveSessionFile(filepath, session)
}

// LoadSession loads LSP instances from a session file
// It stops LSPs not in the session and starts LSPs that are in the session
func (m *LSPManager) LoadSession(ctx context.Context, filepath string) error {
	session, err := LoadSessionFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to load session file: %w", err)
	}

	lspManagerLogger.Info("Loading session with %d LSP instances from %s", len(session.LSPs), filepath)

	// Create a map of desired LSPs (workspace+language -> entry)
	desiredLSPs := make(map[string]LSPSessionEntry)
	for _, entry := range session.LSPs {
		key := fmt.Sprintf("%s:%s", entry.Workspace, entry.Language)
		desiredLSPs[key] = entry
	}

	// Get current running LSPs
	m.mu.RLock()
	currentLSPs := make(map[string]*LSPInstance)
	for _, instance := range m.instances {
		if instance.Status == "running" {
			key := fmt.Sprintf("%s:%s", instance.WorkspacePath, instance.Language)
			currentLSPs[key] = instance
		}
	}
	m.mu.RUnlock()

	// Stop LSPs that are not in the session
	for key, instance := range currentLSPs {
		if _, exists := desiredLSPs[key]; !exists {
			lspManagerLogger.Info("Stopping LSP not in session: %s (workspace=%s, language=%s)",
				instance.ID, instance.WorkspacePath, instance.Language)
			if err := m.StopLSP(instance.ID); err != nil {
				lspManagerLogger.Error("Failed to stop LSP %s: %v", instance.ID, err)
			}
		}
	}

	// Start LSPs that are in the session but not running
	for key, entry := range desiredLSPs {
		if _, exists := currentLSPs[key]; !exists {
			lspManagerLogger.Info("Starting LSP from session: workspace=%s, language=%s",
				entry.Workspace, entry.Language)
			_, err := m.StartLSP(ctx, entry.Workspace, entry.Language)
			if err != nil {
				lspManagerLogger.Error("Failed to start LSP for %s/%s: %v",
					entry.Workspace, entry.Language, err)
				// Continue with other LSPs even if one fails
			}
		}
	}

	lspManagerLogger.Info("Session loaded successfully")
	return nil
}
