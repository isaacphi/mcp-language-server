package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// getLSPClient resolves the LSP client to use based on the optional id parameter
// If id is provided, uses that specific LSP instance
// If id is omitted, uses the default (currently selected) LSP instance
func (s *mcpServer) getLSPClient(id string) (*lsp.Client, error) {
	instance, err := s.lspManager.ResolveLSPInstance(id)
	if err != nil {
		return nil, err
	}
	return instance.Client, nil
}

func (s *mcpServer) registerTools() error {
	coreLogger.Debug("Registering MCP tools")

	// Register LSP management tools in config mode (but not legacy mode)
	if s.config.isConfigMode && !s.config.isLegacyMode {
		s.registerLSPManagementTools()
	}

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
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(applyTextEditTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

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

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing edit_file for file: %s", filePath)
		response, err := tools.ApplyTextEdits(context.Background(), client, filePath, edits)
		if err != nil {
			coreLogger.Error("Failed to apply edits: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to apply edits: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	readDefinitionTool := mcp.NewTool("definition",
		mcp.WithDescription("Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. Returns the complete implementation code where the symbol is defined."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The name of the symbol whose definition you want to find (e.g. 'mypackage.MyFunction', 'MyType.MyMethod')"),
		),
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(readDefinitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing definition for symbol: %s", symbolName)
		text, err := tools.ReadDefinition(context.Background(), client, symbolName)
		if err != nil {
			coreLogger.Error("Failed to get definition: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get definition: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	findReferencesTool := mcp.NewTool("references",
		mcp.WithDescription("Find all usages and references of a symbol throughout the codebase. Returns a list of all files and locations where the symbol appears."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The name of the symbol to search for (e.g. 'mypackage.MyFunction', 'MyType')"),
		),
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(findReferencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing references for symbol: %s", symbolName)
		text, err := tools.FindReferences(context.Background(), client, symbolName)
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
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(getDiagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

		contextLines := 5 // default value
		if contextLinesArg, ok := request.Params.Arguments["contextLines"].(int); ok {
			contextLines = contextLinesArg
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
		}

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing diagnostics for file: %s", filePath)
		text, err := tools.GetDiagnosticsForFile(context.Background(), client, filePath, contextLines, showLineNumbers)
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
	// s.mcp.AddTool(getCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 	// Extract arguments
	// 	filePath, ok := request.Params.Arguments["filePath"].(string)
	// 	if !ok {
	// 		return mcp.NewToolResultError("filePath must be a string"), nil
	// 	}
	//
	// 	coreLogger.Debug("Executing get_codelens for file: %s", filePath)
	// 	text, err := tools.GetCodeLens(context.Background(), s.lspClient, filePath)
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
	// s.mcp.AddTool(executeCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	// 	text, err := tools.ExecuteCodeLens(context.Background(), s.lspClient, filePath, index)
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
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(hoverTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

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

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing hover for file: %s line: %d column: %d", filePath, line, column)
		text, err := tools.GetHoverInfo(context.Background(), client, filePath, line, column)
		if err != nil {
			coreLogger.Error("Failed to get hover information: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get hover information: %v", err)), nil
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
		mcp.WithString("id",
			mcp.Description("LSP instance ID (optional, defaults to last started LSP in config mode)"),
		),
	)

	s.mcp.AddTool(renameSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		newName, ok := request.Params.Arguments["newName"].(string)
		if !ok {
			return mcp.NewToolResultError("newName must be a string"), nil
		}

		id, _ := request.Params.Arguments["id"].(string) // Optional

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

		// Get the appropriate LSP client
		client, err := s.getLSPClient(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get LSP client: %v", err)), nil
		}

		coreLogger.Debug("Executing rename_symbol for file: %s line: %d column: %d newName: %s", filePath, line, column, newName)
		text, err := tools.RenameSymbol(context.Background(), client, filePath, line, column, newName)
		if err != nil {
			coreLogger.Error("Failed to rename symbol: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to rename symbol: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	coreLogger.Info("Successfully registered all MCP tools")
	return nil
}

func (s *mcpServer) registerLSPManagementTools() {
	coreLogger.Debug("Registering LSP management tools")

	// In session mode, only register lsp_list (not lsp_start, lsp_stop, lsp_save, lsp_load)
	if !s.config.isSessionMode {
		// lsp_start tool
		lspStartTool := mcp.NewTool("lsp_start",
			mcp.WithDescription("Start a new LSP instance for a specific language and workspace."),
			mcp.WithString("workspace",
				mcp.Required(),
				mcp.Description("Path to the workspace directory for this LSP instance"),
			),
			mcp.WithString("language",
				mcp.Required(),
				mcp.Description("Language identifier (must match a language defined in the config file, e.g., 'go', 'python', 'rust')"),
			),
		)

		s.mcp.AddTool(lspStartTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			workspace, ok := request.Params.Arguments["workspace"].(string)
			if !ok {
				return mcp.NewToolResultError("workspace must be a string"), nil
			}

			language, ok := request.Params.Arguments["language"].(string)
			if !ok {
				return mcp.NewToolResultError("language must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_start for language: %s, workspace: %s", language, workspace)
			id, err := s.lspManager.StartLSP(context.Background(), workspace, language)
			if err != nil {
				coreLogger.Error("Failed to start LSP: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to start LSP: %v", err)), nil
			}

			result := fmt.Sprintf("Started LSP instance for %s\nID: %s\nWorkspace: %s", language, id, workspace)
			return mcp.NewToolResultText(result), nil
		})

		// lsp_stop tool
		lspStopTool := mcp.NewTool("lsp_stop",
			mcp.WithDescription("Stop a running LSP instance by its ID."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("The ID of the LSP instance to stop"),
			),
		)

		s.mcp.AddTool(lspStopTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, ok := request.Params.Arguments["id"].(string)
			if !ok {
				return mcp.NewToolResultError("id must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_stop for ID: %s", id)
			err := s.lspManager.StopLSP(id)
			if err != nil {
				coreLogger.Error("Failed to stop LSP: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to stop LSP: %v", err)), nil
			}

			result := fmt.Sprintf("Stopped LSP instance: %s", id)
			return mcp.NewToolResultText(result), nil
		})

		// lsp_save tool
		lspSaveTool := mcp.NewTool("lsp_save",
			mcp.WithDescription("Save the current running LSP instances to a session file."),
			mcp.WithString("filepath",
				mcp.Required(),
				mcp.Description("Path where the session file should be saved"),
			),
		)

		s.mcp.AddTool(lspSaveTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filepath, ok := request.Params.Arguments["filepath"].(string)
			if !ok {
				return mcp.NewToolResultError("filepath must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_save to file: %s", filepath)
			err := s.lspManager.SaveSession(filepath)
			if err != nil {
				coreLogger.Error("Failed to save session: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to save session: %v", err)), nil
			}

			instances := s.lspManager.ListLSPs()
			result := fmt.Sprintf("Saved %d LSP instance(s) to %s", len(instances), filepath)
			return mcp.NewToolResultText(result), nil
		})

		// lsp_load tool
		lspLoadTool := mcp.NewTool("lsp_load",
			mcp.WithDescription("Load LSP instances from a session file. Stops LSPs not in the file and starts LSPs that are in the file but not running."),
			mcp.WithString("filepath",
				mcp.Required(),
				mcp.Description("Path to the session file to load"),
			),
		)

		s.mcp.AddTool(lspLoadTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filepath, ok := request.Params.Arguments["filepath"].(string)
			if !ok {
				return mcp.NewToolResultError("filepath must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_load from file: %s", filepath)
			err := s.lspManager.LoadSession(context.Background(), filepath)
			if err != nil {
				coreLogger.Error("Failed to load session: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to load session: %v", err)), nil
			}

			instances := s.lspManager.ListLSPs()
			result := fmt.Sprintf("Loaded session from %s\nCurrently running: %d LSP instance(s)", filepath, len(instances))
			return mcp.NewToolResultText(result), nil
		})

		// lsp_select tool
		lspSelectTool := mcp.NewTool("lsp_select",
			mcp.WithDescription("Select a different LSP instance as the default for commands that don't specify an ID."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("The ID of the LSP instance to set as default"),
			),
		)

		s.mcp.AddTool(lspSelectTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, ok := request.Params.Arguments["id"].(string)
			if !ok {
				return mcp.NewToolResultError("id must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_select for ID: %s", id)
			err := s.lspManager.SelectLSP(id)
			if err != nil {
				coreLogger.Error("Failed to select LSP: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to select LSP: %v", err)), nil
			}

			// Get the instance details for confirmation
			instance, _ := s.lspManager.GetLSP(id)
			result := fmt.Sprintf("Selected LSP as default:\nID: %s\nLanguage: %s\nWorkspace: %s", id, instance.Language, instance.WorkspacePath)
			return mcp.NewToolResultText(result), nil
		})

		// lsp_languages tool - show available languages from config (only in config mode, not session mode)
		lspLanguagesTool := mcp.NewTool("lsp_languages",
			mcp.WithDescription("List all available language identifiers defined in the configuration file."),
		)

		s.mcp.AddTool(lspLanguagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			coreLogger.Debug("Executing lsp_languages")

			if s.lspManager.config == nil {
				return mcp.NewToolResultError("No config loaded"), nil
			}

			languages := make([]string, 0, len(s.lspManager.config.Defaults))
			for lang := range s.lspManager.config.Defaults {
				languages = append(languages, lang)
			}

			if len(languages) == 0 {
				return mcp.NewToolResultText("No languages defined in configuration"), nil
			}

			// Sort alphabetically for consistent output
			sort.Strings(languages)

			result := "Available Languages:\n\n"
			for _, lang := range languages {
				defaultConfig := s.lspManager.config.Defaults[lang]
				result += fmt.Sprintf("- %s (command: %s)\n", lang, defaultConfig.Command)
			}

			return mcp.NewToolResultText(result), nil
		})
	}

	// lsp_list tool - always registered, even in session mode
	lspListTool := mcp.NewTool("lsp_list",
		mcp.WithDescription("List all currently running LSP instances with their IDs, languages, and workspaces."),
	)

	s.mcp.AddTool(lspListTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		coreLogger.Debug("Executing lsp_list")
		instances := s.lspManager.ListLSPs()

		if len(instances) == 0 {
			return mcp.NewToolResultText("No LSP instances running"), nil
		}

		result := "Running LSP Instances:\n\n"
		for _, inst := range instances {
			result += fmt.Sprintf("ID: %s\nLanguage: %s\nWorkspace: %s\nStatus: %s\n\n",
				inst["id"], inst["language"], inst["workspace"], inst["status"])
		}

		return mcp.NewToolResultText(result), nil
	})

	coreLogger.Info("Successfully registered LSP management tools")
}
