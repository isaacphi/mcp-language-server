package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func GetIncomingCalls(ctx context.Context, client *lsp.Client, symbolName string, maxDepth int) (string, error) {
	return getCallHierarchy(ctx, client, symbolName, maxDepth, recurseIncomingCalls)
}

func GetOutgoingCalls(ctx context.Context, client *lsp.Client, symbolName string, maxDepth int) (string, error) {
	return getCallHierarchy(ctx, client, symbolName, maxDepth, recurseOutgoingCalls)
}

func getCallHierarchy(
	ctx context.Context, client *lsp.Client, symbolName string, maxDepth int,
	recurse func(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, result *strings.Builder, depth int, maxDepth int),
) (string, error) {
	// First get the symbol location like ReadDefinition does
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	// After this point we just return errors instead of erroring out
	var result strings.Builder

	for _, symbol := range results {
		// Handle different matching strategies based on the search term
		if strings.Contains(symbolName, ".") {
			// For qualified names like "Type.Method", check for various matches
			parts := strings.Split(symbolName, ".")
			methodName := parts[len(parts)-1]

			// Try matching the unqualified method name for languages that don't use qualified names in symbols
			if symbol.GetName() != symbolName && symbol.GetName() != methodName {
				continue
			}
		} else if symbol.GetName() != symbolName {
			// For unqualified names, exact match only
			continue
		}

		result.WriteString("\n---\n")

		// Get the location of the symbol
		loc := symbol.GetLocation()

		chParams := protocol.CallHierarchyPrepareParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: loc.URI,
				},
				Position: loc.Range.Start,
			},
		}
		items, err := client.PrepareCallHierarchy(ctx, chParams)
		if err != nil {
			result.WriteString(fmt.Sprintf("%s: Error: %v\n", symbol.GetName(), err))
			continue
		}

		for _, item := range items {
			recurse(ctx, client, item, &result, 0, maxDepth)
		}
	}

	return result.String(), nil
}

func recurseIncomingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, result *strings.Builder, depth int, maxDepth int) {

	var prefix string
	if depth != 0 {
		prefix = strings.Repeat(" ", (depth-1)*2+2)

		result.WriteString(strings.Repeat(" ", (depth-1)*2))
		result.WriteRune('-')
		result.WriteString(" Called By: ")
	} else {
		result.WriteString("Name: ")
	}

	result.WriteString(item.Name)
	result.WriteRune('\n')

	result.WriteString(prefix)
	result.WriteString("Detail: ")
	result.WriteString(item.Detail)
	result.WriteRune('\n')

	if depth >= maxDepth {
		return
	}

	calls, err := client.IncomingCalls(ctx, protocol.CallHierarchyIncomingCallsParams{
		Item: item,
	})

	if err != nil {
		result.WriteString(prefix)
		result.WriteString("Error: ")
		result.WriteString(err.Error())
		result.WriteRune('\n')
		return
	}

	// ensure output is deterministic for tests
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].From.Name < calls[j].From.Name
	})

	for _, call := range calls {
		recurseIncomingCalls(ctx, client, call.From, result, depth+1, maxDepth)
	}
}

func recurseOutgoingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, result *strings.Builder, depth int, maxDepth int) {

	var prefix string
	if depth != 0 {
		prefix = strings.Repeat(" ", (depth-1)*2+2)

		result.WriteString(strings.Repeat(" ", (depth-1)*2))
		result.WriteRune('-')
		result.WriteString(" Calls: ")
	} else {
		result.WriteString("Name: ")
	}

	result.WriteString(item.Name)
	result.WriteRune('\n')

	result.WriteString(prefix)
	result.WriteString("Detail: ")
	result.WriteString(item.Detail)
	result.WriteRune('\n')

	if depth >= maxDepth {
		return
	}

	calls, err := client.OutgoingCalls(ctx, protocol.CallHierarchyOutgoingCallsParams{
		Item: item,
	})

	if err != nil {
		result.WriteString(prefix)
		result.WriteString("Error: ")
		result.WriteString(err.Error())
		result.WriteRune('\n')
		return
	}

	// ensure output is deterministic for tests
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].To.Name < calls[j].To.Name
	})

	for _, call := range calls {
		recurseOutgoingCalls(ctx, client, call.To, result, depth+1, maxDepth)
	}
}
