package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetInactiveRegions retrieves inactive preprocessor regions for a specific file using
// clangd's textDocument/inactiveRegions notification. The file is opened first, then
// we wait briefly for clangd to publish inactive regions and read them from the client cache.
func GetInactiveRegions(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %v", err)
	}

	if err := client.OpenFile(ctx, absPath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("%s\nError reading file: %v", absPath, err), nil
	}
	lines := strings.Split(string(content), "\n")

	// Allow clangd time to analyze the file and send textDocument/inactiveRegions.
	time.Sleep(3 * time.Second)

	uri := protocol.URIFromPath(absPath)
	regions := client.GetInactiveRegions(uri)

	if len(regions) == 0 {
		return fmt.Sprintf("No inactive regions found for %s", absPath), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s\nInactive regions: %d\n", absPath, len(regions)))

	for i, r := range regions {
		startLine := int(r.Start.Line) + 1
		startCol := int(r.Start.Character) + 1
		endLine := int(r.End.Line) + 1
		endCol := int(r.End.Character) + 1

		b.WriteString(fmt.Sprintf("Region %d: L%d:C%d - L%d:C%d\n", i+1, startLine, startCol, endLine, endCol))

		// Show the first line of the region as context if available.
		if startLine-1 >= 0 && startLine-1 < len(lines) {
			lineText := strings.TrimSpace(lines[startLine-1])
			if lineText != "" {
				b.WriteString(fmt.Sprintf("  Line %d: %s\n", startLine, lineText))
			}
		}
	}

	return strings.TrimSuffix(b.String(), "\n"), nil
}
