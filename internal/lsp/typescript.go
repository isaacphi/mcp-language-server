package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// initializeTypescriptLanguageServer initializes the TypeScript language server
// with specific configurations and opens all TypeScript files in the workspace.
func initializeTypescriptLanguageServer(ctx context.Context, client *Client, workspaceDir string) error {
	lspLogger.Info("Initializing TypeScript language server with workspace: %s", workspaceDir)

	// First, open all TypeScript files in the workspace
	if err := openAllTypeScriptFiles(ctx, client, workspaceDir); err != nil {
		return fmt.Errorf("failed to open TypeScript files: %w", err)
	}

	return nil
}

// openAllTypeScriptFiles finds and opens all TypeScript files in the workspace
func openAllTypeScriptFiles(ctx context.Context, client *Client, workspaceDir string) error {
	lspLogger.Info("Opening all TypeScript files in workspace: %s", workspaceDir)

	// Track count of opened files for logging
	fileCount := 0

	// Walk the workspace directory
	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip node_modules, .git, and other common directories to avoid processing too many files
			basename := filepath.Base(path)
			if basename == "node_modules" || basename == ".git" || strings.HasPrefix(basename, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is a TypeScript file
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			if err := client.OpenFile(ctx, path); err != nil {
				lspLogger.Warn("Failed to open TypeScript file %s: %v", path, err)
				return nil // Continue with other files even if one fails
			}
			fileCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking workspace directory: %w", err)
	}

	lspLogger.Info("Opened %d TypeScript files", fileCount)
	return nil
}
