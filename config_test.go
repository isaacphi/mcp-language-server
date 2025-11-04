package main

import (
	"encoding/json"
	"os"
	"testing"
)

// osReadFile is a reference to os.ReadFile, used for testing mocking
var (
	osReadFile = os.ReadFile
)

func setupMockFS(t *testing.T, files map[string][]byte) func() {
	originalReadFile := osReadFile

	return func() {
		osReadFile = originalReadFile
	}
}

func TestLoadConfigFile(t *testing.T) {
	tests := []struct {
		name        string                    // Test case description
		content     string                    // JSON config file content
		expectErr   bool                      // Whether an error is expected
		expectLsps  int                       // Expected number of LSP definitions
		checkConfig func(*testing.T, *Config) // Custom assertion function
	}{
		{
			name: "Valid single LSP config",
			content: `{
				"lsps": {
					"go": {
						"command": "gopls",
						"args": [],
						"env": {"GOPATH": "/home/user/go"}
					}
				}
			}`,
			expectErr:  false,
			expectLsps: 1,
			checkConfig: func(t *testing.T, cfg *Config) {
				if _, ok := cfg.Defaults["go"]; !ok {
					t.Errorf("Expected 'go' LSP in config")
				}
				if cfg.Defaults["go"].Command != "gopls" {
					t.Errorf("Expected gopls command, got %s", cfg.Defaults["go"].Command)
				}
				if cfg.Defaults["go"].Env["GOPATH"] != "/home/user/go" {
					t.Errorf("Expected GOPATH env var")
				}
			},
		},
		{
			name: "Multiple LSPs",
			content: `{
				"lsps": {
					"go": {"command": "gopls", "args": []},
					"rust": {"command": "rust-analyzer", "args": []},
					"python": {"command": "pyright-langserver", "args": ["--", "--stdio"]}
				}
			}`,
			expectErr:  false,
			expectLsps: 3,
			checkConfig: func(t *testing.T, cfg *Config) {
				expected := []string{"go", "rust", "python"}
				for _, lang := range expected {
					if _, ok := cfg.Defaults[lang]; !ok {
						t.Errorf("Expected '%s' LSP in config", lang)
					}
				}
			},
		},
		{
			name: "LSP with args",
			content: `{
				"lsps": {
					"typescript": {
						"command": "typescript-language-server",
						"args": ["--stdio"]
					}
				}
			}`,
			expectErr:  false,
			expectLsps: 1,
			checkConfig: func(t *testing.T, cfg *Config) {
				ts := cfg.Defaults["typescript"]
				if len(ts.Args) != 1 || ts.Args[0] != "--stdio" {
					t.Errorf("Expected args [--stdio], got %v", ts.Args)
				}
			},
		},
		{
			name:        "Empty LSPs",
			content:     `{"lsps": {}}`,
			expectErr:   true,
			expectLsps:  0,
			checkConfig: nil,
		},
		{
			name:        "Invalid JSON",
			content:     `{invalid json}`,
			expectErr:   true,
			expectLsps:  0,
			checkConfig: nil,
		},
		{
			name:        "Missing lsps field",
			content:     `{"other": {}}`,
			expectErr:   true,
			expectLsps:  0,
			checkConfig: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "config-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpfile.Close()

			cfg, err := LoadConfigFile(tmpfile.Name())
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(cfg.Defaults) != tt.expectLsps {
					t.Errorf("Expected %d LSPs, got %d", tt.expectLsps, len(cfg.Defaults))
				}
				if tt.checkConfig != nil {
					tt.checkConfig(t, cfg)
				}
			}
		})
	}
}

func TestLSPConfigStructure(t *testing.T) {
	tests := []struct {
		name        string
		lspDefault  LSPDefaultConfig
		workspace   string
		checkFields func(*testing.T, LSPConfig)
	}{
		{
			name: "Basic LSP config",
			lspDefault: LSPDefaultConfig{
				Command: "gopls",
				Args:    []string{},
				Env:     map[string]string{},
			},
			workspace: "/path/to/workspace",
			checkFields: func(t *testing.T, cfg LSPConfig) {
				if cfg.Command != "gopls" {
					t.Errorf("Expected command gopls")
				}
				if cfg.Workspace != "/path/to/workspace" {
					t.Errorf("Expected workspace /path/to/workspace")
				}
			},
		},
		{
			name: "LSP with environment variables",
			lspDefault: LSPDefaultConfig{
				Command: "gopls",
				Env: map[string]string{
					"GOPATH": "/home/user/go",
					"GOROOT": "/usr/local/go",
				},
			},
			workspace: "/project",
			checkFields: func(t *testing.T, cfg LSPConfig) {
				if cfg.Env["GOPATH"] != "/home/user/go" {
					t.Errorf("GOPATH env var not set correctly")
				}
				if cfg.Env["GOROOT"] != "/usr/local/go" {
					t.Errorf("GOROOT env var not set correctly")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LSPConfig{
				LSPDefaultConfig: tt.lspDefault,
				Workspace:        tt.workspace,
			}
			tt.checkFields(t, cfg)
		})
	}
}

func TestConfigFileMarshaling(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		checkContents bool
	}{
		{
			name: "Marshal and unmarshal config preserves commands",
			config: &Config{
				Defaults: map[string]LSPDefaultConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
						Env:     map[string]string{"GOPATH": "/go"},
					},
					"rust": {
						Command: "rust-analyzer",
						Args:    []string{},
						Env:     map[string]string{},
					},
				},
				LSPs: make(map[string]map[string]LSPConfig),
			},
			checkContents: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(map[string]map[string]LSPDefaultConfig{
				"lsps": tt.config.Defaults,
			})
			if err != nil {
				t.Errorf("Failed to marshal config: %v", err)
				return
			}

			// Unmarshal back
			var configFile struct {
				LSPs map[string]LSPDefaultConfig `json:"lsps"`
			}
			err = json.Unmarshal(data, &configFile)
			if err != nil {
				t.Errorf("Failed to unmarshal config: %v", err)
				return
			}

			if tt.checkContents {
				// Check that both LSPs are present
				if len(configFile.LSPs) != 2 {
					t.Errorf("Expected 2 LSPs, got %d", len(configFile.LSPs))
				}
				if configFile.LSPs["go"].Command != "gopls" {
					t.Errorf("Expected gopls command for go")
				}
				if configFile.LSPs["rust"].Command != "rust-analyzer" {
					t.Errorf("Expected rust-analyzer command for rust")
				}
				if configFile.LSPs["go"].Env["GOPATH"] != "/go" {
					t.Errorf("Expected GOPATH env var")
				}
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "Valid config with LSPs",
			config: &Config{
				Defaults: map[string]LSPDefaultConfig{
					"go": {Command: "gopls"},
				},
				LSPs: make(map[string]map[string]LSPConfig),
			},
			expectErr: false,
		},
		{
			name: "Empty defaults (but valid structure)",
			config: &Config{
				Defaults: map[string]LSPDefaultConfig{},
				LSPs:     make(map[string]map[string]LSPConfig),
			},
			expectErr: false, // Empty defaults is valid, but LoadConfigFile rejects it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic structure validation
			if tt.config == nil {
				t.Errorf("Config is nil")
			}
			if tt.config.Defaults == nil {
				t.Errorf("Defaults map is nil")
			}
			if tt.config.LSPs == nil {
				t.Errorf("LSPs map is nil")
			}
		})
	}
}
