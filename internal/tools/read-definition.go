package tools

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type ReadDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The exact name of the symbol to fetch. Method names must be fully specified e.g. MyClass.MyMethod"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=If true, adds line numbers to the output"`
}

func ReadDefinition(ctx context.Context, client *lsp.Client, args ReadDefinitionArgs) (string, error) {
	symbolName := args.SymbolName
	showLineNumbers := args.ShowLineNumbers

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
			if v.Kind == protocol.Method && strings.HasSuffix(symbol.GetName(), symbolName) {
				break
			}
			if symbol.GetName() != symbolName {
				continue
			}
		default:
			if symbol.GetName() != symbolName {
				continue
			}
		}

		log.Printf("Symbol: %s\n", symbol.GetName())
		loc := symbol.GetLocation()

		banner := strings.Repeat("=", 80) + "\n"
		definition, loc, err := GetFullDefinition(ctx, client, loc)
		locationInfo := fmt.Sprintf(
			"Symbol: %s\n"+
				"File: %s\n"+
				kind+
				container+
				"Start Position: Line %d, Column %d\n"+
				"End Position: Line %d, Column %d\n"+
				"%s\n",
			symbol.GetName(),
			strings.TrimPrefix(string(loc.URI), "file://"),
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
			loc.Range.End.Line+1,
			loc.Range.End.Character+1,
			strings.Repeat("=", 80))

		if err != nil {
			log.Printf("Error getting definition: %v\n", err)
			continue
		}

		if showLineNumbers {
			definition = addLineNumbers(definition, int(loc.Range.Start.Line)+1)
		}

		definitions = append(definitions, banner+locationInfo+definition+"\n")
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, "\n"), nil
}
