package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
    // Normalize common user inputs, e.g. "foo()" -> "foo"
    normalized := strings.TrimSpace(symbolName)
    if strings.HasSuffix(normalized, "()") {
        normalized = strings.TrimSuffix(normalized, "()")
    }

    symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
        Query: normalized,
    })
	if err != nil {
		return "", fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	var definitions []string
    // Determine if the query is qualified (e.g., Class.method or ns::Class)
    isQualified := strings.Contains(normalized, ".") || strings.Contains(normalized, "::")
    // Extract container and member parts if qualified
    containerWanted := ""
    memberWanted := normalized
    if isQualified {
        // Split on last occurrence of '.' or '::'
        // Prefer the right-most separator to support nested containers
        if idx := strings.LastIndex(normalized, "::"); idx != -1 {
            containerWanted = normalized[:idx]
            memberWanted = normalized[idx+2:]
        } else if idx := strings.LastIndex(normalized, "."); idx != -1 {
            containerWanted = normalized[:idx]
            memberWanted = normalized[idx+1:]
        }
    }

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

            // Matching strategy
            name := symbol.GetName()
            if isQualified {
                // Qualified lookup
                // Accept if:
                // - name equals memberWanted; and, if we know the container, it matches containerWanted (exact or suffix)
                // - or name equals the full normalized string (some servers include container in name)
                if name != memberWanted && name != normalized && !strings.HasSuffix(name, "::"+memberWanted) && !strings.HasSuffix(name, "."+memberWanted) {
                    continue
                }
                if containerWanted != "" && v.ContainerName != "" {
                    // containerName may be only the immediate container. Allow suffix match to tolerate nesting
                    if v.ContainerName != containerWanted &&
                        !strings.HasSuffix(v.ContainerName, "."+containerWanted) &&
                        !strings.HasSuffix(v.ContainerName, "::"+containerWanted) &&
                        !strings.HasSuffix(containerWanted, "."+v.ContainerName) &&
                        !strings.HasSuffix(containerWanted, "::"+v.ContainerName) {
                        continue
                    }
                }
            } else {
                // Unqualified lookup
                if name != normalized {
                    // For methods, also allow suffix match like *.method
                    if v.Kind == protocol.Method {
                        if !strings.HasSuffix(name, "."+normalized) && !strings.HasSuffix(name, "::"+normalized) {
                            continue
                        }
                    } else {
                        continue
                    }
                }
            }
        default:
            if !isQualified {
                if symbol.GetName() != normalized {
                    continue
                }
            } else {
                // For unknown result types, require exact match with full normalized string
                if symbol.GetName() != normalized &&
                    !strings.HasSuffix(symbol.GetName(), "."+memberWanted) &&
                    !strings.HasSuffix(symbol.GetName(), "::"+memberWanted) {
                    continue
                }
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
