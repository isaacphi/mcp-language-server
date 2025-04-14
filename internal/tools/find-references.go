package tools

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
	// "github.com/davecgh/go-spew/spew" // Useful for debugging complex structs
)

// --- At the top of your tools package ---
var debugLogger *log.Logger

// ScopeIdentifier uniquely identifies a scope (function, method, etc.) in a file
type ScopeIdentifier struct {
	URI       protocol.DocumentUri
	StartLine uint32
	EndLine   uint32
	// Adding StartChar and EndChar might make it more unique if needed, but Line is usually enough
	// StartChar uint32
	// EndChar   uint32
}

// ReferencePosition represents a single reference position within a scope
type ReferencePosition struct {
	Line      uint32
	Character uint32
}

// ScopeInfo stores information about a code scope including its name and kind
type ScopeInfo struct {
	Name    string              // Name of the scope (from DocumentSymbol)
	Kind    protocol.SymbolKind // Kind of the symbol (from DocumentSymbol)
	HasKind bool                // Whether we have kind information (always true if found via symbol)
}

func init() {
	debugLogger = log.New(io.Discard, "DEBUG_FIND_REFS: ", log.LstdFlags|log.Lmicroseconds)

	enableDebug := os.Getenv("MCP_DEBUG_LOG") == "true"
	if enableDebug {
		logFilePath := "debug_find_refs.log"
		logFileHandle, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Fallback to stderr if file fails
			debugLogger.SetOutput(os.Stderr) // Change output to stderr
			debugLogger.Printf("!!! FAILED TO OPEN DEBUG LOG FILE '%s': %v - Logging to Stderr !!!\n", logFilePath, err)
		} else {
			debugLogger.SetOutput(logFileHandle) // Change output to the file
			// Optionally close the file handle gracefully on server shutdown
			debugLogger.Printf("--- Debug logging explicitly enabled to file: %s ---", logFilePath)
		}
	}
}

// Helper function to find the smallest DocumentSymbol containing the target position
// Returns the symbol and a boolean indicating if found.
func findSymbolContainingPosition(symbols []protocol.DocumentSymbolResult, targetPos protocol.Position, level int) (*protocol.DocumentSymbol, bool) {
	indent := strings.Repeat("  ", level)
	debugLogger.Printf("%sDEBUG: [%d] findSymbolContainingPosition called for TargetPos: L%d:C%d (0-based)\n", indent, level, targetPos.Line, targetPos.Character)

	var bestMatch *protocol.DocumentSymbol = nil

	for i, symResult := range symbols {
		debugLogger.Printf("%sDEBUG: [%d] Checking symbol #%d: Name='%s'\n", indent, level, i, symResult.GetName())

		ds, ok := symResult.(*protocol.DocumentSymbol)
		if !ok {
			debugLogger.Printf("%sDEBUG: [%d]   Skipping symbol '%s' - not *protocol.DocumentSymbol\n", indent, level, symResult.GetName())
			continue // Skip if it's not the expected type
		}

		symRange := ds.GetRange()
		debugLogger.Printf("%sDEBUG: [%d]   Symbol: '%s', Kind: %d, Range: L%d:C%d - L%d:C%d (0-based)\n",
			indent, level, ds.Name, ds.Kind,
			symRange.Start.Line, symRange.Start.Character,
			symRange.End.Line, symRange.End.Character)

		// Check if the symbol's range contains the target position
		posInLineRange := targetPos.Line >= symRange.Start.Line && targetPos.Line <= symRange.End.Line
		posInRange := false
		if posInLineRange {
			// Must be strictly *after* start on start line, and strictly *before* end on end line? No, LSP range includes boundaries.
			// Check: Not before start on start line AND not after end on end line.
			if targetPos.Line == symRange.Start.Line && targetPos.Character < symRange.Start.Character {
				// Before start char on start line - NO MATCH
				debugLogger.Printf("%sDEBUG: [%d]   RangeCheck: posInLineRange=true. Target char %d < Start char %d on Start Line %d.\n", indent, level, targetPos.Character, symRange.Start.Character, targetPos.Line)
			} else if targetPos.Line == symRange.End.Line && targetPos.Character > symRange.End.Character {
				// After end char on end line - NO MATCH
				debugLogger.Printf("%sDEBUG: [%d]   RangeCheck: posInLineRange=true. Target char %d > End char %d on End Line %d.\n", indent, level, targetPos.Character, symRange.End.Character, targetPos.Line)
			} else {
				posInRange = true
			}
		}
		debugLogger.Printf("%sDEBUG: [%d]   RangeCheck Result: posInLineRange=%v, posInRange=%v\n", indent, level, posInLineRange, posInRange) // Log the crucial check result

		if posInRange {
			debugLogger.Printf("%sDEBUG: [%d]   Position IS within '%s' range. Checking children...\n", indent, level, ds.Name)
			// This symbol contains the position. Check children for a more specific match.
			var childMatch *protocol.DocumentSymbol = nil
			var childFound bool = false
			if len(ds.Children) > 0 {
				childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
				for i := range ds.Children {
					childSymbols[i] = &ds.Children[i]
				}
				// Pass level + 1 for indentation
				childMatch, childFound = findSymbolContainingPosition(childSymbols, targetPos, level+1)
				debugLogger.Printf("%sDEBUG: [%d]   Recursive call for children of '%s' returned: found=%v, childMatch=%p\n", indent, level, ds.Name, childFound, childMatch)
				if childFound {
					debugLogger.Printf("%sDEBUG: [%d]     Child match name: '%s'\n", indent, level, childMatch.Name)
				}
			} else {
				debugLogger.Printf("%sDEBUG: [%d]   Symbol '%s' has no children.\n", indent, level, ds.Name)
			}

			if childFound {
				// A child is a more specific match
				debugLogger.Printf("%sDEBUG: [%d]   Updating bestMatch to child: '%s' (%p)\n", indent, level, childMatch.Name, childMatch)
				bestMatch = childMatch // Use the child match
			} else {
				// This symbol is the best match found so far *at or below this level*.
				// Compare its size with any existing bestMatch (which could be from a sibling branch's child).
				debugLogger.Printf("%sDEBUG: [%d]   No better child found for '%s'. Comparing with current bestMatch (%p).\n", indent, level, ds.Name, bestMatch)

				// Determine if this symbol (ds) is better (smaller) than the current bestMatch
				isBetter := bestMatch == nil ||
					// Smaller line range is better
					(symRange.End.Line-symRange.Start.Line < bestMatch.Range.End.Line-bestMatch.Range.Start.Line) ||
					// Same line range, smaller character range is better (more specific)
					((symRange.End.Line-symRange.Start.Line == bestMatch.Range.End.Line-bestMatch.Range.Start.Line) &&
						(symRange.End.Character-symRange.Start.Character < bestMatch.Range.End.Character-bestMatch.Range.Start.Character))

				if isBetter {
					debugLogger.Printf("%sDEBUG: [%d]   Current symbol '%s' IS better than bestMatch (%p). Updating bestMatch.\n", indent, level, ds.Name, bestMatch)
					bestMatch = ds // Update bestMatch to this symbol
				} else {
					debugLogger.Printf("%sDEBUG: [%d]   Current symbol '%s' is NOT better than bestMatch ('%s').\n", indent, level, ds.Name, bestMatch.Name)
				}
			}
		} else {
			debugLogger.Printf("%sDEBUG: [%d]   Position is NOT within '%s' range.\n", indent, level, ds.Name)
		}
		debugLogger.Printf("%sDEBUG: [%d] --- End Check for Symbol '%s' ---\n", indent, level, ds.Name)
	} // End loop through symbols at this level

	debugLogger.Printf("%sDEBUG: [%d] findSymbolContainingPosition returning: found=%v, bestMatch=%p\n", indent, level, bestMatch != nil, bestMatch)
	if bestMatch != nil {
		debugLogger.Printf("%sDEBUG: [%d]   Return Symbol Name: '%s'\n", indent, level, bestMatch.Name)
	}
	return bestMatch, bestMatch != nil
}

// Helper function to get text content for a specific range (implementation needed)
// This might use file reading or potentially a custom LSP request if available.
// For simplicity, we'll read the file content here. Could be optimized.
func getTextForRange(ctx context.Context, uri protocol.DocumentUri, fileContent []byte, targetRange protocol.Range) (string, error) {
	lines := strings.Split(string(fileContent), "\n") // Assumes LF endings for simplicity here

	startLine := int(targetRange.Start.Line)
	endLine := int(targetRange.End.Line)
	startChar := int(targetRange.Start.Character)
	endChar := int(targetRange.End.Character)

	if startLine < 0 || startLine >= len(lines) || endLine < 0 || endLine >= len(lines) || startLine > endLine {
		return "", fmt.Errorf("invalid range for file content: lines %d-%d (file has %d lines)", startLine+1, endLine+1, len(lines))
	}

	var sb strings.Builder

	if startLine == endLine {
		// Single line range
		line := lines[startLine]
		if startChar > len(line) {
			startChar = len(line)
		}
		if endChar > len(line) {
			endChar = len(line)
		}
		if startChar < 0 {
			startChar = 0
		}
		if endChar < 0 {
			endChar = 0
		}
		if startChar > endChar {
			startChar = endChar
		} // Ensure start <= end
		sb.WriteString(line[startChar:endChar])
	} else {
		// Multi-line range
		// Start line: from startChar to end
		firstLine := lines[startLine]
		if startChar > len(firstLine) {
			startChar = len(firstLine)
		}
		if startChar < 0 {
			startChar = 0
		}
		sb.WriteString(firstLine[startChar:])
		sb.WriteString("\n") // Add newline separator

		// Middle lines: entire lines
		for i := startLine + 1; i < endLine; i++ {
			sb.WriteString(lines[i])
			sb.WriteString("\n")
		}

		// End line: from beginning to endChar
		lastLine := lines[endLine]
		if endChar > len(lastLine) {
			endChar = len(lastLine)
		}
		if endChar < 0 {
			endChar = 0
		}
		sb.WriteString(lastLine[:endChar])
	}

	return sb.String(), nil
}

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string, showLineNumbers bool) (string, error) {
	// --- Stage 1: Find Symbol Definitions ---
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: symbolName})
	if err != nil {
		return "", fmt.Errorf("Failed to fetch symbol: %v", err)
	}
	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("Failed to parse results: %v", err)
	}

	processedLocations := make(map[protocol.Location]struct{})
	var uniqueLocations []protocol.Location
	for _, symbol := range results {
		if symbol.GetName() != symbolName {
			continue
		}
		loc := symbol.GetLocation()
		// Ensure loc is valid (sometimes workspace/symbol might return incomplete info)
		if loc.URI == "" || loc.Range.Start.Line == 0 && loc.Range.Start.Character == 0 && loc.Range.End.Line == 0 && loc.Range.End.Character == 0 {
			// debugLogger.Printf( "Warning: Skipping invalid location for symbol %s\n", symbolName)
			continue
		}
		if _, exists := processedLocations[loc]; !exists {
			processedLocations[loc] = struct{}{}
			uniqueLocations = append(uniqueLocations, loc)
		}
	}
	if len(uniqueLocations) == 0 {
		return fmt.Sprintf("Symbol definition not found for: %s", symbolName), nil
	}

	// --- Stage 2: Find All References ---
	var allFoundRefs []protocol.Location
	for _, loc := range uniqueLocations {
		refsParams := protocol.ReferenceParams{ /* ... as before ... */
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
				Position:     loc.Range.Start,
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: false},
		}
		refs, err := client.References(ctx, refsParams)
		if err != nil {
			// Log or report, but continue if possible
			debugLogger.Printf("Warning: Failed to get references for definition at %s:%d: %v\n",
				loc.URI, loc.Range.Start.Line+1, err)
			continue
		}
		allFoundRefs = append(allFoundRefs, refs...)
	}
	totalRefs := len(allFoundRefs)
	if totalRefs == 0 {
		return fmt.Sprintf("No references found for symbol: %s (definition found at %d location(s))", symbolName, len(uniqueLocations)), nil
	}

	// --- Stage 3: Group References by File and Scope ---
	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range allFoundRefs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}

	allReferences := []string{fmt.Sprintf("Symbol: %s (%d references in %d files)", symbolName, totalRefs, len(refsByFile))}

	filesProcessed := 0
	for uri, fileRefs := range refsByFile {
		filesProcessed++
		filePath := strings.TrimPrefix(string(uri), "file://")
		// Sort refs by position within the file
		sort.Slice(fileRefs, func(i, j int) bool { /* ... as before ... */
			if fileRefs[i].Range.Start.Line != fileRefs[j].Range.Start.Line {
				return fileRefs[i].Range.Start.Line < fileRefs[j].Range.Start.Line
			}
			return fileRefs[i].Range.Start.Character < fileRefs[j].Range.Start.Character
		})
		allReferences = append(allReferences, fmt.Sprintf("File: %s (%d references)", filePath, len(fileRefs)))

		// --- Sub-Stage 3a: Get Symbols and File Content Once Per File ---
		var docSymbols []protocol.DocumentSymbolResult
		symParams := protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}}
		symResult, symErr := client.DocumentSymbol(ctx, symParams)
		if symErr == nil {
			docSymbols, _ = symResult.Results()
			// Check if we got DocumentSymbol, not SymbolInformation
			if len(docSymbols) > 0 {
				if _, ok := docSymbols[0].(*protocol.DocumentSymbol); !ok {
					debugLogger.Printf("Warning: Received SymbolInformation instead of DocumentSymbol for %s, scope identification might be limited.\n", uri)
					docSymbols = nil // Treat as no symbols found for our purpose
				}
			}
		} else {
			debugLogger.Printf("Warning: Failed to get document symbols for %s: %v\n", uri, symErr)
		}

		// Read file content once for fetching scope text later
		fileContent, readErr := os.ReadFile(filePath)
		if readErr != nil {
			debugLogger.Printf("Warning: Failed to read file content for %s: %v. Scope text will be unavailable.\n", filePath, readErr)
			fileContent = nil // Mark content as unavailable
		}

		// --- Sub-Stage 3b: Group References by Symbol Scope ---
		scopeRefs := make(map[ScopeIdentifier][]ReferencePosition)
		scopeInfos := make(map[ScopeIdentifier]ScopeInfo)
		scopeTexts := make(map[ScopeIdentifier]string) // Store text based on symbol range

		for _, ref := range fileRefs {
			var containingSymbol *protocol.DocumentSymbol
			var foundSymbol bool

			// ** KEY CHANGE: Find the symbol containing the *reference position* **
			if len(docSymbols) > 0 {
				// Call the debugged function with initial level 0
				debugLogger.Printf("\n--- Searching for symbol containing reference at L%d:C%d (0-based Line %d) ---\n", ref.Range.Start.Line+1, ref.Range.Start.Character+1, ref.Range.Start.Line)
				containingSymbol, foundSymbol = findSymbolContainingPosition(docSymbols, ref.Range.Start, 0) // Start recursion level at 0
				debugLogger.Printf("--- Search complete for L%d:C%d. Found: %v ---\n\n", ref.Range.Start.Line+1, ref.Range.Start.Character+1, foundSymbol)
			}

			var scopeID ScopeIdentifier
			var scopeRange protocol.Range // The range used for fetching text

			if foundSymbol {
				// --- Case 1: Reference is within a known symbol ---
				scopeRange = containingSymbol.Range // Use the symbol's range
				scopeID = ScopeIdentifier{
					URI:       uri,
					StartLine: containingSymbol.Range.Start.Line,
					EndLine:   containingSymbol.Range.End.Line,
					// Optional: Add character info if needed for uniqueness:
					// StartChar: containingSymbol.Range.Start.Character,
					// EndChar:   containingSymbol.Range.End.Character,
				}

				// Store scope info only once per symbol
				if _, exists := scopeInfos[scopeID]; !exists {
					scopeInfos[scopeID] = ScopeInfo{
						Name:    containingSymbol.Name,
						Kind:    containingSymbol.Kind,
						HasKind: true, // We got it from a symbol
					}
					// Fetch and store text for this symbol's range
					if fileContent != nil {
						text, err := getTextForRange(ctx, uri, fileContent, scopeRange)
						if err == nil {
							scopeTexts[scopeID] = text
						} else {
							debugLogger.Printf("Warning: Failed to get text for symbol %s range (%d-%d): %v\n", containingSymbol.Name, scopeRange.Start.Line+1, scopeRange.End.Line+1, err)
							scopeTexts[scopeID] = fmt.Sprintf("Error fetching text for symbol '%s'", containingSymbol.Name)
						}
					} else {
						scopeTexts[scopeID] = "[File content unavailable]"
					}
				}

			} else {
				// --- Case 2: Reference is NOT within a known symbol (e.g., top-level, import, comment) ---
				// Fallback: Use context snippet approach
				contextLines := 5
				scopeText, scopeLoc, err := GetDefinitionWithContext(ctx, client, ref, contextLines)
				if err != nil {
					debugLogger.Printf("Warning: Could not get context for reference outside symbol at %s:%d: %v\n", ref.URI, ref.Range.Start.Line+1, err)
					// Create a dummy scopeID just for this reference if needed, or skip
					continue
				}

				scopeRange = scopeLoc.Range // Use the context range
				scopeID = ScopeIdentifier{  // Create ID based on context range
					URI:       uri,
					StartLine: scopeLoc.Range.Start.Line,
					EndLine:   scopeLoc.Range.End.Line,
				}

				// Store info for this fallback scope only once
				if _, exists := scopeInfos[scopeID]; !exists {
					scopeInfos[scopeID] = ScopeInfo{
						Name:    fmt.Sprintf("Context near L%d", ref.Range.Start.Line+1),
						Kind:    0, // Unknown kind
						HasKind: false,
					}
					scopeTexts[scopeID] = scopeText // Store the fetched context text
				}
			}

			// Add the reference position to the determined scope (symbol-based or context-based)
			position := ReferencePosition{
				Line:      ref.Range.Start.Line,
				Character: ref.Range.Start.Character,
			}
			scopeRefs[scopeID] = append(scopeRefs[scopeID], position)

		} // End loop through references in file

		// --- Stage 4: Format Output ---
		// Get the keys (scopeIDs) and sort them by starting line
		scopeIDs := make([]ScopeIdentifier, 0, len(scopeRefs))
		for id := range scopeRefs {
			scopeIDs = append(scopeIDs, id)
		}
		sort.Slice(scopeIDs, func(i, j int) bool { /* ... as before ... */
			return scopeIDs[i].StartLine < scopeIDs[j].StartLine
		})

		// Loop through sorted scopes and format output
		for _, scopeID := range scopeIDs {
			positions := scopeRefs[scopeID]
			scopeInfo := scopeInfos[scopeID]
			scopeText := scopeTexts[scopeID] // Get the stored text

			// Debug info (now reflects symbol finding)
			// debugInfo := fmt.Sprintf("DEBUG: Scope='%s', HasKind=%v, Kind=%d (L%d-%d)",
			// 	scopeInfo.Name, scopeInfo.HasKind, scopeInfo.Kind, scopeID.StartLine+1, scopeID.EndLine+1)
			// allReferences = append(allReferences, "  "+debugInfo)

			// Format scope header (using Kind if HasKind is true)
			var scopeHeader string
			if scopeInfo.HasKind {
				kindStr := utilities.GetSymbolKindString(scopeInfo.Kind)
				displayName := scopeInfo.Name
				if kindStr != "" && kindStr != "Unknown" {
					displayName = fmt.Sprintf("%s %s", kindStr, scopeInfo.Name)
				}
				scopeHeader = fmt.Sprintf("  %s (lines %d-%d, %d references)", displayName, scopeID.StartLine+1, scopeID.EndLine+1, len(positions))
			} else {
				scopeHeader = fmt.Sprintf("  Scope: %s (lines %d-%d, %d references)", scopeInfo.Name, scopeID.StartLine+1, scopeID.EndLine+1, len(positions))
			}
			allReferences = append(allReferences, scopeHeader)

			// Format reference positions (no changes)
			var positionStrs []string
			var highlightLineIndices []int // Relative to the start of the scopeText
			for _, pos := range positions {
				positionStrs = append(positionStrs, fmt.Sprintf("L%d:C%d", pos.Line+1, pos.Character+1))
				// Calculate highlight index relative to scope start
				highlightLineIndices = append(highlightLineIndices, int(pos.Line-scopeID.StartLine))
			}
			// ... (chunking logic as before) ...
			const chunkSize = 4
			for i := 0; i < len(positionStrs); i += chunkSize {
				end := i + chunkSize
				if end > len(positionStrs) {
					end = len(positionStrs)
				}
				positionChunk := positionStrs[i:end]
				allReferences = append(allReferences, fmt.Sprintf("    References: %s", strings.Join(positionChunk, ", ")))
			}

			// Format scope text (truncation, line numbers, highlighting)
			scopeLines := strings.Split(scopeText, "\n") // Use the stored text

			// --- Truncation Logic --- (needs adjustment for highlightLineIndices)
			finalScopeLines := scopeLines                 // Start with original lines
			finalHighlightIndices := highlightLineIndices // Start with original indices
			if len(scopeLines) > 50 {
				// ... (Existing truncation logic, BUT ensure it correctly maps original highlightLineIndices to the indices in the *truncated* output) ...

				// Simplified recalculation (can be improved for precision)
				importantLines := make(map[int]bool)
				for i := 0; i < 5 && i < len(scopeLines); i++ {
					importantLines[i] = true
				}
				for i := len(scopeLines) - 3; i < len(scopeLines) && i >= 0; i++ {
					importantLines[i] = true
				}
				for _, hlLine := range highlightLineIndices { // Use original indices here
					for offset := -2; offset <= 2; offset++ {
						lineIdx := hlLine + offset
						if lineIdx >= 0 && lineIdx < len(scopeLines) {
							importantLines[lineIdx] = true
						}
					}
				}

				var truncatedLines []string
				originalToTruncatedIndexMap := make(map[int]int)
				currentTruncatedIndex := 0
				inSkipSection := false
				lastShownIndex := -1

				for i := 0; i < len(scopeLines); i++ {
					if importantLines[i] {
						if inSkipSection {
							truncatedLines = append(truncatedLines, fmt.Sprintf("    ... %d lines skipped ...", i-lastShownIndex-1))
							currentTruncatedIndex++ // Account for the skip line
							inSkipSection = false
						}
						truncatedLines = append(truncatedLines, scopeLines[i])
						originalToTruncatedIndexMap[i] = currentTruncatedIndex // Map original index to truncated index
						currentTruncatedIndex++
						lastShownIndex = i
					} else if !inSkipSection && lastShownIndex >= 0 {
						inSkipSection = true
					}
				}
				if inSkipSection && lastShownIndex < len(scopeLines)-1 {
					skippedLines := len(scopeLines) - lastShownIndex - 1
					if skippedLines > 0 {
						truncatedLines = append(truncatedLines, fmt.Sprintf("    ... %d lines skipped ...", skippedLines))
					}
				}

				// Recalculate highlight indices based on the map
				newHighlightIndices := []int{}
				for _, origIdx := range highlightLineIndices {
					if truncatedIdx, ok := originalToTruncatedIndexMap[origIdx]; ok {
						newHighlightIndices = append(newHighlightIndices, truncatedIdx)
					}
				}

				finalScopeLines = truncatedLines            // Use the truncated lines for display
				finalHighlightIndices = newHighlightIndices // Use the new indices for highlighting

			} // End truncation

			// --- Line Numbering / Formatting ---
			var formattedScope strings.Builder
			lineNum := int(scopeID.StartLine) + 1 // Start numbering from original scope start

			for i, line := range finalScopeLines {
				isRef := false
				for _, hl := range finalHighlightIndices { // Use potentially recalculated indices
					if i == hl {
						isRef = true
						break
					}
				}

				if strings.Contains(line, "lines skipped") {
					// Handle skip marker line
					if showLineNumbers {
						var skipped int
						fmt.Sscanf(line, "    ... %d lines skipped ...", &skipped) // Ignore error, default skip is 1 line display adjust
						formattedScope.WriteString(line + "\n")
						lineNum += skipped // Adjust line number count
					} else {
						formattedScope.WriteString(line + "\n") // Show skip marker even without line nums
					}
				} else {
					// Handle regular code line
					if showLineNumbers {
						numStr := fmt.Sprintf("%d", lineNum)
						padding := strings.Repeat(" ", 5-len(numStr))
						marker := "|"
						if isRef {
							marker = ">"
						}
						formattedScope.WriteString(fmt.Sprintf("%s%s%s %s\n", padding, numStr, marker, line))
					} else {
						// Add simple marker even without line numbers
						marker := "  " // Indent non-ref lines
						if isRef {
							marker = "> "
						}
						formattedScope.WriteString(marker + line + "\n")
					}
					lineNum++ // Increment for the next actual code line
				}
			}

			// Add the formatted scope with indentation
			trimmedFormattedScope := strings.TrimRight(formattedScope.String(), " \n\t")
			allReferences = append(allReferences, "    "+strings.ReplaceAll(trimmedFormattedScope, "\n", "\n    "))

		} // End loop through scopes

		// Add blank line between files
		if filesProcessed < len(refsByFile) {
			allReferences = append(allReferences, "")
		}

	} // End loop through files

	return strings.Join(allReferences, "\n"), nil
}
