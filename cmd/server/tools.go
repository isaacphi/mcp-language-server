package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/metoro-io/mcp-golang"
)

type getDefinitionArgs struct {
	SymbolName string `json:"symbolName" jsonschema:"required,description=The exact name of the symbol (function, class or something else) to fetch."`
}

type applyTextEditArgs struct {
	Path    string `json:"path"`
	Start   string `json:"start"`
	End     string `json:"end"`
	NewText string `json:"newText"`
}

func (s *server) registerTools() error {

	err := s.mcpServer.RegisterTool(
		"read-definition",
		"Read the source code for a given symbol from the codebase",
		func(args getDefinitionArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.ReadDefinition(s.ctx, s.lspClient, args.SymbolName)
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
		"Apply a text edit to a file. Specify path, start line, end line, and new text. Line numbers are 1-based.",
		func(args applyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			start, err := parsePosition(args.Start)
			if err != nil {
				return nil, fmt.Errorf("invalid start position: %v", err)
			}

			end, err := parsePosition(args.End)
			if err != nil {
				return nil, fmt.Errorf("invalid end position: %v", err)
			}

			edit := protocol.WorkspaceEdit{
				Changes: map[protocol.DocumentUri][]protocol.TextEdit{
					protocol.DocumentUri(args.Path): {{
						Range: protocol.Range{
							Start: start,
							End:   end,
						},
						NewText: args.NewText,
					}},
				},
			}

			if err := tools.ApplyWorkspaceEdit(edit); err != nil {
				return nil, fmt.Errorf("failed to apply text edit: %v", err)
			}

			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent("Successfully applied text edit")), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	return nil
}

func parsePosition(pos string) (protocol.Position, error) {
	parts := strings.Split(pos, ":")
	if len(parts) != 1 && len(parts) != 2 {
		return protocol.Position{}, fmt.Errorf("position must be in format line[:column]")
	}

	line, err := strconv.Atoi(parts[0])
	if err != nil {
		return protocol.Position{}, fmt.Errorf("invalid line number: %v", err)
	}

	char := 1 // Default to column 1 if not specified
	if len(parts) == 2 {
		char, err = strconv.Atoi(parts[1])
		if err != nil {
			return protocol.Position{}, fmt.Errorf("invalid column number: %v", err)
		}
	}

	// Convert from 1-based to 0-based indexing
	return protocol.Position{
		Line:      uint32(line - 1),
		Character: uint32(char - 1),
	}, nil
}

