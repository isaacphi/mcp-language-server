package main

import (
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	mcp_golang "github.com/metoro-io/mcp-golang"
)

type ReadDefinitionArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The name of the symbol whose definition you want to find (e.g. 'mypackage.MyFunction', 'MyType.MyMethod')"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=Include line numbers in the returned source code"`
}

type FindReferencesArgs struct {
	SymbolName      string `json:"symbolName" jsonschema:"required,description=The name of the symbol to search for (e.g. 'mypackage.MyFunction', 'MyType')"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=Include line numbers when showing where the symbol is used"`
}

type ApplyTextEditArgs struct {
	FilePath string           `json:"filePath"`
	Edits    []tools.TextEdit `json:"edits"`
}

type GetDiagnosticsArgs struct {
	FilePath        string `json:"filePath" jsonschema:"required,description=The path to the file to get diagnostics for"`
	IncludeContext  bool   `json:"includeContext" jsonschema:"default=false,description=Include additional context for each diagnostic. Prefer false."`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=If true, adds line numbers to the output"`
}

type GetCodeLensArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file to get code lens information for"`
}

type ExecuteCodeLensArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file containing the code lens to execute"`
	Index    int    `json:"index" jsonschema:"required,description=The index of the code lens to execute (from get_codelens output), 1 indexed"`
}

type RenameSymbolArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file containing the symbol to rename"`
	Line     int    `json:"line" jsonschema:"required,description=The line number (1-indexed) where the symbol appears"`
	Column   int    `json:"column" jsonschema:"required,description=The column number (1-indexed) where the symbol appears"`
	NewName  string `json:"newName" jsonschema:"required,description=The new name for the symbol"`
}

type HoverArgs struct {
	FilePath string `json:"filePath" jsonschema:"required,description=The path to the file containing the symbol to get hover information for"`
	Line     int    `json:"line" jsonschema:"required,description=The line number (1-indexed) where the symbol appears"`
	Column   int    `json:"column" jsonschema:"required,description=The column number (1-indexed) where the symbol appears"`
}

type DocumentSymbolsArgs struct {
	FilePath        string `json:"filePath" jsonschema:"required,description=The path to the file to list symbols for"`
	ShowLineNumbers bool   `json:"showLineNumbers" jsonschema:"required,default=true,description=Include line numbers in the output"`
}

func (s *server) registerTools() error {
	err := s.mcpServer.RegisterTool(
		"apply_text_edit",
		"Apply multiple text edits to a file.",
		func(args ApplyTextEditArgs) (*mcp_golang.ToolResponse, error) {
			response, err := tools.ApplyTextEdits(s.ctx, s.lspClient, args.FilePath, args.Edits)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply edits: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(response)), nil
		})
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"read_definition",
		"Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. Returns the complete implementation code where the symbol is defined.",
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
		"find_references",
		"Find all usages and references of a symbol throughout the codebase. Returns a list of all files and locations where the symbol appears.",
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
		"get_diagnostics",
		"Get diagnostic information for a specific file from the language server.",
		func(args GetDiagnosticsArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.GetDiagnosticsForFile(s.ctx, s.lspClient, args.FilePath, args.IncludeContext, args.ShowLineNumbers)
			if err != nil {
				return nil, fmt.Errorf("Failed to get diagnostics: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"get_codelens",
		"Get code lens hints for a given file from the language server.",
		func(args GetCodeLensArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.GetCodeLens(s.ctx, s.lspClient, args.FilePath)
			if err != nil {
				return nil, fmt.Errorf("Failed to get code lens: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"execute_codelens",
		"Execute a code lens command for a given file and lens index.",
		func(args ExecuteCodeLensArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.ExecuteCodeLens(s.ctx, s.lspClient, args.FilePath, args.Index)
			if err != nil {
				return nil, fmt.Errorf("Failed to execute code lens: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"rename_symbol",
		"Rename a symbol (variable, function, class, etc.) and all its references across files.",
		func(args RenameSymbolArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.RenameSymbol(s.ctx, s.lspClient, args.FilePath, args.Line, args.Column, args.NewName)
			if err != nil {
				return nil, fmt.Errorf("Failed to rename symbol: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"hover",
		"Get hover information (type, documentation) for a symbol at the specified position.",
		func(args HoverArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.GetHoverInfo(s.ctx, s.lspClient, args.FilePath, args.Line, args.Column)
			if err != nil {
				return nil, fmt.Errorf("Failed to get hover information: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	err = s.mcpServer.RegisterTool(
		"document_symbols",
		"List all symbols (functions, methods, classes, etc.) in a document in a hierarchical structure.",
		func(args DocumentSymbolsArgs) (*mcp_golang.ToolResponse, error) {
			text, err := tools.GetDocumentSymbols(s.ctx, s.lspClient, args.FilePath, args.ShowLineNumbers)
			if err != nil {
				return nil, fmt.Errorf("Failed to get document symbols: %v", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(text)), nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register tool: %v", err)
	}

	return nil
}
