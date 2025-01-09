package lsp

import (
	"encoding/json"
	"log"
)

type WorkspaceConfigurationHandler struct{}

func (h *WorkspaceConfigurationHandler) Handle(params json.RawMessage) (interface{}, error) {
	return []map[string]interface{}{{}}, nil
}

type RegisterCapabilityHandler struct{}

func (h *RegisterCapabilityHandler) Handle(params json.RawMessage) (interface{}, error) {
	return nil, nil
}

func ServerMessageHandler(method string, params json.RawMessage) {
	var msg struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &msg); err == nil {
		log.Printf("Server message: %s\n", msg.Message)
	}
}
