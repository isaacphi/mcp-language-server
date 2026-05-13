package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDiagnosticsForFile retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string, contextLines int, showLineNumbers bool) (string, error) {
	// Override with environment variable if specified
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Convert the file path to URI format
	uri := protocol.DocumentUri("file://" + filePath)

	// Wait for the LSP to publish diagnostics for this URI (push mode).
	// 3s upper bound matches the previous hardcoded sleep; settle window
	// (150ms) absorbs follow-up republishes some servers send after a
	// project-wide rescan.
	client.WaitForDiagnostics(ctx, uri, 3*time.Second, 150*time.Millisecond)

	// Also attempt pull-mode (textDocument/diagnostic) for servers that
	// support it. Push-only servers (e.g. Civet) return -32601; that's
	// fine since the cache is already populated from publishDiagnostics.
	diagParams := protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}
	if _, err = client.Diagnostic(ctx, diagParams); err != nil {
		toolsLogger.Debug("Pull-mode diagnostic unavailable (server likely push-only): %v", err)
	}

	// Get diagnostics from the cache
	diagnostics := client.GetFileDiagnostics(uri)

	if len(diagnostics) == 0 {
		return "No diagnostics found for " + filePath, nil
	}

	// Format file header
	fileInfo := fmt.Sprintf("%s\nDiagnostics in File: %d\n",
		filePath,
		len(diagnostics),
	)

	// Create a summary of all the diagnostics
	var diagSummaries []string
	var diagLocations []protocol.Location

	for _, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("L%d:C%d",
			diag.Range.Start.Line+1,
			diag.Range.Start.Character+1)

		summary := fmt.Sprintf("%s at %s: %s",
			severity,
			location,
			diag.Message)

		// Add source and code if available
		if diag.Source != "" {
			summary += fmt.Sprintf(" (Source: %s", diag.Source)
			if diag.Code != nil {
				summary += fmt.Sprintf(", Code: %v", diag.Code)
			}
			summary += ")"
		} else if diag.Code != nil {
			summary += fmt.Sprintf(" (Code: %v)", diag.Code)
		}

		diagSummaries = append(diagSummaries, summary)

		// Create a location for this diagnostic to use with line ranges
		diagLocations = append(diagLocations, protocol.Location{
			URI:   uri,
			Range: diag.Range,
		})
	}

	// Format content with context
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fileInfo + "\nError reading file: " + err.Error(), nil
	}

	lines := strings.Split(string(fileContent), "\n")

	// Collect lines to display
	var linesToShow map[int]bool
	if contextLines > 0 {
		// Use GetLineRangesToDisplay for context
		linesToShow, err = GetLineRangesToDisplay(ctx, client, diagLocations, len(lines), contextLines)
		if err != nil {
			// If error, just show the diagnostic lines
			linesToShow = make(map[int]bool)
			for _, diag := range diagnostics {
				linesToShow[int(diag.Range.Start.Line)] = true
			}
		}
	} else {
		// Just show the diagnostic lines
		linesToShow = make(map[int]bool)
		for _, diag := range diagnostics {
			linesToShow[int(diag.Range.Start.Line)] = true
		}
	}

	// Convert to line ranges
	lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

	// Format with diagnostics summary in header
	result := fileInfo
	if len(diagSummaries) > 0 {
		result += strings.Join(diagSummaries, "\n") + "\n"
	}

	// Format the content with ranges
	if showLineNumbers {
		result += "\n" + FormatLinesWithRanges(lines, lineRanges)
	}

	return result, nil
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
