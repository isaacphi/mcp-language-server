package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDiagnostics retrieves diagnostics for a specific file or all files from the language server
func GetDiagnostics(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	if filePath != "" {
		return getDiagnosticsForFile(ctx, client, filePath)
	}

	// Get all diagnostics from the cache
	allDiagnostics := client.GetAllDiagnostics()

	if len(allDiagnostics) == 0 {
		return "No diagnostics found in any files", nil
	}

	// Format all diagnostics, grouped by file
	var allFormattedDiagnostics []string

	for uri, diagnostics := range allDiagnostics {
		if len(diagnostics) == 0 {
			continue
		}

		// Add file header with banner
		filePath := strings.TrimPrefix(string(uri), "file://")
		fileHeader := fmt.Sprintf("\n%s\nFile: %s\nDiagnostics Count: %d\n%s\n",
			strings.Repeat("=", 60),
			filePath,
			len(diagnostics),
			strings.Repeat("=", 60))
		allFormattedDiagnostics = append(allFormattedDiagnostics, fileHeader)

		// Format the diagnostics for this file
		var fileDiagnostics []string
		for _, diag := range diagnostics {
			severity := getSeverityString(diag.Severity)
			location := fmt.Sprintf("Line %d, Column %d",
				diag.Range.Start.Line+1,
				diag.Range.Start.Character+1)

			formattedDiag := fmt.Sprintf(
				"[%s]\nLocation: %s\nMessage: %s\n",
				severity,
				location,
				diag.Message)

			if diag.Source != "" {
				formattedDiag += fmt.Sprintf("Source: %s\n", diag.Source)
			}

			if diag.Code != nil {
				formattedDiag += fmt.Sprintf("Code: %v\n", diag.Code)
			}

			formattedDiag += strings.Repeat("-", 40) + "\n"
			fileDiagnostics = append(fileDiagnostics, formattedDiag)
		}

		allFormattedDiagnostics = append(allFormattedDiagnostics, strings.Join(fileDiagnostics, "\n"))
	}

	return strings.Join(allFormattedDiagnostics, "\n"), nil
}

// GetDiagnostics retrieves diagnostics for a specific file from the language server
func getDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	// Convert the file path to URI format
	uri := protocol.DocumentUri("file://" + filePath)

	// Request fresh diagnostics
	diagParams := protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}
	result, err := client.Diagnostic(ctx, diagParams)
	if err != nil {
		return "", fmt.Errorf("failed to get diagnostics: %w", err)
	}
	fmt.Println("Result")
	fmt.Println(result)

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
