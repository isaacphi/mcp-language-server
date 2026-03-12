package main

import (
	"context"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *mcpServer) registerTools() error {
	coreLogger.Debug("Registering MCP tools")

	applyTextEditTool := mcp.NewTool("edit_file",
		mcp.WithDescription("Apply multiple text edits to a file."),
		mcp.WithArray("edits",
			mcp.Required(),
			mcp.Description("List of edits to apply"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"startLine": map[string]any{
						"type":        "number",
						"description": "Start line to replace, inclusive, one-indexed",
					},
					"endLine": map[string]any{
						"type":        "number",
						"description": "End line to replace, inclusive, one-indexed",
					},
					"newText": map[string]any{
						"type":        "string",
						"description": "Replacement text. Replace with the new text. Leave blank to remove lines.",
					},
				},
				"required": []string{"startLine", "endLine"},
			}),
		),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the file to edit"),
		),
	)

	s.mcpServer.AddTool(applyTextEditTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		// Extract edits array
		editsArg, ok := request.Params.Arguments["edits"]
		if !ok {
			return mcp.NewToolResultError("edits is required"), nil
		}

		// Type assert and convert the edits
		editsArray, ok := editsArg.([]any)
		if !ok {
			return mcp.NewToolResultError("edits must be an array"), nil
		}

		var edits []tools.TextEdit
		for _, editItem := range editsArray {
			editMap, ok := editItem.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("each edit must be an object"), nil
			}

			startLine, ok := editMap["startLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("startLine must be a number"), nil
			}

			endLine, ok := editMap["endLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("endLine must be a number"), nil
			}

			newText, _ := editMap["newText"].(string) // newText can be empty

			edits = append(edits, tools.TextEdit{
				StartLine: int(startLine),
				EndLine:   int(endLine),
				NewText:   newText,
			})
		}

		coreLogger.Debug("Executing edit_file for file: %s", filePath)
		response, err := tools.ApplyTextEdits(s.ctx, s.lspClient, filePath, edits)
		if err != nil {
			coreLogger.Error("Failed to apply edits: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to apply edits: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	readDefinitionTool := mcp.NewTool("definition",
		mcp.WithDescription("Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. Returns the complete implementation code where the symbol is defined. For clangd (C/C++), prefer providing filePath, line, and character so the definition is resolved at that position; workspace/symbol often returns nothing until the project is indexed."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The name of the symbol whose definition you want to find (e.g. 'mypackage.MyFunction', 'MyType.MyMethod')"),
		),
		mcp.WithString("filePath",
			mcp.Description("Optional: path to the file where the symbol is used or declared. Preferred for clangd. If provided with line and character, definition is resolved at that position (avoids empty results when the file is not indexed)."),
		),
		mcp.WithNumber("line",
			mcp.Description("Optional: 1-based line number of the symbol. Use with filePath and character for position-based definition lookup."),
		),
		mcp.WithNumber("character",
			mcp.Description("Optional: 1-based character/column of the symbol. Use with filePath and line for position-based definition lookup."),
		),
	)

	s.mcpServer.AddTool(readDefinitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		var position *tools.DefPosition
		if fp, hasPath := request.Params.Arguments["filePath"].(string); hasPath && fp != "" {
			line, _ := request.Params.Arguments["line"].(float64)
			char, _ := request.Params.Arguments["character"].(float64)
			if line >= 1 {
				position = &tools.DefPosition{
					FilePath:  fp,
					Line:      int(line),
					Character: int(char),
				}
				if position.Character < 1 {
					position.Character = 1
				}
			}
		}

		coreLogger.Debug("Executing definition for symbol: %s", symbolName)
		text, err := tools.ReadDefinition(s.ctx, s.lspClient, symbolName, position)
		if err != nil {
			coreLogger.Error("Failed to get definition: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get definition: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	findReferencesTool := mcp.NewTool("references",
		mcp.WithDescription("Find all usages and references of a symbol throughout the codebase. Returns a list of all files and locations where the symbol appears. For clangd (C/C++), prefer providing filePath, line, and character so references are found at that position; workspace search often returns nothing until the project is indexed."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The name of the symbol to search for (e.g. 'mypackage.MyFunction', 'MyType')"),
		),
		mcp.WithString("filePath",
			mcp.Description("Optional: path to the file where the symbol appears. Preferred for clangd. If provided with line and character, references are found at that position (avoids empty results when the file is not indexed)."),
		),
		mcp.WithNumber("line",
			mcp.Description("Optional: 1-based line number of the symbol. Use with filePath and character for position-based reference search."),
		),
		mcp.WithNumber("character",
			mcp.Description("Optional: 1-based character/column of the symbol. Use with filePath and line for position-based reference search."),
		),
	)

	s.mcpServer.AddTool(findReferencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		var position *tools.RefPosition
		if fp, hasPath := request.Params.Arguments["filePath"].(string); hasPath && fp != "" {
			line, _ := request.Params.Arguments["line"].(float64)
			char, _ := request.Params.Arguments["character"].(float64)
			if line >= 1 {
				position = &tools.RefPosition{
					FilePath:  fp,
					Line:      int(line),
					Character: int(char),
				}
				if position.Character < 1 {
					position.Character = 1
				}
			}
		}

		coreLogger.Debug("Executing references for symbol: %s", symbolName)
		text, err := tools.FindReferences(s.ctx, s.lspClient, symbolName, position)
		if err != nil {
			coreLogger.Error("Failed to find references: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	getDiagnosticsTool := mcp.NewTool("diagnostics",
		mcp.WithDescription("Get diagnostic information for a specific file from the language server."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file to get diagnostics for"),
		),
		mcp.WithBoolean("contextLines",
			mcp.Description("Lines to include around each diagnostic."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("showLineNumbers",
			mcp.Description("If true, adds line numbers to the output"),
			mcp.DefaultBool(true),
		),
	)

	s.mcpServer.AddTool(getDiagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		contextLines := 5 // default value
		if contextLinesArg, ok := request.Params.Arguments["contextLines"].(int); ok {
			contextLines = contextLinesArg
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
		}

		coreLogger.Debug("Executing diagnostics for file: %s", filePath)
		text, err := tools.GetDiagnosticsForFile(s.ctx, s.lspClient, filePath, contextLines, showLineNumbers)
		if err != nil {
			coreLogger.Error("Failed to get diagnostics: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get diagnostics: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	// Uncomment to add codelens tools
	//
	// getCodeLensTool := mcp.NewTool("get_codelens",
	// 	mcp.WithDescription("Get code lens hints for a given file from the language server."),
	// 	mcp.WithString("filePath",
	// 		mcp.Required(),
	// 		mcp.Description("The path to the file to get code lens information for"),
	// 	),
	// )
	//
	// s.mcpServer.AddTool(getCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 	// Extract arguments
	// 	filePath, ok := request.Params.Arguments["filePath"].(string)
	// 	if !ok {
	// 		return mcp.NewToolResultError("filePath must be a string"), nil
	// 	}
	//
	// 	coreLogger.Debug("Executing get_codelens for file: %s", filePath)
	// 	text, err := tools.GetCodeLens(s.ctx, s.lspClient, filePath)
	// 	if err != nil {
	// 		coreLogger.Error("Failed to get code lens: %v", err)
	// 		return mcp.NewToolResultError(fmt.Sprintf("failed to get code lens: %v", err)), nil
	// 	}
	// 	return mcp.NewToolResultText(text), nil
	// })
	//
	// executeCodeLensTool := mcp.NewTool("execute_codelens",
	// 	mcp.WithDescription("Execute a code lens command for a given file and lens index."),
	// 	mcp.WithString("filePath",
	// 		mcp.Required(),
	// 		mcp.Description("The path to the file containing the code lens to execute"),
	// 	),
	// 	mcp.WithNumber("index",
	// 		mcp.Required(),
	// 		mcp.Description("The index of the code lens to execute (from get_codelens output), 1 indexed"),
	// 	),
	// )
	//
	// s.mcpServer.AddTool(executeCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 	// Extract arguments
	// 	filePath, ok := request.Params.Arguments["filePath"].(string)
	// 	if !ok {
	// 		return mcp.NewToolResultError("filePath must be a string"), nil
	// 	}
	//
	// 	// Handle both float64 and int for index due to JSON parsing
	// 	var index int
	// 	switch v := request.Params.Arguments["index"].(type) {
	// 	case float64:
	// 		index = int(v)
	// 	case int:
	// 		index = v
	// 	default:
	// 		return mcp.NewToolResultError("index must be a number"), nil
	// 	}
	//
	// 	coreLogger.Debug("Executing execute_codelens for file: %s index: %d", filePath, index)
	// 	text, err := tools.ExecuteCodeLens(s.ctx, s.lspClient, filePath, index)
	// 	if err != nil {
	// 		coreLogger.Error("Failed to execute code lens: %v", err)
	// 		return mcp.NewToolResultError(fmt.Sprintf("failed to execute code lens: %v", err)), nil
	// 	}
	// 	return mcp.NewToolResultText(text), nil
	// })

	hoverTool := mcp.NewTool("hover",
		mcp.WithDescription("Get hover information (type, documentation) for a symbol at the specified position."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file to get hover information for"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("The line number where the hover is requested (1-indexed)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("The column number where the hover is requested (1-indexed)"),
		),
	)

	s.mcpServer.AddTool(hoverTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return mcp.NewToolResultError("line must be a number"), nil
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return mcp.NewToolResultError("column must be a number"), nil
		}

		coreLogger.Debug("Executing hover for file: %s line: %d column: %d", filePath, line, column)
		text, err := tools.GetHoverInfo(s.ctx, s.lspClient, filePath, line, column)
		if err != nil {
			coreLogger.Error("Failed to get hover information: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get hover information: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	getCallHierarchyTool := mcp.NewTool("get_call_hierarchy",
		mcp.WithDescription("Get the full incoming call hierarchy at the given position: who calls this symbol, and who calls those callers, etc., with cycle detection. The file is opened automatically. Use position-based API (filePath, line, column). For clangd (C/C++), position-based lookup is required."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the file containing the symbol"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("Line number where the symbol is located (1-indexed)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("Column number where the symbol is located (1-indexed)"),
		),
	)

	s.mcpServer.AddTool(getCallHierarchyTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return mcp.NewToolResultError("line must be a number"), nil
		}
		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return mcp.NewToolResultError("column must be a number"), nil
		}

		coreLogger.Debug("Executing get_call_hierarchy for file: %s line: %d column: %d", filePath, line, column)
		text, err := tools.GetFullCallHierarchy(s.ctx, s.lspClient, filePath, line, column)
		if err != nil {
			coreLogger.Error("Failed to get call hierarchy: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get call hierarchy: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	documentSymbolTool := mcp.NewTool("document_symbol",
		mcp.WithDescription("Get the document outline (symbols) for a file: functions, classes, methods, variables, etc. The file is opened automatically. Returns a hierarchical outline with name, kind, and range for each symbol."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the file to get document symbols (outline) for"),
		),
	)

	s.mcpServer.AddTool(documentSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		coreLogger.Debug("Executing document_symbol for file: %s", filePath)
		text, err := tools.DocumentSymbolOutline(s.ctx, s.lspClient, filePath)
		if err != nil {
			coreLogger.Error("Failed to get document symbols: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get document symbols: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	renameSymbolTool := mcp.NewTool("rename_symbol",
		mcp.WithDescription("Rename a symbol (variable, function, class, etc.) at the specified position and update all references throughout the codebase."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file containing the symbol to rename"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("The line number where the symbol is located (1-indexed)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("The column number where the symbol is located (1-indexed)"),
		),
		mcp.WithString("newName",
			mcp.Required(),
			mcp.Description("The new name for the symbol"),
		),
	)

	s.mcpServer.AddTool(renameSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		newName, ok := request.Params.Arguments["newName"].(string)
		if !ok {
			return mcp.NewToolResultError("newName must be a string"), nil
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return mcp.NewToolResultError("line must be a number"), nil
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return mcp.NewToolResultError("column must be a number"), nil
		}

		coreLogger.Debug("Executing rename_symbol for file: %s line: %d column: %d newName: %s", filePath, line, column, newName)
		text, err := tools.RenameSymbol(s.ctx, s.lspClient, filePath, line, column, newName)
		if err != nil {
			coreLogger.Error("Failed to rename symbol: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to rename symbol: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	workspaceSymbolTool := mcp.NewTool("workspace_symbol",
		mcp.WithDescription("Search for project-wide symbols matching a query string (LSP workspace/symbol). Returns a list of symbols with name, kind, and location. For best results with clangd (C/C++), open relevant files first so the project is indexed."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query string to filter symbols by (e.g. function name, type name). Can be empty to request all symbols."),
		),
	)

	s.mcpServer.AddTool(workspaceSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, ok := request.Params.Arguments["query"].(string)
		if !ok {
			return mcp.NewToolResultError("query must be a string"), nil
		}

		coreLogger.Debug("Executing workspace_symbol for query: %s", query)
		text, err := tools.WorkspaceSymbol(s.ctx, s.lspClient, query)
		if err != nil {
			coreLogger.Error("Failed to get workspace symbols: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get workspace symbols: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	clangdInactiveRegionsTool := mcp.NewTool("clangd_inactive_regions",
		mcp.WithDescription("Show inactive preprocessor regions (e.g. #if 0 blocks) for a C/C++ file using clangd's textDocument/inactiveRegions extension."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the C/C++ source file to inspect"),
		),
	)

	s.mcpServer.AddTool(clangdInactiveRegionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		coreLogger.Debug("Executing clangd_inactive_regions for file: %s", filePath)
		text, err := tools.GetInactiveRegions(s.ctx, s.lspClient, filePath)
		if err != nil {
			coreLogger.Error("Failed to get inactive regions: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get inactive regions: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	clangdASTTool := mcp.NewTool("clangd_ast",
		mcp.WithDescription("Dump the clangd AST for a C/C++ source range using the textDocument/ast extension."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the C/C++ source file to inspect"),
		),
		mcp.WithNumber("startLine",
			mcp.Required(),
			mcp.Description("1-based start line of the range to fetch the AST for"),
		),
		mcp.WithNumber("startColumn",
			mcp.Required(),
			mcp.Description("1-based start column of the range to fetch the AST for"),
		),
		mcp.WithNumber("endLine",
			mcp.Required(),
			mcp.Description("1-based end line of the range to fetch the AST for"),
		),
		mcp.WithNumber("endColumn",
			mcp.Required(),
			mcp.Description("1-based end column of the range to fetch the AST for"),
		),
	)

	s.mcpServer.AddTool(clangdASTTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		getInt := func(name string) (int, error) {
			raw, ok := request.Params.Arguments[name]
			if !ok {
				return 0, fmt.Errorf("%s is required", name)
			}
			v, ok := raw.(float64)
			if !ok {
				return 0, fmt.Errorf("%s must be a number", name)
			}
			return int(v), nil
		}

		startLine, err := getInt("startLine")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startColumn, err := getInt("startColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endLine, err := getInt("endLine")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endColumn, err := getInt("endColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		coreLogger.Debug("Executing clangd_ast for file: %s range: L%d:C%d - L%d:C%d", filePath, startLine, startColumn, endLine, endColumn)
		text, err := tools.GetAST(s.ctx, s.lspClient, filePath, startLine, startColumn, endLine, endColumn)
		if err != nil {
			coreLogger.Error("Failed to get AST: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get AST: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	coreLogger.Info("Successfully registered all MCP tools")
	return nil
}
