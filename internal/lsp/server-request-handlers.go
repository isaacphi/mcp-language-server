package lsp

import (
	"encoding/json"
	"log"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// Requests

func HandleWorkspaceConfiguration(params json.RawMessage) (interface{}, error) {
	return []map[string]interface{}{{}}, nil
}

func HandleRegisterCapability(params json.RawMessage) (interface{}, error) {
	return nil, nil
}

// Notifications

func HandleServerMessage(params json.RawMessage) {
	var msg struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &msg); err == nil {
		log.Printf("Server message: %s\n", msg.Message)
	}
}

func HandleDiagnostics(client *Client, params json.RawMessage) {
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		log.Printf("Error unmarshaling diagnostic params: %v", err)
		return
	}

	client.diagnosticsMu.Lock()
	defer client.diagnosticsMu.Unlock()

	client.diagnostics[diagParams.URI] = diagParams.Diagnostics

	log.Printf("Received diagnostics for %s: %d items", diagParams.URI, len(diagParams.Diagnostics))
}

