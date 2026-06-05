package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	results, err := workspaceSymbols(ctx, client, symbolName)
	if err != nil {
		return "", err
	}

	var definitions []string
	for _, symbol := range results {
		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
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

		symPath, err := uriToPath(symbol.GetLocation().URI)
		if err != nil {
			toolsLogger.Error("Error resolving symbol path: %v", err)
			continue
		}
		if err := client.OpenFile(ctx, symPath); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}

		definition, loc, err := GetFullDefinition(ctx, client, symbol.GetLocation())
		if err != nil {
			toolsLogger.Error("Error getting definition: %v", err)
			continue
		}

		// Extract kind/container for both SymbolInformation and WorkspaceSymbol
		// results so the header is consistent regardless of which shape the
		// server returns.
		kind, container := symbolKindAndContainer(symbol)
		definitions = append(definitions, formatDefinitionEntry(symbol.GetName(), kind, container, definition, loc))
	}

	// Fallback for qualified names that workspace/symbol could not match
	// directly. Some servers (e.g. jdtls) only index types, so fully
	// qualified names ("com.example.Foo") and member symbols
	// ("Class.method") never appear in workspace/symbol results.
	if len(definitions) == 0 && strings.Contains(symbolName, ".") {
		entries, err := resolveQualifiedEntries(ctx, client, symbolName, func(sym resolvedSymbol) ([]string, error) {
			symPath, err := uriToPath(sym.Location.URI)
			if err != nil {
				return nil, err
			}
			if err := client.OpenFile(ctx, symPath); err != nil {
				return nil, fmt.Errorf("error opening file: %v", err)
			}
			definition, loc, err := GetFullDefinition(ctx, client, sym.Location)
			if err != nil {
				return nil, fmt.Errorf("error getting definition: %v", err)
			}
			return []string{formatDefinitionEntry(sym.Name, sym.Kind, sym.ContainerName, definition, loc)}, nil
		})
		if err != nil {
			return "", err
		}
		definitions = append(definitions, entries...)
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
}

// formatDefinitionEntry renders a single definition block. It is shared by the
// workspace/symbol path and the qualified-name fallback so the header format
// stays in one place. kindName/containerName are raw names; their labels are
// emitted only when non-empty.
func formatDefinitionEntry(name, kindName, containerName, definition string, loc protocol.Location) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Symbol: %s\n", name)
	// Display the filesystem path for file:// URIs; for anything else (e.g. a
	// jdt:// URI from jdtls) fall back to the raw URI rather than panicking.
	displayPath, err := uriToPath(loc.URI)
	if err != nil {
		displayPath = string(loc.URI)
	}
	fmt.Fprintf(&b, "File: %s\n", displayPath)
	// kindName/containerName are server-provided; pass them as args (never as
	// part of the format string) so a '%' in a name is not read as a verb.
	if kindName != "" {
		fmt.Fprintf(&b, "Kind: %s\n", kindName)
	}
	if containerName != "" {
		fmt.Fprintf(&b, "Container Name: %s\n", containerName)
	}
	fmt.Fprintf(&b, "Range: L%d:C%d - L%d:C%d\n\n",
		loc.Range.Start.Line+1,
		loc.Range.Start.Character+1,
		loc.Range.End.Line+1,
		loc.Range.End.Character+1,
	)

	return "---\n\n" + b.String() + addLineNumbers(definition, int(loc.Range.Start.Line)+1) + "\n"
}
