package main

import (
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/metoro-io/mcp-golang"
)

type ReadDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The exact name of the symbol to fetch. Method names must be fully specified e.g. MyClass.MyMethod"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=If true, adds line numbers to the output"`
}

type FindReferencesArgs struct {
	SymbolName      string `json:"symbolName"`
	ShowLineNumbers bool   `json:"showLineNumbers"`
}

type ApplyTextEditArgs struct {
	FilePath string           `json:"filePath"`
	Edits    []tools.TextEdit `json:"edits"`
}

type GetDiagnosticsArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file to get diagnostics for"`
}

func (s *server) registerTools() error {

	err := s.mcpServer.RegisterTool(
		"apply-text-edit",
		"Apply multiple text edits to a file.",
		func(args ApplyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			response, err := tools.ApplyTextEdits(args.FilePath, args.Edits)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply edits: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(response)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"read-definition",
		"Read the source code for a given symbol from the codebase.",
		func(args ReadDefinitionArgs) (*mcp_golang.ToolResponse, error) {
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
		"find-references",
		"Find references to a symbol in the codebase.",
		func(args FindReferencesArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.FindReferences(s.ctx, s.lspClient, args.SymbolName, args.ShowLineNumbers)
			if err != nil {
				return nil, fmt.Errorf("Failed to find references: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"get-diagnostics",
		"Get diagnostic information for a specific file from the language server.",
		func(args GetDiagnosticsArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.GetDiagnosticsForFile(s.ctx, s.lspClient, args.FilePath)
			if err != nil {
				return nil, fmt.Errorf("Failed to get diagnostics: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	return nil
}
