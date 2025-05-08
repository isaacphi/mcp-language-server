package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
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

	var definitions []string
	for _, symbol := range results {
		kind := ""
		container := ""

		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
			// SymbolInformation results have richer data.
			kind = fmt.Sprintf("Kind: %s\n", protocol.TableKindMap[v.Kind])
			if v.ContainerName != "" {
				container = fmt.Sprintf("Container Name: %s\n", v.ContainerName)
			}

			// Handle different matching strategies based on the search term
			if strings.Contains(symbolName, ".") {
				// For qualified names like "Type.Method", require exact match
				if symbol.GetName() != symbolName {
					continue
				}
			} else {
				// For unqualified names like "Method"
				if v.Kind == protocol.Method {
					// For methods, only match if the method name matches exactly Type.symbolName or Type::symbolName or symbolName
					if !strings.HasSuffix(symbol.GetName(), "::"+symbolName) && !strings.HasSuffix(symbol.GetName(), "."+symbolName) && symbol.GetName() != symbolName {
						continue
					}
				} else if symbol.GetName() != symbolName {
					// For non-methods, exact match only
					continue
				}
			}
		default:
			if symbol.GetName() != symbolName {
				continue
			}
		}

		toolsLogger.Debug("Found symbol: %s", symbol.GetName())
		loc := symbol.GetLocation()

		err := client.OpenFile(ctx, loc.URI.Path())
		if err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}

		banner := "---\n\n"
		definition, loc, err := GetFullDefinition(ctx, client, loc)
		locationInfo := fmt.Sprintf(
			"Symbol: %s\n"+
				"File: %s\n"+
				kind+
				container+
				"Range: L%d:C%d - L%d:C%d\n\n",
			symbol.GetName(),
			strings.TrimPrefix(string(loc.URI), "file://"),
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
			loc.Range.End.Line+1,
			loc.Range.End.Character+1,
		)

		if err != nil {
			toolsLogger.Error("Error getting definition: %v", err)
			continue
		}

		definition = addLineNumbers(definition, int(loc.Range.Start.Line)+1)

		definitions = append(definitions, banner+locationInfo+definition+"\n")
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
}
