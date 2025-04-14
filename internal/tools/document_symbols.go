package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

// GetDocumentSymbols retrieves all symbols in a document and formats them in a hierarchical structure
func GetDocumentSymbols(ctx context.Context, client *lsp.Client, filePath string, showLineNumbers bool) (string, error) {
	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Convert to URI format for LSP protocol
	uri := protocol.DocumentUri("file://" + filePath)

	// Create the document symbol parameters
	symParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	}

	// Execute the document symbol request
	symResult, err := client.DocumentSymbol(ctx, symParams)
	if err != nil {
		return "", fmt.Errorf("failed to get document symbols: %v", err)
	}

	symbols, err := symResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to process document symbols: %v", err)
	}

	if len(symbols) == 0 {
		return fmt.Sprintf("No symbols found in %s", filePath), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Symbols in %s\n\n", filePath))

	// Format symbols hierarchically
	formatSymbols(&result, symbols, 0, showLineNumbers)

	return result.String(), nil
}

// formatSymbols recursively formats symbols with proper indentation
func formatSymbols(sb *strings.Builder, symbols []protocol.DocumentSymbolResult, level int, showLineNumbers bool) {
	indent := strings.Repeat("  ", level)

	for _, sym := range symbols {
		// Get symbol information
		name := sym.GetName()

		// Format location information
		location := ""
		if showLineNumbers {
			r := sym.GetRange()
			if r.Start.Line == r.End.Line {
				location = fmt.Sprintf("Line %d", r.Start.Line+1)
			} else {
				location = fmt.Sprintf("Lines %d-%d", r.Start.Line+1, r.End.Line+1)
			}
		}

		// Use the shared utility to extract kind information
		kindStr := utilities.ExtractSymbolKind(sym)

		// Format the symbol entry
		if location != "" {
			sb.WriteString(fmt.Sprintf("%s%s %s (%s)\n", indent, kindStr, name, location))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s %s\n", indent, kindStr, name))
		}

		// Format children if it's a DocumentSymbol
		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
			for i := range ds.Children {
				childSymbols[i] = &ds.Children[i]
			}
			formatSymbols(sb, childSymbols, level+1, showLineNumbers)
		}
	}
}
