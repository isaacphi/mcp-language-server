package tools

import (
	"context"
	"fmt"
	"os"
	"sort" // Needed for sorting definitions if multiple found
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
	// "github.com/davecgh/go-spew/spew" // Useful for debugging complex structs
)

// DefinitionInfo holds the refined information for a single definition
type DefinitionInfo struct {
	SymbolName     string
	SymbolKind     protocol.SymbolKind
	HasKind        bool
	FilePath       string
	Range          protocol.Range // The precise range of the definition symbol
	DefinitionText string
	// ContainerName string // Can be added if needed by traversing DocumentSymbol parents
}

// ReadDefinition intelligently finds and extracts the definition text for a symbol.
// It prioritizes using documentSymbol for precise range finding.
func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string, showLineNumbers bool) (string, error) {
	debugLogger.Printf("--- GetDefinition called for symbol: %s ---\n", symbolName)

	// --- Stage 1: Find *potential* symbol locations ---
	// We use workspace/symbol first to get *any* location (definition or usage) to start the process.
	wsSymbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: symbolName})
	if err != nil {
		return "", fmt.Errorf("failed to fetch workspace symbols for '%s': %w", symbolName, err)
	}
	wsSymbols, err := wsSymbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse workspace symbol results for '%s': %w", symbolName, err)
	}

	var initialLocations []protocol.Location
	processedURIs := make(map[protocol.DocumentUri]bool) // Avoid hitting definition/documentSymbol multiple times for the same file if symbol has multiple entries there

	debugLogger.Printf("Found %d potential workspace symbols for '%s'\n", len(wsSymbols), symbolName)
	for _, symbol := range wsSymbols {
		// Strict name match is crucial here
		if symbol.GetName() != symbolName {
			continue
		}
		loc := symbol.GetLocation()
		// Skip invalid locations or already processed files
		if loc.URI == "" || processedURIs[loc.URI] {
			continue
		}

		// We only need one good starting point per file.
		// Using the first match is usually sufficient.
		initialLocations = append(initialLocations, loc)
		processedURIs[loc.URI] = true
		debugLogger.Printf("  -> Found potential initial location in %s at L%d\n", loc.URI, loc.Range.Start.Line+1)
		// Optimization: If we only need *one* definition, we could potentially break here.
		// But let's find all distinct definitions for completeness.
	}

	if len(initialLocations) == 0 {
		debugLogger.Printf("No initial locations found via workspace/symbol matching name '%s' exactly.\n", symbolName)
		return fmt.Sprintf("Symbol '%s' not found in workspace.", symbolName), nil
	}

	// --- Stage 2 & 3: Refine Location & Find Precise Scope ---
	var foundDefinitions []DefinitionInfo
	processedDefinitionRanges := make(map[string]bool) // Key: "URI:StartLine:StartChar"

	for _, startLoc := range initialLocations {
		debugLogger.Printf("\n--- Processing initial location: %s:%d ---\n", startLoc.URI, startLoc.Range.Start.Line+1)

		// --- Stage 2: Use textDocument/definition for canonical location ---
		defParams := protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: startLoc.URI},
				Position:     startLoc.Range.Start, // Use the start of the workspace symbol's range
			},
		}
		defResult, err := client.Definition(ctx, defParams)
		if err != nil {
			debugLogger.Printf("Warning: textDocument/definition call failed for %s:%d: %v. Skipping this path.\n", startLoc.URI, startLoc.Range.Start.Line+1, err)
			continue // Try next initial location if any
		}

		// --- Stage 3: Process each definition location found ---
		var definitionLocations []protocol.Location

		// --- Unpack the result ---
		// Helper function to extract locations from the potentially nested value
		extractLocations := func(value interface{}) ([]protocol.Location, bool) {
			var extracted []protocol.Location
			switch v := value.(type) {
			case nil:
				debugLogger.Printf("  Inner definition value is nil.\n")
				return nil, true // Successfully processed null, result is empty list
			case protocol.Location:
				extracted = []protocol.Location{v}
				debugLogger.Printf("  Inner definition resolved to Single Location: %s L%d:%d\n", v.URI, v.Range.Start.Line+1, v.Range.Start.Character+1)
				return extracted, true
			case []protocol.Location:
				if len(v) == 0 {
					debugLogger.Printf("  Inner definition resolved to an EMPTY slice of Locations.\n")
				} else {
					debugLogger.Printf("  Inner definition resolved to Multiple Locations (%d)\n", len(v))
					// Optionally log the first few locations
					for i := 0; i < len(v) && i < 3; i++ {
						debugLogger.Printf("    Loc %d: %s L%d:%d\n", i, v[i].URI, v[i].Range.Start.Line+1, v[i].Range.Start.Character+1)
					}
				}
				extracted = v
				return extracted, true
			case []protocol.LocationLink:
				if len(v) == 0 {
					debugLogger.Printf("  Inner definition resolved to an EMPTY slice of LocationLinks.\n")
					extracted = []protocol.Location{} // Initialize empty slice
				} else {
					debugLogger.Printf("  Inner definition resolved to LocationLinks (%d), extracting targets...\n", len(v))
					extracted = make([]protocol.Location, 0, len(v)) // Initialize slice
					for linkIdx, link := range v {
						targetRange := link.TargetSelectionRange
						zeroRange := protocol.Range{}
						if targetRange == zeroRange || (targetRange.Start.Line == 0 && targetRange.Start.Character == 0 && targetRange.End.Line == 0 && targetRange.End.Character == 0) {
							debugLogger.Printf("    Link %d: TargetSelectionRange is zero/empty, falling back to TargetRange.\n", linkIdx)
							targetRange = link.TargetRange
						}

						if link.TargetURI == "" {
							debugLogger.Printf("    Link %d: Skipping because TargetURI is empty.\n", linkIdx)
							continue
						}

						if targetRange.Start.Line > targetRange.End.Line || (targetRange.Start.Line == targetRange.End.Line && targetRange.Start.Character > targetRange.End.Character) {
							debugLogger.Printf("    Link %d: Skipping Link Target '%s' due to invalid range: L%d:%d - L%d:%d\n",
								linkIdx, link.TargetURI, targetRange.Start.Line+1, targetRange.Start.Character+1, targetRange.End.Line+1, targetRange.End.Character+1)
							continue
						}

						extractedLoc := protocol.Location{
							URI:   link.TargetURI,
							Range: targetRange,
						}
						extracted = append(extracted, extractedLoc)
						debugLogger.Printf("    Link %d: Extracted Target: %s L%d:%d - L%d:%d\n",
							linkIdx,
							extractedLoc.URI,
							extractedLoc.Range.Start.Line+1, extractedLoc.Range.Start.Character+1,
							extractedLoc.Range.End.Line+1, extractedLoc.Range.End.Character+1)
					}
					if len(extracted) == 0 {
						debugLogger.Printf("  Finished processing LocationLinks, but none resulted in a valid Location.\n")
					}
				}
				return extracted, true // Return the (potentially empty) extracted list

			default:
				// This case means the *inner* value was unexpected
				debugLogger.Printf("Error: Inner definition value contained an unexpected type (%T).\n", value)
				return nil, false // Indicate failure to extract
			}
		}

		// --- Main Type Switch on defResult.Value ---
		var ok bool
		// ** Adjust the type name 'protocol.Or_Definition' if it's different in your library! **
		switch v := defResult.Value.(type) {
		case protocol.Or_Definition: // Check for the nested "Or" type first
			debugLogger.Printf("  Definition result Value is type %T, extracting inner value...\n", v)
			// Recursively (or directly) check the inner value
			definitionLocations, ok = extractLocations(v.Value)
			if !ok {
				// The inner extraction failed
				debugLogger.Printf("Error: Failed to extract locations from nested %T. Skipping this path.\n", v)
				continue
			}
		default:
			// Try extracting directly if it wasn't the nested type
			debugLogger.Printf("  Definition result Value is type %T, attempting direct extraction...\n", v)
			definitionLocations, ok = extractLocations(v) // v here is defResult.Value
			if !ok {
				// Direct extraction failed (e.g., default case in extractLocations hit)
				debugLogger.Printf("Error: Direct extraction failed for type %T. Skipping this path.\n", v)
				continue
			}
		}

		// Now, check if we successfully extracted any locations after handling potential nesting
		if len(definitionLocations) == 0 {
			debugLogger.Printf("Warning: No valid definition locations were extracted after processing the response for %s:%d. Skipping to next initial location (if any).\n", startLoc.URI, startLoc.Range.Start.Line+1)
			continue // Try next initial location
		}

		// --- Proceed with the rest of the loop using the definitionLocations slice ---
		processedAnyInThisBatch := false // Track if we successfully process at least one defLoc from this batch
		for _, defLoc := range definitionLocations {
			// ... (rest of the code: checking defLoc, processedRanges, getting symbols, reading file, getting text, appending results)
			// ... (No changes needed in the rest of the loop below this point) ...

			// Check if defLoc itself is valid (sometimes servers return empty locations)
			if defLoc.URI == "" {
				debugLogger.Printf("  -> Skipping an empty/invalid location received from definition result.\n")
				continue
			}

			defLocKey := fmt.Sprintf("%s:%d:%d", defLoc.URI, defLoc.Range.Start.Line, defLoc.Range.Start.Character)
			if processedDefinitionRanges[defLocKey] {
				debugLogger.Printf("  -> Skipping already processed definition location: %s\n", defLocKey)
				continue // Avoid processing the exact same definition multiple times
			}
			// Mark immediately *before* trying file IO etc.
			processedDefinitionRanges[defLocKey] = true
			debugLogger.Printf("  -> Processing definition location: %s L%d:%d - L%d:%d\n", defLoc.URI, defLoc.Range.Start.Line+1, defLoc.Range.Start.Character+1, defLoc.Range.End.Line+1, defLoc.Range.End.Character+1)
			filePath := strings.TrimPrefix(string(defLoc.URI), "file://")

			// --- Stage 3a: Get Document Symbols for the definition's file ---
			var preciseRange protocol.Range = defLoc.Range // Default to definition result range
			var defSymbolKind protocol.SymbolKind = 0
			var hasKind bool = false

			docSymParams := protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: defLoc.URI}}
			docSymResult, docSymErr := client.DocumentSymbol(ctx, docSymParams)

			if docSymErr == nil {
				docSymbols, _ := docSymResult.Results()
				if len(docSymbols) > 0 {
					if _, ok := docSymbols[0].(*protocol.DocumentSymbol); ok {
						debugLogger.Printf("  -> Searching document symbols in %s for position L%d:%d\n", defLoc.URI, defLoc.Range.Start.Line+1, defLoc.Range.Start.Character+1)
						containingSymbol, foundSymbol := findSymbolContainingPosition(docSymbols, defLoc.Range.Start, 0)

						if foundSymbol {
							if containingSymbol.Name == symbolName {
								debugLogger.Printf("    --> Found matching DocumentSymbol: '%s' (%s), Range: L%d:%d - L%d:%d\n",
									containingSymbol.Name, utilities.GetSymbolKindString(containingSymbol.Kind),
									containingSymbol.Range.Start.Line+1, containingSymbol.Range.Start.Character+1,
									containingSymbol.Range.End.Line+1, containingSymbol.Range.End.Character+1)
								preciseRange = containingSymbol.Range
								defSymbolKind = containingSymbol.Kind
								hasKind = true
							} else {
								debugLogger.Printf("    --> Found containing DocumentSymbol '%s' but name mismatch (expected '%s'). Using its range: L%d:%d - L%d:%d\n",
									containingSymbol.Name, symbolName,
									containingSymbol.Range.Start.Line+1, containingSymbol.Range.Start.Character+1,
									containingSymbol.Range.End.Line+1, containingSymbol.Range.End.Character+1)
								preciseRange = containingSymbol.Range
								defSymbolKind = containingSymbol.Kind
								hasKind = true
							}
						} else {
							debugLogger.Printf("    --> No specific DocumentSymbol found containing L%d:%d. Using range from textDocument/definition.\n", defLoc.Range.Start.Line+1, defLoc.Range.Start.Character+1)
						}
					} else {
						debugLogger.Printf("  -> Received SymbolInformation instead of DocumentSymbol for %s. Using range from textDocument/definition.\n", defLoc.URI)
					}
				} else {
					debugLogger.Printf("  -> No document symbols returned for %s. Using range from textDocument/definition.\n", defLoc.URI)
				}
			} else {
				debugLogger.Printf("Warning: Failed to get document symbols for %s: %v. Using range from textDocument/definition.\n", defLoc.URI, docSymErr)
			}

			// --- Stage 4: Fetch Definition Text using the determined range ---
			debugLogger.Printf("    Attempting to read file: %s\n", filePath)
			fileContent, readErr := os.ReadFile(filePath)
			if readErr != nil {
				debugLogger.Printf("Error: Failed to read file content for %s: %v. Skipping this definition location.\n", filePath, readErr)
				continue // Skip this defLoc
			}
			debugLogger.Printf("    Successfully read %d bytes from %s\n", len(fileContent), filePath)

			debugLogger.Printf("    Attempting to extract text for range: L%d:%d - L%d:%d\n", preciseRange.Start.Line+1, preciseRange.Start.Character+1, preciseRange.End.Line+1, preciseRange.End.Character+1)
			definitionText, textErr := getTextForRange(ctx, defLoc.URI, fileContent, preciseRange)
			if textErr != nil {
				debugLogger.Printf("Error: Failed to extract text for range L%d-L%d in %s: %v. Skipping this definition location.\n", preciseRange.Start.Line+1, preciseRange.End.Line+1, filePath, textErr)
				continue // Skip this defLoc
			}
			debugLogger.Printf("    Successfully extracted text (length %d).\n", len(definitionText))

			// --- Append to Results ---
			debugLogger.Printf("    --> SUCCESS: Appending definition to results.\n")
			foundDefinitions = append(foundDefinitions, DefinitionInfo{
				SymbolName:     symbolName, // Use the requested name
				SymbolKind:     defSymbolKind,
				HasKind:        hasKind,
				FilePath:       filePath,
				Range:          preciseRange,
				DefinitionText: definitionText,
			})
			processedAnyInThisBatch = true // Mark success for this batch

		} // End loop through definitionLocations

		if !processedAnyInThisBatch {
			debugLogger.Printf("  -> Finished processing all extracted locations for initial location %s:%d, but none resulted in a successful definition append.\n", startLoc.URI, startLoc.Range.Start.Line+1)
		}

	} // End loop through initialLocations

	if len(foundDefinitions) == 0 {
		debugLogger.Printf("--- No definitions found after refining locations for '%s' ---\n", symbolName)
		// Provide a more informative message if possible
		if len(initialLocations) > 0 {
			return fmt.Sprintf("Symbol '%s' found in workspace, but could not resolve its precise definition location.", symbolName), nil
		}
		// Fallback to the original message if even workspace symbols failed
		return fmt.Sprintf("Symbol '%s' not found.", symbolName), nil
	}

	// --- Stage 5: Format Output ---
	// Sort definitions by file path then start line for consistent output
	sort.Slice(foundDefinitions, func(i, j int) bool {
		if foundDefinitions[i].FilePath != foundDefinitions[j].FilePath {
			return foundDefinitions[i].FilePath < foundDefinitions[j].FilePath
		}
		return foundDefinitions[i].Range.Start.Line < foundDefinitions[j].Range.Start.Line
	})

	var output strings.Builder
	for i, defInfo := range foundDefinitions {
		if i > 0 {
			output.WriteString("\n---\n\n") // Separator for multiple definitions
		}

		// Header
		output.WriteString(fmt.Sprintf("Symbol: %s\n", defInfo.SymbolName))
		if defInfo.HasKind {
			kindStr := utilities.GetSymbolKindString(defInfo.SymbolKind)
			if kindStr != "" && kindStr != "Unknown" {
				output.WriteString(fmt.Sprintf("Kind: %s\n", kindStr))
			}
		}
		output.WriteString(fmt.Sprintf("File: %s\n", defInfo.FilePath))
		output.WriteString(fmt.Sprintf("Location: Lines %d-%d\n",
			defInfo.Range.Start.Line+1,
			defInfo.Range.End.Line+1))
		output.WriteString("\n") // Separator before code

		// Code
		codeBlock := defInfo.DefinitionText
		if showLineNumbers {
			codeBlock = addLineNumbers(codeBlock, int(defInfo.Range.Start.Line)+1)
		}
		output.WriteString(codeBlock)
	}

	debugLogger.Printf("--- GetDefinition finished for '%s', found %d definition(s) ---\n", symbolName, len(foundDefinitions))
	return output.String(), nil
}
