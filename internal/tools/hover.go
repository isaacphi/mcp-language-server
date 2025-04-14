package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetHoverInfo retrieves hover information (type, documentation) for a symbol at the specified position
func GetHoverInfo(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Convert 1-indexed line/column to 0-indexed for LSP protocol
	uri := protocol.DocumentUri("file://" + filePath)
	position := protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(column - 1),
	}

	// Create the hover parameters
	params := protocol.HoverParams{}

	// Set TextDocument and Position via embedded struct
	params.TextDocument = protocol.TextDocumentIdentifier{
		URI: uri,
	}
	params.Position = position

	// Execute the hover request
	hoverResult, err := client.Hover(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get hover information: %v", err)
	}

	var result strings.Builder
	result.WriteString("Hover Information\n")

	// Process the hover contents based on Markup content
	if hoverResult.Contents.Value == "" {
		result.WriteString("No hover information available for this position")
	} else {
		if hoverResult.Contents.Kind != "" {
			result.WriteString(fmt.Sprintf("Kind: %s\n\n", hoverResult.Contents.Kind))
		}
		result.WriteString(hoverResult.Contents.Value)
	}

	return result.String(), nil
}
