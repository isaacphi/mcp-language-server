package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	selectedLSP string // ID of the selected LSP
	config      *Config
}

// NewLSPManager creates a new LSP manager
func NewLSPManager(config *Config) *LSPManager {
	return &LSPManager{
		instances: make(map[string]*LSPInstance),
		config:    config,
	}
}

// StartLSP starts a new LSP instance for the given language and workspace.
// duringStartup indicates if this is called during initial startup (true) or at runtime (false).
// When true, auto-selection is skipped to avoid non-determinism from map iteration order.
func (m *LSPManager) StartLSP(workspacePath, language string, duringStartup bool) (string, error) {
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
	// In single-LSP and session mode, look up in LSPs map (language -> workspace -> config)
	// In unbounded mode, look up in defaults and merge with workspace
	var lspConfig LSPConfig

	// Try exact workspace in pre-configured LSPs (session or single-LSP mode)
	if workspaceConfigs, ok := m.config.LSPs[language]; ok {
		if cfg, ok := workspaceConfigs[workspaceDir]; ok {
			// Found exact match in pre-configured LSPs
			lspConfig = cfg
		} else if lspDefault, ok := m.config.Defaults[language]; ok {
			// Language configured but not for this workspace, use default + override
			lspConfig = LSPConfig{
				LSPDefaultConfig: lspDefault,
				Workspace:        workspaceDir,
			}
		} else {
			return "", fmt.Errorf("no LSP configuration found for language: %s", language)
		}
	} else if lspDefault, ok := m.config.Defaults[language]; ok {
		// Not in pre-configured LSPs, use default (unbounded mode)
		lspConfig = LSPConfig{
			LSPDefaultConfig: lspDefault,
			Workspace:        workspaceDir,
		}
	} else {
		return "", fmt.Errorf("no LSP configuration found for language: %s", language)
	}

	// Validate LSP command
	if _, err := exec.LookPath(lspConfig.Command); err != nil {
		return "", fmt.Errorf("LSP command not found: %s", lspConfig.Command)
	}

	// Generate unique ID
	id := uuid.New().String()

	// Create context for this LSP instance
	instanceCtx, cancel := context.WithCancel(context.Background())

	// Build environment variables for the LSP process
	var lspEnv map[string]string
	if len(lspConfig.Env) > 0 {
		// Start with parent process environment
		lspEnv = make(map[string]string)
		for _, envStr := range os.Environ() {
			parts := strings.SplitN(envStr, "=", 2)
			if len(parts) == 2 {
				lspEnv[parts[0]] = parts[1]
			}
		}
		// Override with configured environment variables
		for key, value := range lspConfig.Env {
			lspEnv[key] = value
		}
	}

	// Create LSP client with workspace directory and environment variables
	// NewClient will set cmd.Dir and cmd.Env before starting the process
	client, err := lsp.NewClient(lspConfig.Command, workspaceDir, lspEnv, lspConfig.Args...)
	if err != nil {
		cancel()
		return "", fmt.Errorf("failed to create LSP client: %v", err)
	}

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

	// Auto-select if this is the only LSP instance and not during startup
	// (During startup, skip auto-select to avoid non-determinism from random map iteration order)
	if !duringStartup && len(m.instances) == 1 {
		m.selectedLSP = id
		lspManagerLogger.Debug("Auto-selected LSP instance %s (only running instance)", id)
	}

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

	// Clear selected LSP if this was the selected instance
	if m.selectedLSP == id {
		m.selectedLSP = ""
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

// GetSelectedLSP retrieves the currently selected LSP instance
func (m *LSPManager) GetSelectedLSP() (*LSPInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.selectedLSP == "" {
		if len(m.instances) == 0 {
			return nil, fmt.Errorf("no LSP instance has been started. Use lsp_start tool first")
		}
		return nil, fmt.Errorf("no LSP selected. Use lsp_select(id) to choose one")
	}

	instance, ok := m.instances[m.selectedLSP]
	if !ok || instance.Status != "running" {
		return nil, fmt.Errorf("selected LSP instance is no longer available")
	}

	return instance, nil
}

// ListLSPs returns information about all running LSP instances
func (m *LSPManager) ListLSPs() []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]map[string]string, 0, len(m.instances))
	for _, instance := range m.instances {
		selected := "false"
		if m.selectedLSP == instance.ID {
			selected = "true"
		}
		result = append(result, map[string]string{
			"id":        instance.ID,
			"language":  instance.Language,
			"workspace": instance.WorkspacePath,
			"status":    instance.Status,
			"selected":  selected,
		})
	}

	return result
}

// StopAll stops all LSP instances
func (m *LSPManager) StopAll() {
	m.mu.Lock()
	// Snapshot the instance IDs while holding the lock
	ids := make([]string, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	// Stop each LSP outside the lock to avoid data race during iteration
	for _, id := range ids {
		m.StopLSP(id)
	}
}

// ResolveLSPInstance returns the LSP instance to use based on the optional ID
// If id is empty, returns the selected LSP
func (m *LSPManager) ResolveLSPInstance(id string) (*LSPInstance, error) {
	if id == "" {
		return m.GetSelectedLSP()
	}
	return m.GetLSP(id)
}

// SelectLSP sets the specified LSP instance as selected
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

	m.selectedLSP = id
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
func (m *LSPManager) LoadSession(filepath string) error {
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
			_, err := m.StartLSP(entry.Workspace, entry.Language, false)
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
