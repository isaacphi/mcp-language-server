// Code generated by "generate"; DO NOT EDIT.
package methods

import (
	"fmt"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/kralicky/tools-lite/gopls/pkg/protocol"
)

// Wrapper provides type-safe methods for LSP operations
type Wrapper struct {
	client *lsp.Client
}

// NewWrapper creates a new LSP method wrapper
func NewWrapper(client *lsp.Client) *Wrapper {
	return &Wrapper{client: client}
}

// Initialize sends a General request for initialize
func (w *Wrapper) Initialize(params protocol.InitializeParams) (protocol.InitializeResult, error) {
	var result protocol.InitializeResult

	err := w.client.Call("initialize", params, &result)

	return result, err
}

// Initialized sends a General notification for initialized
func (w *Wrapper) Initialized(params protocol.InitializedParams) error {

	return w.client.Notify("initialized", params)

}

// Shutdown sends a General request for shutdown
func (w *Wrapper) Shutdown() error {

	return w.client.Call("shutdown", struct{}{}, nil)

}

// Exit sends a General notification for exit
func (w *Wrapper) Exit() error {

	return w.client.Notify("exit", struct{}{})

}

// TextDocumentDidOpen sends a TextDocument notification for textDocument/didOpen
func (w *Wrapper) TextDocumentDidOpen(params protocol.DidOpenTextDocumentParams) error {

	return w.client.Notify("textDocument/didOpen", params)

}

// TextDocumentDidChange sends a TextDocument notification for textDocument/didChange
func (w *Wrapper) TextDocumentDidChange(params protocol.DidChangeTextDocumentParams) error {

	return w.client.Notify("textDocument/didChange", params)

}

// TextDocumentDidClose sends a TextDocument notification for textDocument/didClose
func (w *Wrapper) TextDocumentDidClose(params protocol.DidCloseTextDocumentParams) error {

	return w.client.Notify("textDocument/didClose", params)

}

// TextDocumentWillSave sends a TextDocument notification for textDocument/willSave
func (w *Wrapper) TextDocumentWillSave(params protocol.WillSaveTextDocumentParams) error {

	return w.client.Notify("textDocument/willSave", params)

}

// TextDocumentWillSaveWaitUntil sends a TextDocument request for textDocument/willSaveWaitUntil
func (w *Wrapper) TextDocumentWillSaveWaitUntil(params protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit

	err := w.client.Call("textDocument/willSaveWaitUntil", params, &result)

	return result, err
}

// TextDocumentDidSave sends a TextDocument notification for textDocument/didSave
func (w *Wrapper) TextDocumentDidSave(params protocol.DidSaveTextDocumentParams) error {

	return w.client.Notify("textDocument/didSave", params)

}

// TextDocumentCompletion sends a TextDocument request for textDocument/completion
// Returns: protocol.CompletionList or []protocol.CompletionItem
func (w *Wrapper) TextDocumentCompletion(params protocol.CompletionParams) (interface{}, error) {

	// Try type CompletionList
	{
		var result0 protocol.CompletionList

		err := w.client.Call("textDocument/completion", params, &result0)

		if err == nil {
			return result0, nil
		}
	}
	// Try type CompletionItem
	{
		var result1 []protocol.CompletionItem

		err := w.client.Call("textDocument/completion", params, &result1)

		if err == nil {
			return result1, nil
		}
	}
	return nil, fmt.Errorf("all response type attempts failed")
}

// TextDocumentHover sends a TextDocument request for textDocument/hover
func (w *Wrapper) TextDocumentHover(params protocol.HoverParams) (protocol.Hover, error) {
	var result protocol.Hover

	err := w.client.Call("textDocument/hover", params, &result)

	return result, err
}

// TextDocumentSignatureHelp sends a TextDocument request for textDocument/signatureHelp
func (w *Wrapper) TextDocumentSignatureHelp(params protocol.SignatureHelpParams) (protocol.SignatureHelp, error) {
	var result protocol.SignatureHelp

	err := w.client.Call("textDocument/signatureHelp", params, &result)

	return result, err
}

// TextDocumentDefinition sends a TextDocument request for textDocument/definition
// Returns: protocol.Location or []protocol.Location or []protocol.DefinitionLink
func (w *Wrapper) TextDocumentDefinition(params protocol.DefinitionParams) (interface{}, error) {

	// Try type Location
	{
		var result0 protocol.Location

		err := w.client.Call("textDocument/definition", params, &result0)

		if err == nil {
			return result0, nil
		}
	}
	// Try type Location
	{
		var result1 []protocol.Location

		err := w.client.Call("textDocument/definition", params, &result1)

		if err == nil {
			return result1, nil
		}
	}
	// Try type DefinitionLink
	{
		var result2 []protocol.DefinitionLink

		err := w.client.Call("textDocument/definition", params, &result2)

		if err == nil {
			return result2, nil
		}
	}
	return nil, fmt.Errorf("all response type attempts failed")
}

// TextDocumentDeclaration sends a TextDocument request for textDocument/declaration
// Returns: protocol.Location or []protocol.Location or []protocol.DeclarationLink
func (w *Wrapper) TextDocumentDeclaration(params protocol.DeclarationParams) (interface{}, error) {

	// Try type Location
	{
		var result0 protocol.Location

		err := w.client.Call("textDocument/declaration", params, &result0)

		if err == nil {
			return result0, nil
		}
	}
	// Try type Location
	{
		var result1 []protocol.Location

		err := w.client.Call("textDocument/declaration", params, &result1)

		if err == nil {
			return result1, nil
		}
	}
	// Try type DeclarationLink
	{
		var result2 []protocol.DeclarationLink

		err := w.client.Call("textDocument/declaration", params, &result2)

		if err == nil {
			return result2, nil
		}
	}
	return nil, fmt.Errorf("all response type attempts failed")
}

// TextDocumentReferences sends a TextDocument request for textDocument/references
func (w *Wrapper) TextDocumentReferences(params protocol.ReferenceParams) ([]protocol.Location, error) {
	var result []protocol.Location

	err := w.client.Call("textDocument/references", params, &result)

	return result, err
}

// TextDocumentDocumentHighlight sends a TextDocument request for textDocument/documentHighlight
func (w *Wrapper) TextDocumentDocumentHighlight(params protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	var result []protocol.DocumentHighlight

	err := w.client.Call("textDocument/documentHighlight", params, &result)

	return result, err
}

// TextDocumentDocumentSymbol sends a TextDocument request for textDocument/documentSymbol
// Returns: []protocol.DocumentSymbol or []protocol.SymbolInformation
func (w *Wrapper) TextDocumentDocumentSymbol(params protocol.DocumentSymbolParams) (interface{}, error) {

	// Try type DocumentSymbol
	{
		var result0 []protocol.DocumentSymbol

		err := w.client.Call("textDocument/documentSymbol", params, &result0)

		if err == nil {
			return result0, nil
		}
	}
	// Try type SymbolInformation
	{
		var result1 []protocol.SymbolInformation

		err := w.client.Call("textDocument/documentSymbol", params, &result1)

		if err == nil {
			return convertSymbolInformationSliceToDocumentSymbolSlice(result1), nil
		}
	}
	return nil, fmt.Errorf("all response type attempts failed")
}

// TextDocumentFormatting sends a TextDocument request for textDocument/formatting
func (w *Wrapper) TextDocumentFormatting(params protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit

	err := w.client.Call("textDocument/formatting", params, &result)

	return result, err
}

// TextDocumentRangeFormatting sends a TextDocument request for textDocument/rangeFormatting
func (w *Wrapper) TextDocumentRangeFormatting(params protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit

	err := w.client.Call("textDocument/rangeFormatting", params, &result)

	return result, err
}