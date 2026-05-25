package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// WorkspaceSymbol searches the entire workspace for symbols matching
// the given query. The query is forwarded directly to the language
// server's workspace/symbol method, so matching semantics depend on
// the server: gopls + tsserver + clangd all support fuzzy matching
// (e.g. `Wsymb` finds `WorkspaceSymbol`); pyright and rust-analyzer
// match by substring.
//
// Returns one match per line in the format
// `<file>:<line>:<column> <name> <Kind>[ (in <ContainerName>)]`
// where Kind is the LSP SymbolKind enum's canonical name (Function,
// Class, Interface, etc.).
//
// Use this BEFORE the file-scoped tools (definition, references,
// hover) when you don't know which file a symbol lives in — one
// workspace/symbol call replaces N grep-the-codebase calls.
func WorkspaceSymbol(ctx context.Context, client *lsp.Client, query string) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query must be non-empty")
	}

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: query})
	if err != nil {
		return "", fmt.Errorf("workspace/symbol failed: %v", err)
	}
	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}
	if len(results) == 0 {
		return fmt.Sprintf("No symbols found matching %q", query), nil
	}

	var out strings.Builder
	for _, s := range results {
		loc := s.GetLocation()
		filePath := strings.TrimPrefix(string(loc.URI), "file://")
		fmt.Fprintf(&out, "%s:%d:%d %s %s",
			filePath,
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
			s.GetName(),
			symbolKindName(s.GetKind()),
		)
		if container := s.GetContainerName(); container != "" {
			fmt.Fprintf(&out, " (in %s)", container)
		}
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// symbolKindName maps the LSP SymbolKind enum to its canonical
// human-readable name. Pinned by the LSP spec (3.17). Unknown
// values pass through as numbers — better than panicking when a
// future spec adds a new kind.
func symbolKindName(k protocol.SymbolKind) string {
	switch k {
	case protocol.File:
		return "File"
	case protocol.Module:
		return "Module"
	case protocol.Namespace:
		return "Namespace"
	case protocol.Package:
		return "Package"
	case protocol.Class:
		return "Class"
	case protocol.Method:
		return "Method"
	case protocol.Property:
		return "Property"
	case protocol.Field:
		return "Field"
	case protocol.Constructor:
		return "Constructor"
	case protocol.Enum:
		return "Enum"
	case protocol.Interface:
		return "Interface"
	case protocol.Function:
		return "Function"
	case protocol.Variable:
		return "Variable"
	case protocol.Constant:
		return "Constant"
	case protocol.String:
		return "String"
	case protocol.Number:
		return "Number"
	case protocol.Boolean:
		return "Boolean"
	case protocol.Array:
		return "Array"
	case protocol.Object:
		return "Object"
	case protocol.Key:
		return "Key"
	case protocol.Null:
		return "Null"
	case protocol.EnumMember:
		return "EnumMember"
	case protocol.Struct:
		return "Struct"
	case protocol.Event:
		return "Event"
	case protocol.Operator:
		return "Operator"
	case protocol.TypeParameter:
		return "TypeParameter"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}
