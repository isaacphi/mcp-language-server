package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string, showLineNumbers bool) (string, error) {
	// Get context lines from environment variable
	contextLines := 5
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

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
			filePath := strings.TrimPrefix(uriStr, "file://")

			// Format file header
			fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n",
				filePath,
				len(fileRefs),
			)

			// Format locations with context
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				// Log error but continue with other files
				allReferences = append(allReferences, fileInfo+"\nError reading file: "+err.Error())
				continue
			}

			lines := strings.Split(string(fileContent), "\n")

			// Track reference locations for header display
			var locStrings []string
			for _, ref := range fileRefs {
				locStr := fmt.Sprintf("L%d:C%d",
					ref.Range.Start.Line+1,
					ref.Range.Start.Character+1)
				locStrings = append(locStrings, locStr)
			}

			// Collect lines to display using the utility function
			linesToShow, err := GetLineRangesToDisplay(ctx, client, fileRefs, len(lines), contextLines)
			if err != nil {
				// Log error but continue with other files
				continue
			}

			// Convert to line ranges using the utility function
			lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

			// Format with locations in header
			formattedOutput := fileInfo
			if len(locStrings) > 0 {
				formattedOutput += "At: " + strings.Join(locStrings, ", ") + "\n"
			}

			// Format the content with ranges
			formattedOutput += "\n" + FormatLinesWithRanges(lines, lineRanges)
			allReferences = append(allReferences, formattedOutput)
		}
	}

	if len(allReferences) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	return strings.Join(allReferences, "\n"), nil
}
