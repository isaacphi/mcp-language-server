package tools

import (
	"context"
	"fmt"
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
		return "", fmt.Errorf("Failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("Failed to parse results: %v", err)
	}

	var allReferences []string
	for _, symbol := range results {
		if symbol.GetName() != symbolName {
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
			// Format file header similarly to ReadDefinition style
			fileInfo := fmt.Sprintf("\n%s\nFile: %s\nReferences in File: %d\n%s\n",
				strings.Repeat("=", 60),
				strings.TrimPrefix(string(uri), "file://"),
				len(fileRefs),
				strings.Repeat("=", 60))
			allReferences = append(allReferences, fileInfo)

			for _, ref := range fileRefs {
				// Use GetFullDefinition but with a smaller context window
				snippet, _, err := GetFullDefinition(ctx, client, ref)
				if err != nil {
					continue
				}

				if showLineNumbers {
					snippet = addLineNumbers(snippet, int(ref.Range.Start.Line)+1)
				}

				// Format reference location info
				refInfo := fmt.Sprintf("Reference at Line %d, Column %d:\n%s\n%s\n",
					ref.Range.Start.Line+1,
					ref.Range.Start.Character+1,
					strings.Repeat("-", 40),
					snippet)

				allReferences = append(allReferences, refInfo)
			}
		}
	}

	if len(allReferences) == 0 {
		banner := strings.Repeat("=", 80) + "\n"
		return fmt.Sprintf("%sNo references found for symbol: %s\n%s",
			banner, symbolName, banner), nil
	}

	return strings.Join(allReferences, "\n"), nil
}
