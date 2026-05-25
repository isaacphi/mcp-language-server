package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// FindImplementations returns every type or method that implements
// the interface (or interface method) at the given position. Wraps
// `textDocument/implementation`.
//
// Use case: Go and TypeScript both use structural typing — there is
// no `implements` keyword for grep to match — so this is the only
// reliable way to enumerate the satisfaction set of an interface.
// `references` returns USES of the interface (callers); this
// returns IMPLEMENTATIONS (the types that satisfy it).
//
// Output format: one match per line as `<file>:<line>:<column>`,
// matching what `gopls implementation` emits on the CLI.
func FindImplementations(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.DocumentUri("file://" + filePath)
	params := protocol.ImplementationParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	result, err := client.Implementation(ctx, params)
	if err != nil {
		return "", fmt.Errorf("textDocument/implementation failed: %v", err)
	}

	locations := flattenImplementationResult(result)
	if len(locations) == 0 {
		return "No implementations found.", nil
	}

	var out strings.Builder
	for _, loc := range locations {
		file := strings.TrimPrefix(string(loc.URI), "file://")
		fmt.Fprintf(&out, "%s:%d:%d\n",
			file,
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
		)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// flattenImplementationResult normalizes the `Or_Result_textDocument
// _implementation` union (Location | []Location | []DefinitionLink |
// []LocationLink) into a single []Location slice.
//
// LSP servers vary in which shape they return:
//   - gopls returns []Location
//   - tsserver returns Location[]
//   - rust-analyzer / clangd may return LocationLink[] when
//     linkSupport=true is advertised by the client
//
// The flattener keeps the tool agnostic to which server is on the
// other end.
func flattenImplementationResult(r protocol.Or_Result_textDocument_implementation) []protocol.Location {
	if r.Value == nil {
		return nil
	}
	switch v := r.Value.(type) {
	case protocol.Definition:
		return flattenDefinition(v)
	case protocol.Location:
		return []protocol.Location{v}
	case []protocol.Location:
		return v
	case []protocol.DefinitionLink:
		out := make([]protocol.Location, 0, len(v))
		for _, link := range v {
			out = append(out, protocol.Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			})
		}
		return out
	default:
		return nil
	}
}

// flattenDefinition reduces a Definition (Or_Definition union of
// Location | []Location) to a flat []Location. Same idea as
// flattenImplementationResult but for the inner Definition layer.
func flattenDefinition(d protocol.Definition) []protocol.Location {
	if d.Value == nil {
		return nil
	}
	switch v := d.Value.(type) {
	case protocol.Location:
		return []protocol.Location{v}
	case []protocol.Location:
		return v
	default:
		return nil
	}
}
