package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string, showLineNumbers bool) (string, error) {
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

	var allReferences []string
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

		// Get the location of the symbol
		loc := symbol.GetLocation()

		// Use LSP references request with correct params structure
		refsParams := protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: loc.URI,
				},
				Position: loc.Range.Start,
			},
			Context: protocol.ReferenceContext{
				IncludeDeclaration: false,
			},
		}

		refs, err := client.References(ctx, refsParams)
		if err != nil {
			return "", fmt.Errorf("failed to get references: %v", err)
		}

		// Group references by file
		refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
		for _, ref := range refs {
			refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
		}

		// Get sorted list of URIs
		uris := make([]string, 0, len(refsByFile))
		for uri := range refsByFile {
			uris = append(uris, string(uri))
		}
		sort.Strings(uris)

		// Process each file's references in sorted order
		for _, uriStr := range uris {
			uri := protocol.DocumentUri(uriStr)
			fileRefs := refsByFile[uri]

			// Format file header similarly to ReadDefinition style
			fileInfo := fmt.Sprintf("%s\nFile: %s\nReferences in File: %d\n%s\n",
				strings.Repeat("=", 3),
				strings.TrimPrefix(uriStr, "file://"),
				len(fileRefs),
				strings.Repeat("=", 3))
			allReferences = append(allReferences, fileInfo)

			for _, ref := range fileRefs {
				// Use GetFullDefinition but with a smaller context window
				snippet, snippetLocation, err := GetFullDefinition(ctx, client, ref)
				if err != nil {
					continue
				}

				if showLineNumbers {
					snippet = addLineNumbers(snippet, int(snippetLocation.Range.Start.Line)+1)
				}

				// Format reference location info
				refInfo := fmt.Sprintf("Reference at Line %d, Column %d:\n%s\n",
					ref.Range.Start.Line+1,
					ref.Range.Start.Character+1,
					snippet)

				allReferences = append(allReferences, refInfo)
			}
		}

	}

	if len(allReferences) == 0 {
		banner := strings.Repeat("=", 3) + "\n"
		return fmt.Sprintf("%sNo references found for symbol: %s\n%s",
			banner, symbolName, banner), nil
	}

	return strings.Join(allReferences, "\n"), nil
}
