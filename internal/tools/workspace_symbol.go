package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// WorkspaceSymbol calls the LSP workspace/symbol request with the given query
// and returns a human-readable list of symbols (name, kind, location).
func WorkspaceSymbol(ctx context.Context, client *lsp.Client, query string) (string, error) {
	result, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: query})
	if err != nil {
		return "", fmt.Errorf("workspace/symbol request failed: %w", err)
	}

	results, err := result.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse workspace symbol results: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No symbols found for query %q.", query), nil
	}

	var b strings.Builder
	for i, sym := range results {
		loc := sym.GetLocation()
		kindStr := symbolKindString(sym)
		filePath := loc.URI.Path()
		locStr := filePath
		if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 0 || loc.Range.End.Line != 0 || loc.Range.End.Character != 0 {
			locStr = fmt.Sprintf("%s L%d:C%d", filePath, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
		}
		if kindStr != "" {
			fmt.Fprintf(&b, "%d. %s (%s)\n   %s\n", i+1, sym.GetName(), kindStr, locStr)
		} else {
			fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, sym.GetName(), locStr)
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func symbolKindString(sym protocol.WorkspaceSymbolResult) string {
	switch v := sym.(type) {
	case *protocol.SymbolInformation:
		if s, ok := protocol.TableKindMap[v.Kind]; ok {
			return s
		}
		return ""
	case *protocol.WorkspaceSymbol:
		if s, ok := protocol.TableKindMap[v.Kind]; ok {
			return s
		}
		return ""
	default:
		return ""
	}
}
