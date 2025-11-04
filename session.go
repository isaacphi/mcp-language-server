package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// LSPSessionEntry represents a single LSP instance in a session
type LSPSessionEntry struct {
	Workspace string `json:"workspace"`
	Language  string `json:"language"`
}

// LSPSession represents a saved session configuration
type LSPSession struct {
	LSPs []LSPSessionEntry `json:"lsps"`
}

// LoadSessionFile loads a session configuration from a file
func LoadSessionFile(path string) (*LSPSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session LSPSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// SaveSessionFile saves a session configuration to a file
func SaveSessionFile(path string, session *LSPSession) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}
