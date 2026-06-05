package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// openFileError marks a collectReferencesAt failure that originated from being
// unable to open the symbol's file (stale index, deleted/moved file,
// permissions). Such a failure is recoverable per match; a references RPC
// failure is not and is returned as a plain error.
type openFileError struct{ err error }

func (e *openFileError) Error() string { return e.err.Error() }
func (e *openFileError) Unwrap() error { return e.err }

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	// Get context lines from environment variable
	contextLines := 5
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	// First get the symbol location like ReadDefinition does
	results, err := workspaceSymbols(ctx, client, symbolName)
	if err != nil {
		return "", err
	}

	var allReferences []string
	// hardErr records the most recent genuine references-RPC failure (server
	// crash, timeout, protocol error). It is surfaced only if no references
	// were collected at all: a single flaky match must neither discard
	// references already resolved from other matches nor be silently reported
	// as "no references found". The main loop and the qualified-name fallback
	// share this policy via collect.
	var hardErr error
	collect := func(loc protocol.Location, name string) {
		formatted, err := collectReferencesAt(ctx, client, loc, contextLines)
		if err != nil {
			var ofe *openFileError
			if errors.As(err, &ofe) {
				// A single un-openable file (stale index, deleted/moved file,
				// permissions) must not discard references resolvable from the
				// remaining matches.
				toolsLogger.Error("Error opening file for %s: %v", name, err)
				return
			}
			// A genuine references RPC failure is a real error, not an empty
			// result. Remember it but keep going so references from other
			// matches survive; it is surfaced below only if nothing resolved.
			toolsLogger.Error("Error getting references for %s: %v", name, err)
			hardErr = fmt.Errorf("failed to get references for %s: %w", name, err)
			return
		}
		allReferences = append(allReferences, formatted...)
	}

	for _, symbol := range results {
		// Handle different matching strategies based on the search term
		if strings.Contains(symbolName, ".") {
			// For qualified names like "Type.Method", check for various matches
			parts := strings.Split(symbolName, ".")
			methodName := parts[len(parts)-1]

			// Try matching the unqualified method name for languages that don't use qualified names in symbols
			if symbol.GetName() != symbolName && symbol.GetName() != methodName {
				continue
			}
		} else if symbol.GetName() != symbolName {
			// For unqualified names, exact match only
			continue
		}

		collect(symbol.GetLocation(), symbol.GetName())
	}

	// Fallback for qualified names that workspace/symbol could not match
	// directly. Some servers (e.g. jdtls) only index types, so member
	// symbols like "Class.method" never appear in workspace/symbol results.
	if len(allReferences) == 0 && strings.Contains(symbolName, ".") {
		resolved, err := resolveQualifiedSymbol(ctx, client, symbolName)
		if err != nil {
			return "", err
		}
		for _, sym := range resolved {
			collect(sym.Location, sym.Name)
		}
	}

	if len(allReferences) == 0 {
		// Surface a genuine RPC failure instead of masking it as an empty
		// result; otherwise report that nothing was found.
		if hardErr != nil {
			return "", hardErr
		}
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	return strings.Join(allReferences, "\n"), nil
}

// collectReferencesAt runs textDocument/references for the symbol at loc and
// formats the results grouped by file.
func collectReferencesAt(ctx context.Context, client *lsp.Client, loc protocol.Location, contextLines int) ([]string, error) {
	// Use LSP references request with correct params structure
	refsParams := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: loc.URI,
			},
			Position: loc.Range.Start,
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: false,
		},
	}
	// File is likely to be opened already, but may not be. A failure here is
	// reported as an openFileError so callers can skip this match rather than
	// treating it as a hard references failure.
	openPath, err := uriToPath(loc.URI)
	if err != nil {
		return nil, &openFileError{err: err}
	}
	if err := client.OpenFile(ctx, openPath); err != nil {
		return nil, &openFileError{err: fmt.Errorf("error opening file: %v", err)}
	}
	refs, err := client.References(ctx, refsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %v", err)
	}

	var allReferences []string

	// Group references by file
	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range refs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}

	// Get sorted list of URIs
	uris := make([]string, 0, len(refsByFile))
	for uri := range refsByFile {
		uris = append(uris, string(uri))
	}
	sort.Strings(uris)

	// Process each file's references in sorted order
	for _, uriStr := range uris {
		uri := protocol.DocumentUri(uriStr)
		fileRefs := refsByFile[uri]
		// Use uriToPath rather than trimming the "file://" prefix: it
		// percent-decodes the path (e.g. "%20" -> space), so os.ReadFile
		// finds files whose paths contain encoded characters, and the displayed
		// path stays consistent with definition.go. A non-file URI (e.g. a
		// jdt:// reference into a JAR) cannot be read from disk, so skip it
		// rather than panicking.
		filePath, err := uriToPath(uri)
		if err != nil {
			toolsLogger.Error("Skipping references in non-file URI %s: %v", uri, err)
			continue
		}

		// Format file header
		fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n",
			filePath,
			len(fileRefs),
		)

		// Format locations with context
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			// Log error but continue with other files
			allReferences = append(allReferences, fileInfo+"\nError reading file: "+err.Error())
			continue
		}

		lines := strings.Split(string(fileContent), "\n")

		// Track reference locations for header display
		var locStrings []string
		for _, ref := range fileRefs {
			locStr := fmt.Sprintf("L%d:C%d",
				ref.Range.Start.Line+1,
				ref.Range.Start.Character+1)
			locStrings = append(locStrings, locStr)
		}

		// Collect lines to display using the utility function
		linesToShow, err := GetLineRangesToDisplay(ctx, client, fileRefs, len(lines), contextLines)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		// Convert to line ranges using the utility function
		lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

		// Format with locations in header
		formattedOutput := fileInfo
		if len(locStrings) > 0 {
			formattedOutput += "At: " + strings.Join(locStrings, ", ") + "\n"
		}

		// Format the content with ranges
		formattedOutput += "\n" + FormatLinesWithRanges(lines, lineRanges)
		allReferences = append(allReferences, formattedOutput)
	}

	return allReferences, nil
}
