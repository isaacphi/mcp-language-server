package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDiagnostics retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string, includeContext bool, showLineNumbers bool) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
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

	// Create a summary header
	summary := fmt.Sprintf("Diagnostics for %s (%d issues)\n",
		filePath,
		len(diagnostics))

	// Format the diagnostics
	var formattedDiagnostics []string
	formattedDiagnostics = append(formattedDiagnostics, summary)

	for i, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("L%d:C%d",
			diag.Range.Start.Line+1,
			diag.Range.Start.Character+1)

		// Get the file content for context if needed
		var codeContext string
		var startLine uint32

		// Always get at least the line with the diagnostic
		content, err := os.ReadFile(filePath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			if int(diag.Range.Start.Line) < len(lines) {
				codeContext = strings.TrimSpace(lines[diag.Range.Start.Line])

				// Truncate line if it's too long
				const maxLineLength = 80
				if len(codeContext) > maxLineLength {
					startChar := int(diag.Range.Start.Character)
					if startChar > maxLineLength/2 {
						codeContext = "..." + codeContext[startChar-maxLineLength/2:]
					}
					if len(codeContext) > maxLineLength {
						codeContext = codeContext[:maxLineLength] + "..."
					}
				}
			}
		}

		// Get more context if requested
		if includeContext {
			extendedContext, loc, err := GetFullDefinition(ctx, client, protocol.Location{
				URI:   uri,
				Range: diag.Range,
			})
			if err == nil {
				startLine = loc.Range.Start.Line + 1
				if showLineNumbers {
					extendedContext = addLineNumbers(extendedContext, int(startLine))
				}
				codeContext = extendedContext
			}
		}

		// Create a concise diagnostic entry
		var formattedDiag strings.Builder
		formattedDiag.WriteString(fmt.Sprintf("%d. [%s] %s - %s\n",
			i+1,
			severity,
			location,
			diag.Message))

		// Add source and code if present, but keep it compact
		var details []string
		if diag.Source != "" {
			details = append(details, fmt.Sprintf("Source: %s", diag.Source))
		}
		if diag.Code != nil {
			details = append(details, fmt.Sprintf("Code: %v", diag.Code))
		}

		if len(details) > 0 {
			formattedDiag.WriteString(fmt.Sprintf("   %s\n", strings.Join(details, ", ")))
		}

		// Add code context
		if codeContext != "" {
			formattedDiag.WriteString(fmt.Sprintf("   > %s\n", codeContext))
		}

		formattedDiagnostics = append(formattedDiagnostics, formattedDiag.String())
	}

	return strings.Join(formattedDiagnostics, ""), nil
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
