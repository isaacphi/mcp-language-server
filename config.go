package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// LSPConfig defines the configuration for a single language server
type LSPConfig struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Workspace string            `json:"workspace,omitempty"` // Used for legacy mode or explicit workspace
}

// Config represents the configuration file structure
type Config struct {
	LSPs map[string]LSPConfig `json:"lsps"`
}

// LoadConfigFile loads and parses the configuration file
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(config.LSPs) == 0 {
		return nil, fmt.Errorf("config file must contain at least one LSP definition")
	}

	return &config, nil
}
