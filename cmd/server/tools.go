package main

import (
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/metoro-io/mcp-golang"
)

func (s *server) registerTools() error {

	err := s.mcpServer.RegisterTool(
		"read-definition",
		"Read the source code for a given symbol from the codebase.",
		func(args tools.ReadDefinitionArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.ReadDefinition(s.ctx, s.lspClient, args)
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
		"Apply multiple text edits to a file. WARNING: line numbers may change between calls to this tool.",
		func(args tools.ApplyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			response, err := tools.ApplyTextEdits(args)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply edits: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(response)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"find-references",
		"Find all references to a symbol in the codebase.",
		func(args tools.FindReferencesArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.FindReferences(s.ctx, s.lspClient, args)
			if err != nil {
				return nil, fmt.Errorf("Failed to find references: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	return nil
}