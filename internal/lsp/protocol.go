package lsp

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// MessageID represents a JSON-RPC ID which can be a string, number, or null
// per the JSON-RPC 2.0 specification
type MessageID struct {
	Value any
}

// MarshalJSON implements custom JSON marshaling for MessageID
func (id *MessageID) MarshalJSON() ([]byte, error) {
	if id == nil || id.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(id.Value)
}

// UnmarshalJSON implements custom JSON unmarshaling for MessageID
func (id *MessageID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		id.Value = nil
		return nil
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	// Convert float64 (default JSON number type) to int32 for backward compatibility
	if num, ok := value.(float64); ok {
		id.Value = int32(num)
	} else {
		id.Value = value
	}

	return nil
}

// String returns a string representation of the ID
func (id *MessageID) String() string {
	if id == nil || id.Value == nil {
		return "<null>"
	}

	switch v := id.Value.(type) {
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Equals checks if two MessageIDs are equal
func (id *MessageID) Equals(other *MessageID) bool {
	if id == nil || other == nil {
		return id == other
	}
	if id.Value == nil || other.Value == nil {
		return id.Value == other.Value
	}

	return fmt.Sprintf("%v", id.Value) == fmt.Sprintf("%v", other.Value)
}

// Message represents a JSON-RPC 2.0 message
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *MessageID      `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError represents a JSON-RPC 2.0 error
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewRequest(id any, method string, params any) (*Message, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      &MessageID{Value: id},
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

func NewNotification(method string, params any) (*Message, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
		// Notifications don't have an ID by definition
		ID: nil,
	}, nil
}
