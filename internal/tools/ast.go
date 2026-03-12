package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ASTParams is the request payload for clangd's textDocument/ast extension.
type ASTParams struct {
	Range        protocol.Range                 `json:"range"`
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
}

// ASTNode describes a single AST node in clangd's textDocument/ast response.
type ASTNode struct {
	Children []ASTNode       `json:"children,omitempty"`
	Range    *protocol.Range `json:"range,omitempty"`
	Arcana   string          `json:"arcana,omitempty"`
	Detail   string          `json:"detail,omitempty"`
	Kind     string          `json:"kind"`
	Role     string          `json:"role"`
}

// GetAST retrieves and pretty-prints the clangd AST for a specific source range
// using the textDocument/ast extension. The file is opened first to ensure the
// document is known to the server.
//
// All line/column parameters are 1-based and inclusive. If any of the values are
// less than 1, they are clamped to 1. If the end precedes the start, the
// positions are swapped so the range is always well-formed.
func GetAST(ctx context.Context, client *lsp.Client, filePath string, startLine, startColumn, endLine, endColumn int) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %v", err)
	}

	if err := client.OpenFile(ctx, absPath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Normalize and clamp the range to be 1-based and non-empty.
	if startLine < 1 {
		startLine = 1
	}
	if startColumn < 1 {
		startColumn = 1
	}
	if endLine < 1 {
		endLine = 1
	}
	if endColumn < 1 {
		endColumn = 1
	}

	// Ensure (startLine, startColumn) <= (endLine, endColumn).
	if endLine < startLine || (endLine == startLine && endColumn < startColumn) {
		startLine, endLine = endLine, startLine
		startColumn, endColumn = endColumn, startColumn
	}

	uri := protocol.URIFromPath(absPath)
	startPos := protocol.Position{
		Line:      uint32(startLine - 1),
		Character: uint32(startColumn - 1),
	}
	endPos := protocol.Position{
		Line:      uint32(endLine - 1),
		Character: uint32(endColumn - 1),
	}

	params := ASTParams{
		Range: protocol.Range{
			Start: startPos,
			End:   endPos,
		},
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	}

	var root *ASTNode
	if err := client.Call(ctx, "textDocument/ast", params, &root); err != nil {
		return "", fmt.Errorf("failed to get AST: %v", err)
	}

	if root == nil {
		return fmt.Sprintf("No AST available for %s in L%d:C%d - L%d:C%d (clangd returned null).",
			absPath, startLine, startColumn, endLine, endColumn), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("AST for %s in L%d:C%d - L%d:C%d\n\n",
		absPath, startLine, startColumn, endLine, endColumn))
	formatASTNode(&b, root, 0)

	return strings.TrimSuffix(b.String(), "\n"), nil
}

func formatASTNode(b *strings.Builder, n *ASTNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	b.WriteString(prefix)
	b.WriteString(n.Kind)
	if n.Role != "" {
		b.WriteString(" [")
		b.WriteString(n.Role)
		b.WriteString("]")
	}
	if n.Detail != "" {
		b.WriteString(" - ")
		b.WriteString(n.Detail)
	}
	if n.Range != nil {
		startLine := int(n.Range.Start.Line) + 1
		startCol := int(n.Range.Start.Character) + 1
		endLine := int(n.Range.End.Line) + 1
		endCol := int(n.Range.End.Character) + 1
		b.WriteString(fmt.Sprintf(" (L%d:C%d - L%d:C%d)", startLine, startCol, endLine, endCol))
	}
	if n.Arcana != "" {
		b.WriteString("\n")
		b.WriteString(prefix)
		b.WriteString("  ")
		b.WriteString(n.Arcana)
	}
	b.WriteString("\n")

	for i := range n.Children {
		formatASTNode(b, &n.Children[i], indent+1)
	}
}

