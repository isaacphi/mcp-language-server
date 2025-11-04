package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// LSPDefaultConfig defines the default configuration for a language server in the config file
type LSPDefaultConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// LSPConfig defines the configuration needed to start a language server instance
// It extends LSPDefaultConfig with the workspace directory for the instance
type LSPConfig struct {
	LSPDefaultConfig
	Workspace string // Workspace directory for this instance
}

// Config represents the loaded configuration
type Config struct {
	// Defaults are the LSP definitions from the config file
	Defaults map[string]LSPDefaultConfig
	// LSPs maps language -> workspace -> config
	// This structure supports multiple instances per language
	LSPs map[string]map[string]LSPConfig
}

// ConfigFile represents the structure of the config file on disk
type ConfigFile struct {
	LSPs map[string]LSPDefaultConfig `json:"lsps"`
}

// LoadConfigFile loads and parses the configuration file
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(configFile.LSPs) == 0 {
		return nil, fmt.Errorf("config file must contain at least one LSP definition")
	}

	return &Config{
		Defaults: configFile.LSPs,
		LSPs:     make(map[string]map[string]LSPConfig),
	}, nil
}
