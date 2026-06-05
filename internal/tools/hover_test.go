package tools

import (
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/stretchr/testify/assert"
)

// TestRenderHoverContents covers the three hover content shapes the renderer
// must handle: MarkupContent ({kind, value}), a plain-string MarkedString, and
// a []MarkedString containing a MarkedStringWithLanguage (which must be wrapped
// in a language-tagged code fence). These are the shapes jdtls and other
// servers emit, so the fix's formatting is asserted here without a live LSP.
func TestRenderHoverContents(t *testing.T) {
	tests := []struct {
		name     string
		contents protocol.Or_Hover_contents
		want     string
	}{
		{
			name: "MarkupContent returns its value verbatim",
			contents: protocol.Or_Hover_contents{Value: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: "# Title\nbody",
			}},
			want: "# Title\nbody",
		},
		{
			name: "plain-string MarkedString returns the string",
			contents: protocol.Or_Hover_contents{Value: protocol.MarkedString{
				Value: "just a string",
			}},
			want: "just a string",
		},
		{
			name: "[]MarkedString with MarkedStringWithLanguage is code-fenced",
			contents: protocol.Or_Hover_contents{Value: []protocol.MarkedString{
				{Value: protocol.MarkedStringWithLanguage{Language: "java", Value: "int x"}},
				{Value: "documentation text"},
			}},
			want: "```java\nint x\n```\n\ndocumentation text",
		},
		{
			// A top-level MarkedStringWithLanguage (not inside a slice) goes
			// through the distinct `case protocol.MarkedString` branch and must
			// still be wrapped in a language-tagged code fence.
			name: "top-level MarkedStringWithLanguage is code-fenced",
			contents: protocol.Or_Hover_contents{Value: protocol.MarkedString{
				Value: protocol.MarkedStringWithLanguage{Language: "java", Value: "int x"},
			}},
			want: "```java\nint x\n```",
		},
		{
			// Empty []MarkedString entries are dropped so they do not produce
			// stray blank separators in the joined output.
			name: "[]MarkedString drops empty entries",
			contents: protocol.Or_Hover_contents{Value: []protocol.MarkedString{
				{Value: "first"},
				{Value: ""},
				{Value: "second"},
			}},
			want: "first\n\nsecond",
		},
		{
			name:     "unknown shape returns empty string",
			contents: protocol.Or_Hover_contents{Value: 42},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderHoverContents(tt.contents)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRenderMarkedString covers the two MarkedString variants directly: a plain
// string passes through unchanged, and a MarkedStringWithLanguage becomes a
// language-tagged code fence.
func TestRenderMarkedString(t *testing.T) {
	assert.Equal(t, "plain", renderMarkedString(protocol.MarkedString{Value: "plain"}))
	assert.Equal(t,
		"```go\nfunc f()\n```",
		renderMarkedString(protocol.MarkedString{
			Value: protocol.MarkedStringWithLanguage{Language: "go", Value: "func f()"},
		}),
	)
	assert.Equal(t, "", renderMarkedString(protocol.MarkedString{Value: 0}))
}
