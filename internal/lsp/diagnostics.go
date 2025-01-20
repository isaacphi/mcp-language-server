package lsp

import (
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetFileDiagnostics returns the cached diagnostics for a specific file
func (c *Client) GetFileDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}

// GetAllDiagnostics returns all cached diagnostics
func (c *Client) GetAllDiagnostics() map[protocol.DocumentUri][]protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	// Return a copy of the diagnostics map
	result := make(map[protocol.DocumentUri][]protocol.Diagnostic)
	for uri, diagnostics := range c.diagnostics {
		diagCopy := make([]protocol.Diagnostic, len(diagnostics))
		copy(diagCopy, diagnostics)
		result[uri] = diagCopy
	}

	return result
}

