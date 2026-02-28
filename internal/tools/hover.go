package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetHoverInfo retrieves hover information (type, documentation) for a symbol at the specified position
func GetHoverInfo(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	// Use the same canonical path as OpenFile so the hover request URI matches
	// the document opened in didOpen (clangd returns "non-added document" otherwise).
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %v", err)
	}

	if err := client.OpenFile(ctx, absPath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(absPath)

	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	// Execute the hover request
	hoverResult, err := client.Hover(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get hover information: %v", err)
	}

	var result strings.Builder

	// Process the hover contents based on Markup content
	if hoverResult.Contents.Value == "" {
		// Extract the line where the hover was requested
		lineText, err := ExtractTextFromLocation(protocol.Location{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      params.Position.Line,
					Character: 0,
				},
				End: protocol.Position{
					Line:      params.Position.Line + 1,
					Character: 0,
				},
			},
		})
		if err != nil {
			toolsLogger.Warn("failed to extract line at position: %v", err)
		}
		result.WriteString(fmt.Sprintf("No hover information available for this position on the following line:\n%s", lineText))
	} else {
		result.WriteString(hoverResult.Contents.Value)
	}

	return result.String(), nil
}
