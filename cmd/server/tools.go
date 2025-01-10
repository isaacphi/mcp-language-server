package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/metoro-io/mcp-golang"
)

type getDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The exact name of the symbol to fetch. Method names must be fully specified e.g. MyClass.MyMethod"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"default=false,description=If true, adds line numbers to the output"`
}

type TextEdit struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	NewText   string `json:"newText"`
}

type ApplyTextEditArgs struct {
	FilePath string     `json:"filePath"`
	Edits    []TextEdit `json:"edits"`
}

func (s *server) registerTools() error {

	err := s.mcpServer.RegisterTool(
		"read-definition",
		"Read the source code for a given symbol from the codebase",
		func(args getDefinitionArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.ReadDefinition(s.ctx, s.lspClient, args.SymbolName, args.ShowLineNumbers)
			if err != nil {
				return nil, fmt.Errorf("Failed to get definition: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"apply-text-edit",
		"Apply multiple text edits to a file.",
		func(args ApplyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			// Sort edits by line number in descending order to process from bottom to top
			// This way line numbers don't shift under us as we make edits
			sort.Slice(args.Edits, func(i, j int) bool {
				return args.Edits[i].StartLine > args.Edits[j].StartLine
			})

			var textEdits []protocol.TextEdit
			for _, edit := range args.Edits {
				rng, err := getRange(edit.StartLine, edit.EndLine, args.FilePath)
				if err != nil {
					return nil, fmt.Errorf("invalid position: %v", err)
				}

				textEdits = append(textEdits, protocol.TextEdit{
					Range:   rng,
					NewText: edit.NewText,
				})
			}

			edit := protocol.WorkspaceEdit{
				Changes: map[protocol.DocumentUri][]protocol.TextEdit{
					protocol.DocumentUri(args.FilePath): textEdits,
				},
			}

			if err := tools.ApplyWorkspaceEdit(edit); err != nil {
				return nil, fmt.Errorf("failed to apply text edits: %v", err)
			}

			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent("Successfully applied text edits")), nil
		})

	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	return nil
}

func getRange(startLine, endLine int, filePath string) (protocol.Range, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Split into lines while preserving line endings
	lines := strings.SplitAfter(string(content), "\n")

	if startLine < 1 || startLine > len(lines) {
		return protocol.Range{}, fmt.Errorf("start line out of range")
	}

	// For end of file insertions, point to the end of the last line
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Convert to 0-based index
	startIdx := startLine - 1
	endIdx := endLine - 1

	// Get the true line length, excluding the trailing newline if present
	lineLen := len(lines[endIdx])
	if strings.HasSuffix(lines[endIdx], "\n") {
		lineLen--
	}

	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0,
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(lineLen),
		},
	}, nil
}
