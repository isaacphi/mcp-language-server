package tools

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ExecuteCodeLens executes a specific code lens command from a file.
func ExecuteCodeLens(ctx context.Context, client *lsp.Client, filePath string, index int) (string, error) {
	// Open the file
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}
	defer func() {
		if err := client.CloseFile(ctx, filePath); err != nil {
			log.Printf("Could not close file: %v", err)
		}
	}()
	time.Sleep(time.Second)

	// Get code lenses
	docIdentifier := protocol.TextDocumentIdentifier{
		URI: protocol.DocumentUri("file://" + filePath),
	}

	params := protocol.CodeLensParams{
		TextDocument: docIdentifier,
	}
	codeLenses, err := client.CodeLens(ctx, params)
	if err != nil {
		return "", fmt.Errorf("Failed to get code lenses: %v", err)
	}

	if len(codeLenses) == 0 {
		return "", fmt.Errorf("No code lenses found in file")
	}

	if index < 1 || index > len(codeLenses) {
		return "", fmt.Errorf("Invalid code lens index: %d. Available range: 1-%d", index, len(codeLenses))
	}

	lens := codeLenses[index-1]

	// Resolve the code lens if it doesn't have a command
	if lens.Command == nil {
		resolvedLens, err := client.ResolveCodeLens(ctx, lens)
		if err != nil {
			return "", fmt.Errorf("Failed to resolve code lens: %v", err)
		}
		lens = resolvedLens
	}

	if lens.Command == nil {
		return "", fmt.Errorf("Code lens has no command after resolution")
	}

	// Execute the command
	_, err = client.ExecuteCommand(ctx, protocol.ExecuteCommandParams{
		Command:   lens.Command.Command,
		Arguments: lens.Command.Arguments,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to execute code lens command: %v", err)
	}

	return fmt.Sprintf("Successfully executed code lens command: %s", lens.Command.Title), nil
}
