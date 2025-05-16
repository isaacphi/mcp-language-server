package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/logging"
)

// Create component-specific loggers
var lspLogger = logging.NewLogger(logging.LSP)
var wireLogger = logging.NewLogger(logging.LSPWire)
var processLogger = logging.NewLogger(logging.LSPProcess)

// WriteMessage writes an LSP message to the given writer
func WriteMessage(w io.Writer, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// High-level operation log
	lspLogger.Debug("Sending message: method=%s id=%v", msg.Method, msg.ID)

	// Wire protocol log (more detailed)
	wireLogger.Debug("-> Sending: %s", string(data))

	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data))
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// ReadMessage reads a single LSP message from the given reader
func ReadMessage(r *bufio.Reader) (*Message, error) {
	// Read headers
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		line = strings.TrimSpace(line)

		if line == "" {
			break // End of headers
		}

		wireLogger.Debug("<- Header: %s", line)

		if strings.HasPrefix(line, "Content-Length: ") {
			_, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}

	// Read content
	content := make([]byte, contentLength)
	_, err := io.ReadFull(r, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	wireLogger.Debug("<- Received: %s", string(content))

	// Parse message
	var msg Message
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Log higher-level information about the message type
	if msg.Method != "" && msg.ID != nil && msg.ID.Value != nil {
		lspLogger.Debug("Received request from server: method=%s id=%v", msg.Method, msg.ID)
	} else if msg.Method != "" {
		lspLogger.Debug("Received notification: method=%s", msg.Method)
	} else if msg.ID != nil && msg.ID.Value != nil {
		lspLogger.Debug("Received response for ID: %v", msg.ID)
	}

	return &msg, nil
}

// handleMessages reads and dispatches messages in a loop
func (c *Client) handleMessages() {
	for {
		msg, err := ReadMessage(c.stdout)
		if err != nil {
			// Check if this is due to normal shutdown (EOF when closing connection)
			if strings.Contains(err.Error(), "EOF") {
				lspLogger.Info("LSP connection closed (EOF)")
			} else {
				lspLogger.Error("Error reading message: %v", err)
			}
			return
		}

		// Handle server->client request (has both Method and ID)
		if msg.Method != "" && msg.ID != nil && msg.ID.Value != nil {
			response := &Message{
				JSONRPC: "2.0",
				ID:      msg.ID,
			}

			// Look up handler for this method
			c.serverHandlersMu.RLock()
			handler, ok := c.serverRequestHandlers[msg.Method]
			c.serverHandlersMu.RUnlock()

			if ok {
				lspLogger.Debug("Processing server request: method=%s id=%v", msg.Method, msg.ID)
				result, err := handler(msg.Params)
				if err != nil {
					lspLogger.Error("Error handling server request %s: %v", msg.Method, err)
					response.Error = &ResponseError{
						Code:    -32603,
						Message: err.Error(),
					}
				} else {
					rawJSON, err := json.Marshal(result)
					if err != nil {
						lspLogger.Error("Failed to marshal response for %s: %v", msg.Method, err)
						response.Error = &ResponseError{
							Code:    -32603,
							Message: fmt.Sprintf("failed to marshal response: %v", err),
						}
					} else {
						response.Result = rawJSON
					}
				}
			} else {
				lspLogger.Warn("Method not found: %s", msg.Method)
				response.Error = &ResponseError{
					Code:    -32601,
					Message: fmt.Sprintf("method not found: %s", msg.Method),
				}
			}

			// Send response back to server
			if err := WriteMessage(c.stdin, response); err != nil {
				lspLogger.Error("Error sending response to server: %v", err)
			}

			continue
		}

		// Handle notification (has Method but no ID)
		if msg.Method != "" && (msg.ID == nil || msg.ID.Value == nil) {
			c.notificationMu.RLock()
			handler, ok := c.notificationHandlers[msg.Method]
			c.notificationMu.RUnlock()

			if ok {
				lspLogger.Debug("Handling notification: %s", msg.Method)
				go handler(msg.Params)
			} else {
				lspLogger.Debug("No handler for notification: %s", msg.Method)
			}
			continue
		}

		// Handle response to our request (has ID but no Method)
		if msg.ID != nil && msg.ID.Value != nil && msg.Method == "" {
			// Convert ID to string for map lookup
			idStr := msg.ID.String()
			c.handlersMu.RLock()
			ch, ok := c.handlers[idStr]
			c.handlersMu.RUnlock()

			if ok {
				lspLogger.Debug("Sending response for ID %v to handler", msg.ID)
				ch <- msg
				close(ch)
			} else {
				lspLogger.Debug("No handler for response ID: %v", msg.ID)
			}
		}
	}
}

// Call makes a request and waits for the response
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	id := c.nextID.Add(1)

	lspLogger.Debug("Making call: method=%s id=%v", method, id)

	msg, err := NewRequest(id, method, params)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Create response channel
	ch := make(chan *Message, 1)
	// Convert ID to string for map lookup
	idStr := msg.ID.String()
	c.handlersMu.Lock()
	c.handlers[idStr] = ch
	c.handlersMu.Unlock()

	defer func() {
		c.handlersMu.Lock()
		delete(c.handlers, idStr)
		c.handlersMu.Unlock()
	}()

	// Send request
	if err := WriteMessage(c.stdin, msg); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	lspLogger.Debug("Waiting for response to request ID: %v", msg.ID)

	// Wait for response
	resp := <-ch

	lspLogger.Debug("Received response for request ID: %v", msg.ID)

	if resp.Error != nil {
		lspLogger.Error("Request failed: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		return fmt.Errorf("request failed: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	if result != nil {
		// If result is a json.RawMessage, just copy the raw bytes
		if rawMsg, ok := result.(*json.RawMessage); ok {
			*rawMsg = resp.Result
			return nil
		}
		// Otherwise unmarshal into the provided type
		if err := json.Unmarshal(resp.Result, result); err != nil {
			lspLogger.Error("Failed to unmarshal result: %v", err)
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// Notify sends a notification (a request without an ID that doesn't expect a response)
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	lspLogger.Debug("Sending notification: method=%s", method)

	msg, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	if err := WriteMessage(c.stdin, msg); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

type NotificationHandler func(params json.RawMessage)
type ServerRequestHandler func(params json.RawMessage) (any, error)
