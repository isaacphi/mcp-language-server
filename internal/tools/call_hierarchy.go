package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const maxCallHierarchyDepth = 50

func itemKey(item protocol.CallHierarchyItem) string {
	return fmt.Sprintf("%s|%d:%d", item.URI, item.Range.Start.Line, item.Range.Start.Character)
}

func itemShortLabel(item protocol.CallHierarchyItem) string {
	file := strings.TrimPrefix(string(item.URI), "file://")
	return fmt.Sprintf("%s at %s L%d:C%d", item.Name, file, item.Range.Start.Line+1, item.Range.Start.Character+1)
}

// GetFullCallHierarchy builds the entire incoming call tree at the given position,
// with cycle detection (who calls this symbol, and who calls those callers, etc.).
// Position is 1-based line and character. The file is opened if needed.
func GetFullCallHierarchy(ctx context.Context, client *lsp.Client, filePath string, line, character int) (string, error) {
	if filePath == "" || line < 1 {
		return "", fmt.Errorf("filePath and line (1-based) are required")
	}

	line0 := line - 1
	if line0 < 0 {
		line0 = 0
	}
	char0 := character - 1
	if char0 < 0 {
		char0 = 0
	}

	uri := protocol.URIFromPath(filePath)
	prepareParams := protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: uint32(line0), Character: uint32(char0)},
		},
	}

	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("failed to open file for call hierarchy: %v", err)
	}

	items, err := client.PrepareCallHierarchy(ctx, prepareParams)
	if err != nil {
		return "", fmt.Errorf("failed to prepare call hierarchy: %v", err)
	}

	if len(items) == 0 {
		return "No call hierarchy item at this position (e.g. not a function or method).", nil
	}

	visited := make(map[string]bool)
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatFullCallTree(ctx, client, item, visited, 0))
	}
	return b.String(), nil
}

func formatFullCallTree(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, visited map[string]bool, depth int) string {
	if depth >= maxCallHierarchyDepth {
		return itemShortLabel(item) + "\n  [max depth reached]"
	}
	key := itemKey(item)
	if visited[key] {
		return itemShortLabel(item) + "\n  [cycle - already visited]"
	}
	visited[key] = true
	defer func() { delete(visited, key) }()

	var b strings.Builder
	b.WriteString(itemShortLabel(item))

	incoming, err := client.IncomingCalls(ctx, protocol.CallHierarchyIncomingCallsParams{Item: item})
	if err != nil {
		toolsLogger.Error("Failed to get incoming calls for %q: %v", item.Name, err)
		b.WriteString("\n  [error getting incoming calls]")
	} else if len(incoming) > 0 {
		byFrom := make(map[string]protocol.CallHierarchyItem)
		for _, c := range incoming {
			k := itemKey(c.From)
			if _, ok := byFrom[k]; !ok {
				byFrom[k] = c.From
			}
		}
		keys := make([]string, 0, len(byFrom))
		for k := range byFrom {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			caller := byFrom[k]
			sub := formatFullCallTree(ctx, client, caller, visited, depth+1)
			lines := strings.Split(sub, "\n")
			b.WriteString("\n  ← ")
			b.WriteString(lines[0])
			for _, line := range lines[1:] {
				b.WriteString("\n    ")
				b.WriteString(line)
			}
		}
	}

	return b.String()
}
