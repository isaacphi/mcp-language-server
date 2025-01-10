package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/metoro-io/mcp-golang"
)

type getDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The exact name of the symbol to fetch. Method names must be fully specified e.g. MyClass.MyMethod"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"default=false,description=If true, adds line numbers to the output"`
}

type applyTextEditArgs struct {
	FilePath     string `json:"filePath" jsonschema:"required,description=Full path to file"`
	StartLineNum string `json:"startLineNum" jsonschema:"required,description=1 based start line number"`
	EndLineNum   string `json:"endLineNum" jsonschema:"required,description=1 based end line number"`
	NewText      string `json:"newText" jsonschema:"required,description=Text to insert"`
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
		"Apply a text edit to a file.",
		func(args applyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			rng, err := getPosition(args.StartLineNum, args.EndLineNum, args.FilePath)
			if err != nil {
				return nil, fmt.Errorf("invalid position: %v", err)
			}

			edit := protocol.WorkspaceEdit{
				Changes: map[protocol.DocumentUri][]protocol.TextEdit{
					protocol.DocumentUri(args.FilePath): {{
						Range: rng, NewText: args.NewText,
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

func getPosition(startPos, endPos, filePath string) (protocol.Range, error) {
	startLine, err := strconv.Atoi(startPos)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("invalid line number: %v", err)
	}

	endLine, err := strconv.Atoi(endPos)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("invalid line number: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Split(string(content), "\n")

	// Convert to 0 based index
	rng := protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startLine - 1),
			Character: 0,
		},
		End: protocol.Position{
			Line:      uint32(endLine - 1),
			Character: uint32(len(lines[endLine-1])),
		},
	}

	return rng, nil
}

