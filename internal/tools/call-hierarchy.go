package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetCallHierarchy lists every incoming AND outgoing call edge for
// the function / method at the given position. Wraps the three-call
// LSP dance: `textDocument/prepareCallHierarchy` (to anchor the
// target) → `callHierarchy/incomingCalls` (callers) +
// `callHierarchy/outgoingCalls` (callees).
//
// Use case: refactor impact analysis (`who calls X before I change
// X's signature`) and data-flow tracing (`what does X end up
// calling`).
//
// Output format mirrors `gopls call_hierarchy`:
//
//	caller[N]: <ranges> in <file>:<line>:<col> from <name> in <file>:<line>:<col>
//	identifier: <Kind> <name> in <file>:<line>:<col>
//	callee[N]: <Kind> <name> in <file>:<line>:<col>
//
// The cursor MUST be on the function / method NAME (not its body) —
// most language servers reject prepareCallHierarchy at other
// positions with a "no symbol at position" error.
func GetCallHierarchy(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.DocumentUri("file://" + filePath)
	prep := protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	items, err := client.PrepareCallHierarchy(ctx, prep)
	if err != nil {
		return "", fmt.Errorf("prepareCallHierarchy failed: %v", err)
	}
	if len(items) == 0 {
		return "No call hierarchy at this position — cursor must be on a function or method name.", nil
	}

	var out strings.Builder
	for _, item := range items {
		incoming, err := client.IncomingCalls(ctx,
			protocol.CallHierarchyIncomingCallsParams{Item: item})
		if err != nil {
			return "", fmt.Errorf("callHierarchy/incomingCalls failed: %v", err)
		}
		outgoing, err := client.OutgoingCalls(ctx,
			protocol.CallHierarchyOutgoingCallsParams{Item: item})
		if err != nil {
			return "", fmt.Errorf("callHierarchy/outgoingCalls failed: %v", err)
		}

		for i, in := range incoming {
			fmt.Fprintf(&out, "caller[%d]: %s in %s:%d:%d from %s in %s:%d:%d\n",
				i,
				formatRanges(in.FromRanges),
				strings.TrimPrefix(string(in.From.URI), "file://"),
				in.From.SelectionRange.Start.Line+1,
				in.From.SelectionRange.Start.Character+1,
				in.From.Name,
				strings.TrimPrefix(string(in.From.URI), "file://"),
				in.From.Range.Start.Line+1,
				in.From.Range.Start.Character+1,
			)
		}
		fmt.Fprintf(&out, "identifier: %s %s in %s:%d:%d\n",
			symbolKindName(item.Kind), item.Name,
			strings.TrimPrefix(string(item.URI), "file://"),
			item.SelectionRange.Start.Line+1,
			item.SelectionRange.Start.Character+1,
		)
		for i, o := range outgoing {
			fmt.Fprintf(&out, "callee[%d]: %s %s in %s:%d:%d\n",
				i,
				symbolKindName(o.To.Kind), o.To.Name,
				strings.TrimPrefix(string(o.To.URI), "file://"),
				o.To.SelectionRange.Start.Line+1,
				o.To.SelectionRange.Start.Character+1,
			)
		}
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// formatRanges renders a slice of ranges as `L:C-L:C` (the first
// range only — call hierarchy entries almost always have a single
// range, and multi-range cases are rare enough that we let the
// caller's `git grep` follow up if needed).
func formatRanges(rs []protocol.Range) string {
	if len(rs) == 0 {
		return ""
	}
	r := rs[0]
	return fmt.Sprintf("%d:%d-%d:%d",
		r.Start.Line+1, r.Start.Character+1,
		r.End.Line+1, r.End.Character+1,
	)
}
