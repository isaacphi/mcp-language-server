package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetHoverInfo retrieves hover information (type, documentation) for a symbol at the specified position
func GetHoverInfo(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	params := protocol.HoverParams{}

	// Convert 1-indexed line/column to 0-indexed for LSP protocol
	position := protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(column - 1),
	}
	uri := protocol.DocumentUri("file://" + filePath)
	params.TextDocument = protocol.TextDocumentIdentifier{
		URI: uri,
	}
	params.Position = position

	// Execute the hover request
	hoverResult, err := client.Hover(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get hover information: %v", err)
	}

	var result strings.Builder

	// Process the hover contents: MarkedString | []MarkedString | MarkupContent
	contents := renderHoverContents(hoverResult.Contents)
	if contents == "" {
		// Extract the line where the hover was requested. Use a single-line
		// range (start and end on the same line) so this also works when the
		// symbol is on the file's last line; spanning to the start of the next
		// line would push End.Line past the last index and fail the bounds
		// check in ExtractTextFromLocation.
		lineText, err := extractLineText(uri, position.Line)
		if err != nil {
			toolsLogger.Warn("failed to extract line at position: %v", err)
		}
		result.WriteString(fmt.Sprintf("No hover information available for this position on the following line:\n%s", lineText))
	} else {
		result.WriteString(contents)
	}

	return result.String(), nil
}

// renderHoverContents converts any of the three hover content shapes
// (MarkupContent, MarkedString, []MarkedString) to a markdown string.
func renderHoverContents(contents protocol.Or_Hover_contents) string {
	switch v := contents.Value.(type) {
	case protocol.MarkupContent:
		return v.Value
	case protocol.MarkedString:
		return renderMarkedString(v)
	case []protocol.MarkedString:
		parts := make([]string, 0, len(v))
		for _, ms := range v {
			if s := renderMarkedString(ms); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// renderMarkedString converts a MarkedString (plain string or
// {language, value} code block) to markdown.
func renderMarkedString(ms protocol.MarkedString) string {
	switch v := ms.Value.(type) {
	case string:
		return v
	case protocol.MarkedStringWithLanguage:
		return fmt.Sprintf("```%s\n%s\n```", v.Language, v.Value)
	}
	return ""
}
