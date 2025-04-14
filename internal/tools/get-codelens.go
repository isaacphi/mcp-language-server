package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetCodeLens retrieves code lens hints for a given file location
func GetCodeLens(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}
	// TODO: find a more appropriate way to wait
	time.Sleep(time.Second)

	// Create document identifier
	docIdentifier := protocol.TextDocumentIdentifier{
		URI: protocol.DocumentUri("file://" + filePath),
	}

	// Request code lens from LSP
	params := protocol.CodeLensParams{
		TextDocument: docIdentifier,
	}
	codeLensResult, err := client.CodeLens(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get code lens: %w", err)
	}

	if codeLensResult == nil {
		return "No code lens providers available for this file.", nil
	}

	// Format the code lens results
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Code Lens results for %s:\n\n", filePath))

	for i, lens := range codeLensResult {
		output.WriteString(fmt.Sprintf("[%d] Location: Lines %d-%d\n",
			i+1,
			lens.Range.Start.Line+1,
			lens.Range.End.Line+1))

		if lens.Command != nil {
			output.WriteString(fmt.Sprintf("    Title: %s\n", lens.Command.Title))
			if lens.Command.Command != "" {
				output.WriteString(fmt.Sprintf("    Command: %s\n", lens.Command.Command))
			}
			if lens.Command.Arguments != nil {
				output.WriteString("    Arguments:\n")
				for _, arg := range lens.Command.Arguments {
					output.WriteString(fmt.Sprintf("%s\n", arg))
				}
			}
		}

		// Print any custom data that might help identify the provider
		if lens.Data != nil {
			output.WriteString("    Additional Data:\n")
			output.WriteString(fmt.Sprintf("%s\n", lens.Data))
		}
		output.WriteString("\n")
	}

	if len(codeLensResult) == 0 {
		output.WriteString("No code lens found for this file.\n")
	} else {
		output.WriteString(fmt.Sprintf("Found %d code lens items.\n", len(codeLensResult)))
	}

	return output.String(), nil
}
