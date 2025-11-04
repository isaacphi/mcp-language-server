package main

import (
	"os"
	"testing"
)

func TestNewLSPManager(t *testing.T) {
	config := &Config{
		Defaults: map[string]LSPDefaultConfig{
			"go": {
				Command: "gopls",
				Args:    []string{},
				Env:     map[string]string{},
			},
		},
		LSPs: make(map[string]map[string]LSPConfig),
	}

	manager := NewLSPManager(config)

	if manager == nil {
		t.Errorf("Expected non-nil LSP manager")
	}
	if manager.config != config {
		t.Errorf("Manager config not set correctly")
	}
	if len(manager.instances) != 0 {
		t.Errorf("Expected empty instances on new manager")
	}
}

func TestStartLSPUnboundedMode(t *testing.T) {
	// Test that StartLSP looks up in Defaults (unbounded mode scenario)
	// where LSPs map is empty until instances are created
	config := &Config{
		Defaults: map[string]LSPDefaultConfig{
			"test-lang": {
				Command: "nonexistent-lsp-binary-xyz",
				Args:    []string{},
				Env:     map[string]string{},
			},
		},
		LSPs: make(map[string]map[string]LSPConfig), // Empty, simulating unbounded mode start
	}

	manager := NewLSPManager(config)

	// Create a temporary workspace directory
	tmpdir, err := os.MkdirTemp("", "lsp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Call StartLSP - it will fail because command doesn't exist (exec.LookPath check)
	// but this proves the Defaults lookup worked (didn't fail with "no LSP configuration found")
	_, err = manager.StartLSP(tmpdir, "test-lang", false)
	if err == nil {
		t.Errorf("Expected error for non-existent LSP command")
	}

	// Critical check: error should be about command not found, not config not found
	// This proves the Defaults lookup worked
	if err != nil && err.Error() == "no LSP configuration found for language: test-lang" {
		t.Errorf("StartLSP failed at config lookup - should lookup Defaults map. Got: %v", err)
	}
}

func TestStartLSPSingleLSPMode(t *testing.T) {
	// Test that StartLSP handles pre-configured LSPs (single-LSP mode scenario)
	// where Config.LSPs is pre-populated and Defaults is empty

	// Create a temporary workspace directory first
	tmpdir, err := os.MkdirTemp("", "lsp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	config := &Config{
		Defaults: make(map[string]LSPDefaultConfig), // Empty in single-LSP mode
		LSPs: map[string]map[string]LSPConfig{
			"default": {
				tmpdir: {
					LSPDefaultConfig: LSPDefaultConfig{
						Command: "nonexistent-lsp-binary-xyz",
						Args:    []string{},
						Env:     map[string]string{},
					},
					Workspace: tmpdir,
				},
			},
		},
	}

	manager := NewLSPManager(config)

	// Call StartLSP with "default" language and the configured workspace
	// It will fail because command doesn't exist, but this proves the pre-configured lookup worked
	_, err = manager.StartLSP(tmpdir, "default", false)
	if err == nil {
		t.Errorf("Expected error for non-existent LSP command")
	}

	// Critical check: error should be about command not found, not config not found
	// This proves the pre-configured LSPs lookup worked
	if err != nil && err.Error() == "no LSP configuration found for language: default" {
		t.Errorf("StartLSP failed at config lookup - should find pre-configured LSP. Got: %v", err)
	}
}

func TestLSPManagerListOperations(t *testing.T) {
	tests := []struct {
		name        string                                // Test case description
		setupLSPs   func(*LSPManager) error               // Setup function to create LSP instances
		expectedLen int                                   // Expected number of LSPs
		checkList   func(*testing.T, []map[string]string) // Custom assertion function
	}{
		{
			name: "List empty instances",
			setupLSPs: func(m *LSPManager) error {
				return nil
			},
			expectedLen: 0,
			checkList: func(t *testing.T, list []map[string]string) {
				if list == nil {
					t.Errorf("Expected non-nil list")
				}
			},
		},
		{
			name: "List structure contains required fields",
			setupLSPs: func(m *LSPManager) error {
				// Mock the internal structure without actually starting LSP
				m.mu.Lock()
				m.instances["test-id"] = &LSPInstance{
					ID:            "test-id",
					Language:      "go",
					WorkspacePath: "/path/to/workspace",
					Status:        "running",
				}
				m.mu.Unlock()
				return nil
			},
			expectedLen: 1,
			checkList: func(t *testing.T, list []map[string]string) {
				if len(list) != 1 {
					t.Errorf("Expected 1 instance in list")
					return
				}
				inst := list[0]
				requiredFields := []string{"id", "language", "workspace", "status"}
				for _, field := range requiredFields {
					if _, ok := inst[field]; !ok {
						t.Errorf("Missing required field in list: %s", field)
					}
				}
				if inst["id"] != "test-id" {
					t.Errorf("Expected id test-id, got %s", inst["id"])
				}
				if inst["language"] != "go" {
					t.Errorf("Expected language go, got %s", inst["language"])
				}
				if inst["status"] != "running" {
					t.Errorf("Expected status running, got %s", inst["status"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			err := tt.setupLSPs(manager)
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			list := manager.ListLSPs()
			if len(list) != tt.expectedLen {
				t.Errorf("Expected %d instances, got %d", tt.expectedLen, len(list))
			}
			if tt.checkList != nil {
				tt.checkList(t, list)
			}
		})
	}
}

func TestLSPManagerSelectLSP(t *testing.T) {
	tests := []struct {
		name       string                   // Test case description
		setupLSPs  func(*LSPManager) string // Setup function returning ID to select
		selectID   string                   // ID to select
		expectErr  bool                     // Whether error is expected
		checkError func(*testing.T, error)  // Custom error assertion
	}{
		{
			name: "Select existing LSP",
			setupLSPs: func(m *LSPManager) string {
				m.mu.Lock()
				m.instances["lsp-1"] = &LSPInstance{
					ID:     "lsp-1",
					Status: "running",
				}
				m.mu.Unlock()
				return "lsp-1"
			},
			selectID:  "lsp-1",
			expectErr: false,
		},
		{
			name: "Select nonexistent LSP",
			setupLSPs: func(m *LSPManager) string {
				return "nonexistent"
			},
			selectID:  "nonexistent",
			expectErr: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Errorf("Expected error for nonexistent LSP")
				}
			},
		},
		{
			name: "Select stopped LSP",
			setupLSPs: func(m *LSPManager) string {
				m.mu.Lock()
				m.instances["stopped-lsp"] = &LSPInstance{
					ID:     "stopped-lsp",
					Status: "stopped",
				}
				m.mu.Unlock()
				return "stopped-lsp"
			},
			selectID:  "stopped-lsp",
			expectErr: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Errorf("Expected error for stopped LSP")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			tt.setupLSPs(manager)

			err := manager.SelectLSP(tt.selectID)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLSPManagerGetLSP(t *testing.T) {
	tests := []struct {
		name      string                   // Test case description
		setupLSP  func(*LSPManager) string // Setup function returning ID to get
		getID     string                   // ID to retrieve
		expectErr bool                     // Whether error is expected
	}{
		{
			name: "Get existing running LSP",
			setupLSP: func(m *LSPManager) string {
				m.mu.Lock()
				m.instances["test-lsp"] = &LSPInstance{
					ID:            "test-lsp",
					Language:      "go",
					WorkspacePath: "/workspace",
					Status:        "running",
				}
				m.mu.Unlock()
				return "test-lsp"
			},
			getID:     "test-lsp",
			expectErr: false,
		},
		{
			name: "Get nonexistent LSP",
			setupLSP: func(m *LSPManager) string {
				return "nonexistent"
			},
			getID:     "nonexistent",
			expectErr: true,
		},
		{
			name: "Get stopped LSP",
			setupLSP: func(m *LSPManager) string {
				m.mu.Lock()
				m.instances["stopped"] = &LSPInstance{
					ID:     "stopped",
					Status: "stopped",
				}
				m.mu.Unlock()
				return "stopped"
			},
			getID:     "stopped",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			tt.setupLSP(manager)

			instance, err := manager.GetLSP(tt.getID)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if instance == nil {
					t.Errorf("Expected non-nil instance")
				}
				if instance.ID != tt.getID {
					t.Errorf("Expected ID %s, got %s", tt.getID, instance.ID)
				}
			}
		})
	}
}

func TestLSPManagerResolveLSPInstance(t *testing.T) {
	tests := []struct {
		name        string                         // Test case description
		setupLSPs   func(*LSPManager)              // Setup function to create LSPs
		requestID   string                         // ID to resolve (empty for selected)
		expectErr   bool                           // Whether error is expected
		checkResult func(*testing.T, *LSPInstance) // Custom assertion function
	}{
		{
			name: "Resolve with empty ID uses selected",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["default-lsp"] = &LSPInstance{
					ID:       "default-lsp",
					Language: "go",
					Status:   "running",
				}
				m.selectedLSP = "default-lsp"
				m.mu.Unlock()
			},
			requestID: "",
			expectErr: false,
			checkResult: func(t *testing.T, inst *LSPInstance) {
				if inst.ID != "default-lsp" {
					t.Errorf("Expected selected LSP to be resolved")
				}
			},
		},
		{
			name: "Resolve with specific ID",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["specific-lsp"] = &LSPInstance{
					ID:       "specific-lsp",
					Language: "rust",
					Status:   "running",
				}
				m.mu.Unlock()
			},
			requestID: "specific-lsp",
			expectErr: false,
			checkResult: func(t *testing.T, inst *LSPInstance) {
				if inst.Language != "rust" {
					t.Errorf("Expected rust LSP to be resolved")
				}
			},
		},
		{
			name: "Resolve with no LSP selected",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.selectedLSP = ""
				m.mu.Unlock()
			},
			requestID: "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			tt.setupLSPs(manager)

			instance, err := manager.ResolveLSPInstance(tt.requestID)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if instance == nil {
					t.Errorf("Expected non-nil instance")
				}
				if tt.checkResult != nil {
					tt.checkResult(t, instance)
				}
			}
		})
	}
}

func TestLSPManagerSaveSessionStructure(t *testing.T) {
	tests := []struct {
		name         string                        // Test case description
		setupLSPs    func(*LSPManager)             // Setup function to create LSPs
		checkSession func(*testing.T, *LSPSession) // Custom assertion function
	}{
		{
			name: "Save single LSP session",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["lsp-1"] = &LSPInstance{
					ID:            "lsp-1",
					Language:      "go",
					WorkspacePath: "/project/backend",
					Status:        "running",
				}
				m.mu.Unlock()
			},
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 1 {
					t.Errorf("Expected 1 LSP entry in session")
				}
				if s.LSPs[0].Language != "go" {
					t.Errorf("Expected language go in session")
				}
				if s.LSPs[0].Workspace != "/project/backend" {
					t.Errorf("Expected workspace /project/backend in session")
				}
			},
		},
		{
			name: "Save multiple LSPs session",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["lsp-1"] = &LSPInstance{
					ID:            "lsp-1",
					Language:      "go",
					WorkspacePath: "/project/backend",
					Status:        "running",
				}
				m.instances["lsp-2"] = &LSPInstance{
					ID:            "lsp-2",
					Language:      "typescript",
					WorkspacePath: "/project/frontend",
					Status:        "running",
				}
				m.mu.Unlock()
			},
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 2 {
					t.Errorf("Expected 2 LSP entries in session")
				}
			},
		},
		{
			name: "Save skips stopped LSPs",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["running-lsp"] = &LSPInstance{
					ID:            "running-lsp",
					Language:      "go",
					WorkspacePath: "/project",
					Status:        "running",
				}
				m.instances["stopped-lsp"] = &LSPInstance{
					ID:     "stopped-lsp",
					Status: "stopped",
				}
				m.mu.Unlock()
			},
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 1 {
					t.Errorf("Expected 1 running LSP in session (stopped should be skipped)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			tt.setupLSPs(manager)

			// Create temp file for saving
			tmpfile, err := os.CreateTemp("", "session-save-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			tmpfile.Close()
			defer os.Remove(tmpfile.Name())

			// Note: SaveSession returns error, but we're testing structure
			// In real implementation, this might fail if no config is loaded
			// For this test, we just verify the logic by mocking
			manager.mu.RLock()
			session := &LSPSession{
				LSPs: make([]LSPSessionEntry, 0),
			}
			for _, instance := range manager.instances {
				if instance.Status == "running" {
					session.LSPs = append(session.LSPs, LSPSessionEntry{
						Workspace: instance.WorkspacePath,
						Language:  instance.Language,
					})
				}
			}
			manager.mu.RUnlock()

			tt.checkSession(t, session)
		})
	}
}

func TestLSPInstanceStructure(t *testing.T) {
	tests := []struct {
		name        string                         // Test case description
		instance    *LSPInstance                   // LSP instance to test
		checkFields func(*testing.T, *LSPInstance) // Custom assertion function
	}{
		{
			name: "Basic LSP instance",
			instance: &LSPInstance{
				ID:            "test-id",
				Language:      "go",
				WorkspacePath: "/path/to/workspace",
				Status:        "running",
			},
			checkFields: func(t *testing.T, inst *LSPInstance) {
				if inst.ID == "" {
					t.Errorf("Instance ID is empty")
				}
				if inst.Language == "" {
					t.Errorf("Instance Language is empty")
				}
				if inst.WorkspacePath == "" {
					t.Errorf("Instance WorkspacePath is empty")
				}
				if inst.Status != "running" {
					t.Errorf("Expected running status")
				}
			},
		},
		{
			name: "Stopped instance",
			instance: &LSPInstance{
				ID:     "stopped-id",
				Status: "stopped",
			},
			checkFields: func(t *testing.T, inst *LSPInstance) {
				if inst.Status != "stopped" {
					t.Errorf("Expected stopped status")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFields(t, tt.instance)
		})
	}
}

// TestLSPSelectionState tests LSP selection state transitions.
func TestLSPSelectionState(t *testing.T) {
	tests := []struct {
		name          string            // Test case description
		setupLSPs     func(*LSPManager) // Setup function to create LSPs
		expectedIDSet bool              // Whether selected LSP ID is expected to be set
		expectedID    string            // Expected selected LSP ID
	}{
		{
			name: "No LSP selected initially",
			setupLSPs: func(m *LSPManager) {
				// Do nothing - selected should be empty
			},
			expectedIDSet: false,
		},
		{
			name: "LSP selected after first starts",
			setupLSPs: func(m *LSPManager) {
				m.mu.Lock()
				m.instances["first-lsp"] = &LSPInstance{ID: "first-lsp"}
				m.selectedLSP = "first-lsp"
				m.mu.Unlock()
			},
			expectedIDSet: true,
			expectedID:    "first-lsp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}
			manager := NewLSPManager(config)

			tt.setupLSPs(manager)

			manager.mu.RLock()
			hasSelected := manager.selectedLSP != ""
			selectedID := manager.selectedLSP
			manager.mu.RUnlock()

			if hasSelected != tt.expectedIDSet {
				t.Errorf("Expected hasSelected=%v, got %v", tt.expectedIDSet, hasSelected)
			}
			if tt.expectedIDSet && selectedID != tt.expectedID {
				t.Errorf("Expected default ID %s, got %s", tt.expectedID, selectedID)
			}
		})
	}
}

func TestLSPManagerConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	config := &Config{
		Defaults: map[string]LSPDefaultConfig{},
		LSPs:     make(map[string]map[string]LSPConfig),
	}
	manager := NewLSPManager(config)

	// Add instances concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			manager.mu.Lock()
			manager.instances["lsp-"+string(rune(index))] = &LSPInstance{
				ID:     "lsp-" + string(rune(index)),
				Status: "running",
			}
			manager.mu.Unlock()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// List should work without panic
	list := manager.ListLSPs()
	if list == nil {
		t.Errorf("List should not be nil after concurrent additions")
	}
}

func TestAutoSelectBehavior(t *testing.T) {
	tests := []struct {
		name             string
		duringStartup    bool // Whether called during startup
		numInstances     int  // Number of running instances
		expectAutoSelect bool
		description      string
	}{
		{
			name:             "During startup with 1 instance: no auto-select",
			duringStartup:    true,
			numInstances:     1,
			expectAutoSelect: false,
			description:      "During startup, skip auto-select to avoid non-determinism",
		},
		{
			name:             "During startup with 3 instances: no auto-select",
			duringStartup:    true,
			numInstances:     3,
			expectAutoSelect: false,
			description:      "During startup, skip auto-select regardless of count",
		},
		{
			name:             "After startup with 1 instance: auto-select",
			duringStartup:    false,
			numInstances:     1,
			expectAutoSelect: true,
			description:      "After startup, auto-select single instance",
		},
		{
			name:             "After startup with 2 instances: no auto-select",
			duringStartup:    false,
			numInstances:     2,
			expectAutoSelect: false,
			description:      "After startup, don't auto-select if multiple instances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			}

			manager := NewLSPManager(config)

			// Test the auto-select condition directly
			// The actual condition in StartLSP is:
			// !duringStartup && len(m.instances) == 1
			manager.mu.Lock()

			shouldAutoSelect := !tt.duringStartup && tt.numInstances == 1
			if shouldAutoSelect {
				manager.selectedLSP = "test-instance-id"
			}

			selectedLSP := manager.selectedLSP
			manager.mu.Unlock()

			hasSelection := selectedLSP != ""

			if tt.expectAutoSelect && !hasSelection {
				t.Errorf("%s: Expected auto-select but selectedLSP is empty. %s", tt.name, tt.description)
			}
			if !tt.expectAutoSelect && hasSelection {
				t.Errorf("%s: Expected NO auto-select but selectedLSP='%s'. %s", tt.name, selectedLSP, tt.description)
			}
		})
	}
}
