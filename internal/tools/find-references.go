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

		// Process each file's references
		for uri, fileRefs := range refsByFile {
			fileInfo := fmt.Sprintf("\nReferences in %s:\n", strings.TrimPrefix(string(uri), "file://"))
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

				refInfo := fmt.Sprintf("  Line %d, Column %d:\n%s\n",
					ref.Range.Start.Line+1,
					ref.Range.Start.Character+1,
					snippet,
				)

				allReferences = append(allReferences, refInfo)
			}
		}
	}

	if len(allReferences) == 0 {
		return fmt.Sprintf("No references found for %s", args.SymbolName), nil
	}

	return strings.Join(allReferences, "\n"), nil
}

