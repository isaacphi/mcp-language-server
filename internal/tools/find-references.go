package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type FindReferencesArgs struct {
	SymbolName      string `json:"symbolName"`
	ShowLineNumbers bool   `json:"showLineNumbers"`
}

func FindReferences(ctx context.Context, client *lsp.Client, args FindReferencesArgs) (string, error) {
	// First get the symbol location like ReadDefinition does
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: args.SymbolName,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("Failed to parse results: %v", err)
	}

	var allReferences []string
	for _, symbol := range results {
		if symbol.GetName() != args.SymbolName {
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
			return "", fmt.Errorf("Failed to get references: %v", err)
		}

		// Group references by file
		refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
		for _, ref := range refs {
			refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
		}

		banner := strings.Repeat("=", 80) + "\n"
		header := fmt.Sprintf("References for symbol: %s\n", args.SymbolName)
		allReferences = append(allReferences, banner+header+banner)

		// Process each file's references
		for uri, fileRefs := range refsByFile {
			fileInfo := fmt.Sprintf("\nFile: %s\n%s\n",
				strings.TrimPrefix(string(uri), "file://"),
				strings.Repeat("-", 80))
			allReferences = append(allReferences, fileInfo)

			for _, ref := range fileRefs {
				// Use GetFullDefinition but with a smaller context window
				snippet, _, err := GetFullDefinition(ctx, client, ref)
				if err != nil {
					continue
				}

				if args.ShowLineNumbers {
					snippet = addLineNumbers(snippet, int(ref.Range.Start.Line)+1)
				}

				refInfo := fmt.Sprintf("Reference at Line %d, Column %d:\n%s\n",
					ref.Range.Start.Line+1,
					ref.Range.Start.Character+1,
					snippet,
				)

				allReferences = append(allReferences, refInfo)
			}
		}
	}

	if len(allReferences) == 0 {
		banner := strings.Repeat("=", 80) + "\n"
		return fmt.Sprintf("%sNo references found for symbol: %s\n%s",
			banner, args.SymbolName, banner), nil
	}

	return strings.Join(allReferences, "\n"), nil
}

