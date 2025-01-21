package tools

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDiagnostics retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		log.Fatalf("Could not open file: %v", err)
	}
	// Wait for diagnostics
	// TODO: wait for notification
	time.Sleep(time.Second * 3)

	// Convert the file path to URI format
	uri := protocol.DocumentUri("file://" + filePath)

	// Request fresh diagnostics
	diagParams := protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}
	_, err = client.Diagnostic(ctx, diagParams)
	if err != nil {
		log.Printf("failed to get diagnostics: %v", err)
	}

	// Get diagnostics from the cache
	diagnostics := client.GetFileDiagnostics(uri)

	if len(diagnostics) == 0 {
		return "No diagnostics found for " + filePath, nil
	}

	// Format the diagnostics
	var formattedDiagnostics []string
	for _, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("Line %d, Column %d",
			diag.Range.Start.Line+1,
			diag.Range.Start.Character+1)

		formattedDiag := fmt.Sprintf(
			"[%s] %s\n"+
				"Location: %s\n"+
				"Message: %s\n",
			severity,
			filePath,
			location,
			diag.Message)

		if diag.Source != "" {
			formattedDiag += fmt.Sprintf("Source: %s\n", diag.Source)
		}

		if diag.Code != nil {
			formattedDiag += fmt.Sprintf("Code: %v\n", diag.Code)
		}

		err := client.CloseFile(ctx, filePath)
		if err != nil {
			log.Printf("Could not close file: %v", err)
		}

		formattedDiagnostics = append(formattedDiagnostics, formattedDiag)
	}

	return strings.Join(formattedDiagnostics, "\n"), nil
}

func getSeverityString(severity protocol.DiagnosticSeverity) string {
	switch severity {
	case protocol.SeverityError:
		return "ERROR"
	case protocol.SeverityWarning:
		return "WARNING"
	case protocol.SeverityInformation:
		return "INFO"
	case protocol.SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}
