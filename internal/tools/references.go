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

// RefPosition is an optional file position (1-based line and character).
// When provided, references are resolved via textDocument/references at that
// position instead of workspace/symbol. Use this when the file may not be
// indexed yet (e.g. C++/clangd) or when workspace/symbol returns no results.
type RefPosition struct {
	FilePath  string
	Line     int // 1-based
	Character int // 1-based
}

// FindReferences finds all references to a symbol. If position is non-nil and
// has a valid file path and line, references are requested at that position
// (works even when the file is not indexed). Otherwise workspace/symbol is
// used to find the symbol first.
func FindReferences(ctx context.Context, client *lsp.Client, symbolName string, position *RefPosition) (string, error) {
	contextLines := 5
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	var refs []protocol.Location

	if position != nil && position.FilePath != "" && position.Line > 0 {
		// Position-based path: call textDocument/references directly. Use when the
		// file may not be indexed (e.g. clangd) or when workspace/symbol returns empty.
		uri := protocol.URIFromPath(position.FilePath)
		line0 := position.Line - 1
		if line0 < 0 {
			line0 = 0
		}
		char0 := position.Character - 1
		if char0 < 0 {
			char0 = 0
		}
		refsParams := protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position: protocol.Position{
					Line:      uint32(line0),
					Character: uint32(char0),
				},
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: false},
		}
		if err := client.OpenFile(ctx, position.FilePath); err != nil {
			return "", fmt.Errorf("failed to open file for references: %v", err)
		}
		var err error
		refs, err = client.References(ctx, refsParams)
		if err != nil {
			return "", fmt.Errorf("failed to get references at position: %v", err)
		}
	}

	if len(refs) == 0 {
		// Symbol-based path: find symbol via workspace/symbol, then get references.
		symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: symbolName})
		if err != nil {
			return "", fmt.Errorf("failed to fetch symbol: %v", err)
		}
		results, err := symbolResult.Results()
		if err != nil {
			return "", fmt.Errorf("failed to parse results: %v", err)
		}

		for _, symbol := range results {
			if strings.Contains(symbolName, ".") {
				parts := strings.Split(symbolName, ".")
				methodName := parts[len(parts)-1]
				if symbol.GetName() != symbolName && symbol.GetName() != methodName {
					continue
				}
			} else if symbol.GetName() != symbolName {
				continue
			}
			loc := symbol.GetLocation()
			refsParams := protocol.ReferenceParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
					Position:     loc.Range.Start,
				},
				Context: protocol.ReferenceContext{IncludeDeclaration: false},
			}
			if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
				toolsLogger.Error("Error opening file: %v", err)
				continue
			}
			symRefs, err := client.References(ctx, refsParams)
			if err != nil {
				return "", fmt.Errorf("failed to get references: %v", err)
			}
			refs = symRefs
			break
		}
	}

	allReferences := formatReferenceLocations(ctx, client, refs, contextLines)
	if len(allReferences) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}
	return strings.Join(allReferences, "\n"), nil
}

func formatReferenceLocations(ctx context.Context, client *lsp.Client, refs []protocol.Location, contextLines int) []string {
	if len(refs) == 0 {
		return nil
	}
	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range refs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}
	uris := make([]string, 0, len(refsByFile))
	for uri := range refsByFile {
		uris = append(uris, string(uri))
	}
	sort.Strings(uris)

	var out []string
	for _, uriStr := range uris {
		uri := protocol.DocumentUri(uriStr)
		fileRefs := refsByFile[uri]
		filePath := strings.TrimPrefix(uriStr, "file://")

		fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n", filePath, len(fileRefs))
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			out = append(out, fileInfo+"\nError reading file: "+err.Error())
			continue
		}
		lines := strings.Split(string(fileContent), "\n")

		var locStrings []string
		for _, ref := range fileRefs {
			locStrings = append(locStrings, fmt.Sprintf("L%d:C%d", ref.Range.Start.Line+1, ref.Range.Start.Character+1))
		}
		linesToShow, err := GetLineRangesToDisplay(ctx, client, fileRefs, len(lines), contextLines)
		if err != nil {
			continue
		}
		lineRanges := ConvertLinesToRanges(linesToShow, len(lines))
		formatted := fileInfo
		if len(locStrings) > 0 {
			formatted += "At: " + strings.Join(locStrings, ", ") + "\n"
		}
		formatted += "\n" + FormatLinesWithRanges(lines, lineRanges)
		out = append(out, formatted)
	}
	return out
}
