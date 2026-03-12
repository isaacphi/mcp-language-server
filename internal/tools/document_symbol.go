package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// DocumentSymbolOutline returns a human-readable outline of document symbols for the given file.
// The file is opened via the LSP client if needed, then textDocument/documentSymbol is called.
// Result is formatted as a hierarchical outline (name, kind, range, optional detail, children).
func DocumentSymbolOutline(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %w", err)
	}

	if err := client.OpenFile(ctx, absPath); err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	uri := protocol.URIFromPath(absPath)
	params := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}

	result, err := client.DocumentSymbol(ctx, params)
	if err != nil {
		return "", fmt.Errorf("document symbol request failed: %w", err)
	}

	if result.Value == nil {
		return fmt.Sprintf("Document: %s\nNo symbols reported.\n", absPath), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Document: %s\n\n", absPath))

	switch v := result.Value.(type) {
	case []protocol.DocumentSymbol:
		if len(v) == 0 {
			b.WriteString("No symbols reported.\n")
			return b.String(), nil
		}
		for i := range v {
			formatDocumentSymbol(&b, &v[i], 0)
		}
	case []protocol.SymbolInformation:
		if len(v) == 0 {
			b.WriteString("No symbols reported.\n")
			return b.String(), nil
		}
		for i := range v {
			formatSymbolInformation(&b, &v[i])
		}
	default:
		return "", fmt.Errorf("unexpected document symbol result type: %T", result.Value)
	}

	return strings.TrimSuffix(b.String(), "\n"), nil
}

// formatDocumentSymbol appends a hierarchical outline for one DocumentSymbol (and its children).
func formatDocumentSymbol(b *strings.Builder, s *protocol.DocumentSymbol, indent int) {
	prefix := strings.Repeat("  ", indent)
	kindStr := protocol.TableKindMap[s.Kind]
	if kindStr == "" {
		kindStr = fmt.Sprintf("Kind(%d)", s.Kind)
	}
	// LSP range is 0-based; display 1-based for humans
	lineStart := s.SelectionRange.Start.Line + 1
	colStart := s.SelectionRange.Start.Character + 1
	lineEnd := s.SelectionRange.End.Line + 1
	colEnd := s.SelectionRange.End.Character + 1
	b.WriteString(fmt.Sprintf("%s%s (%s) L%d:C%d - L%d:C%d\n", prefix, s.Name, kindStr, lineStart, colStart, lineEnd, colEnd))
	if s.Detail != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", prefix, s.Detail))
	}
	for i := range s.Children {
		formatDocumentSymbol(b, &s.Children[i], indent+1)
	}
}

// formatSymbolInformation appends a flat line for one SymbolInformation.
func formatSymbolInformation(b *strings.Builder, s *protocol.SymbolInformation) {
	kindStr := protocol.TableKindMap[s.Kind]
	if kindStr == "" {
		kindStr = fmt.Sprintf("Kind(%d)", s.Kind)
	}
	r := s.Location.Range
	lineStart := r.Start.Line + 1
	colStart := r.Start.Character + 1
	lineEnd := r.End.Line + 1
	colEnd := r.End.Character + 1
	b.WriteString(fmt.Sprintf("%s (%s) L%d:C%d - L%d:C%d", s.Name, kindStr, lineStart, colStart, lineEnd, colEnd))
	if s.ContainerName != "" {
		b.WriteString(fmt.Sprintf(" [container: %s]", s.ContainerName))
	}
	b.WriteString("\n")
}
