package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ReadDefinitionAt reads the source definition for the symbol at a concrete file position.
func ReadDefinitionAt(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.DocumentUri("file://" + filePath)
	position := protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(column - 1),
	}
	params := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	definitionResult, err := client.Definition(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get definition: %v", err)
	}

	locations := definitionLocations(definitionResult)
	if len(locations) == 0 {
		return fmt.Sprintf("No definition found at %s:L%d:C%d", filePath, line, column), nil
	}

	var definitions []string
	for _, loc := range locations {
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening definition file: %v", err)
			continue
		}

		definition, fullLoc, err := GetFullDefinition(ctx, client, loc)
		if err != nil {
			toolsLogger.Error("Error getting full definition: %v", err)
			definition, err = ExtractTextFromLocation(loc)
			if err != nil {
				toolsLogger.Error("Error extracting definition text: %v", err)
				continue
			}
			fullLoc = loc
		}

		locationInfo := fmt.Sprintf(
			"---\n\nFile: %s\nRange: L%d:C%d - L%d:C%d\n\n",
			strings.TrimPrefix(string(fullLoc.URI), "file://"),
			fullLoc.Range.Start.Line+1,
			fullLoc.Range.Start.Character+1,
			fullLoc.Range.End.Line+1,
			fullLoc.Range.End.Character+1,
		)
		definitions = append(definitions, locationInfo+addLineNumbers(definition, int(fullLoc.Range.Start.Line)+1)+"\n")
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("No definition found at %s:L%d:C%d", filePath, line, column), nil
	}
	return strings.Join(definitions, ""), nil
}

func definitionLocations(result protocol.Or_Result_textDocument_definition) []protocol.Location {
	switch value := result.Value.(type) {
	case nil:
		return nil
	case protocol.Definition:
		return definitionValueLocations(value)
	case []protocol.DefinitionLink:
		locations := make([]protocol.Location, 0, len(value))
		for _, link := range value {
			locations = append(locations, protocol.Location{URI: link.TargetURI, Range: link.TargetRange})
		}
		return locations
	case protocol.Location:
		return []protocol.Location{value}
	case []protocol.Location:
		return value
	default:
		toolsLogger.Warn("Unhandled definition result type: %T", value)
		return nil
	}
}

func definitionValueLocations(definition protocol.Definition) []protocol.Location {
	switch value := definition.Value.(type) {
	case nil:
		return nil
	case protocol.Location:
		return []protocol.Location{value}
	case []protocol.Location:
		return value
	default:
		toolsLogger.Warn("Unhandled definition value type: %T", value)
		return nil
	}
}

// FindReferencesAt finds references for the symbol at a concrete file position.
func FindReferencesAt(ctx context.Context, client *lsp.Client, filePath string, line, column int, includeDeclaration bool, contextLines int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.DocumentUri("file://" + filePath)
	position := protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(column - 1),
	}
	refsParams := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: includeDeclaration},
	}

	refs, err := client.References(ctx, refsParams)
	if err != nil {
		return "", fmt.Errorf("failed to get references: %v", err)
	}
	if len(refs) == 0 {
		return fmt.Sprintf("No references found at %s:L%d:C%d", filePath, line, column), nil
	}
	return FormatReferences(ctx, client, refs, contextLines)
}

func FormatReferences(ctx context.Context, client *lsp.Client, refs []protocol.Location, contextLines int) (string, error) {
	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range refs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}

	uris := make([]string, 0, len(refsByFile))
	for uri := range refsByFile {
		uris = append(uris, string(uri))
	}
	sort.Strings(uris)

	var allReferences []string
	for _, uriStr := range uris {
		uri := protocol.DocumentUri(uriStr)
		fileRefs := refsByFile[uri]
		filePath := strings.TrimPrefix(uriStr, "file://")

		fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n", filePath, len(fileRefs))
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			allReferences = append(allReferences, fileInfo+"\nError reading file: "+err.Error())
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

		formattedOutput := fileInfo
		if len(locStrings) > 0 {
			formattedOutput += "At: " + strings.Join(locStrings, ", ") + "\n"
		}
		formattedOutput += "\n" + FormatLinesWithRanges(lines, ConvertLinesToRanges(linesToShow, len(lines)))
		allReferences = append(allReferences, formattedOutput)
	}

	if len(allReferences) == 0 {
		return "No references found", nil
	}
	return strings.Join(allReferences, "\n"), nil
}
