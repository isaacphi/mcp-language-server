package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// DefPosition is an optional file position (1-based line and character).
// When provided, definition is resolved via textDocument/definition at that
// position. For clangd (C/C++), prefer this when workspace/symbol returns no results.
type DefPosition struct {
	FilePath  string
	Line      int // 1-based
	Character int // 1-based
}

// definitionLocationFromResult returns the first Location from a definition result.
func definitionLocationFromResult(result protocol.Or_Result_textDocument_definition) (protocol.Location, bool) {
	if result.Value == nil {
		return protocol.Location{}, false
	}
	switch v := result.Value.(type) {
	case protocol.Location:
		return v, true
	case protocol.Or_Definition:
		if v.Value == nil {
			return protocol.Location{}, false
		}
		switch w := v.Value.(type) {
		case protocol.Location:
			return w, true
		case []protocol.Location:
			if len(w) == 0 {
				return protocol.Location{}, false
			}
			return w[0], true
		}
		return protocol.Location{}, false
	case []protocol.LocationLink:
		if len(v) == 0 {
			return protocol.Location{}, false
		}
		link := v[0]
		return protocol.Location{URI: link.TargetURI, Range: link.TargetRange}, true
	default:
		return protocol.Location{}, false
	}
}

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string, position *DefPosition) (string, error) {
	// Position-based path: use textDocument/definition at the given location.
	// Use when workspace/symbol returns [] (e.g. clangd before indexing).
	if position != nil && position.FilePath != "" && position.Line > 0 {
		line0 := position.Line - 1
		if line0 < 0 {
			line0 = 0
		}
		char0 := position.Character - 1
		if char0 < 0 {
			char0 = 0
		}
		uri := protocol.URIFromPath(position.FilePath)
		params := protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: uint32(line0), Character: uint32(char0)},
			},
		}
		if err := client.OpenFile(ctx, position.FilePath); err != nil {
			return "", fmt.Errorf("failed to open file for definition: %v", err)
		}
		result, err := client.Definition(ctx, params)
		if err != nil {
			return "", fmt.Errorf("failed to get definition at position: %v", err)
		}
		loc, ok := definitionLocationFromResult(result)
		if !ok {
			return fmt.Sprintf("%s not found at given position", symbolName), nil
		}
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening definition file: %v", err)
			return "", fmt.Errorf("failed to open definition file: %v", err)
		}
		definition, defLoc, err := GetFullDefinition(ctx, client, loc)
		if err != nil {
			return "", fmt.Errorf("failed to get full definition: %v", err)
		}
		locationInfo := fmt.Sprintf(
			"Symbol: %s\nFile: %s\nRange: L%d:C%d - L%d:C%d\n\n",
			symbolName,
			strings.TrimPrefix(string(loc.URI), "file://"),
			defLoc.Range.Start.Line+1, defLoc.Range.Start.Character+1,
			defLoc.Range.End.Line+1, defLoc.Range.End.Character+1,
		)
		definition = addLineNumbers(definition, int(defLoc.Range.Start.Line)+1)
		return "---\n\n" + locationInfo + definition + "\n", nil
	}

	// Symbol-based path: find symbol via workspace/symbol, then get definition.
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
