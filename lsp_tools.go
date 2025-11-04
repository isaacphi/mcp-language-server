package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerLSPManagementTools registers tools for managing LSP instances
func (s *mcpServer) registerLSPManagementTools() {
	coreLogger.Debug("Registering LSP management tools")

	// In session mode, only register lsp_list and lsp_select (not lsp_start, lsp_stop, lsp_save, lsp_load)
	// These tools are available in unbounded mode only
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

		s.mcpServer.AddTool(lspStartTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			workspace, ok := request.Params.Arguments["workspace"].(string)
			if !ok {
				return mcp.NewToolResultError("workspace must be a string"), nil
			}

			language, ok := request.Params.Arguments["language"].(string)
			if !ok {
				return mcp.NewToolResultError("language must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_start for language: %s, workspace: %s", language, workspace)
			id, err := s.lspManager.StartLSP(workspace, language, false)
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

		s.mcpServer.AddTool(lspStopTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		s.mcpServer.AddTool(lspSaveTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		s.mcpServer.AddTool(lspLoadTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filepath, ok := request.Params.Arguments["filepath"].(string)
			if !ok {
				return mcp.NewToolResultError("filepath must be a string"), nil
			}

			coreLogger.Debug("Executing lsp_load from file: %s", filepath)
			err := s.lspManager.LoadSession(filepath)
			if err != nil {
				coreLogger.Error("Failed to load session: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to load session: %v", err)), nil
			}

			instances := s.lspManager.ListLSPs()
			result := fmt.Sprintf("Loaded session from %s\nCurrently running: %d LSP instance(s)", filepath, len(instances))
			return mcp.NewToolResultText(result), nil
		})

		// lsp_languages tool - show available languages from config (only in unbounded mode, not session mode)
		lspLanguagesTool := mcp.NewTool("lsp_languages",
			mcp.WithDescription("List all available language identifiers defined in the configuration file."),
		)

		s.mcpServer.AddTool(lspLanguagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// lsp_select tool - registered in all modes (free and session)
	lspSelectTool := mcp.NewTool("lsp_select",
		mcp.WithDescription("Select a different LSP instance as the default for commands that don't specify an ID."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("The ID of the LSP instance to set as default"),
		),
	)

	s.mcpServer.AddTool(lspSelectTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// lsp_list tool - always registered, even in session mode
	lspListTool := mcp.NewTool("lsp_list",
		mcp.WithDescription("List all currently running LSP instances with their IDs, languages, and workspaces."),
	)

	s.mcpServer.AddTool(lspListTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
