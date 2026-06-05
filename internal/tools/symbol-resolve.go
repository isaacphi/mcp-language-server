package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// resolvedSymbol is a symbol location resolved via the qualified-name
// fallback (workspace/symbol for the container + documentSymbol for the
// member). Some servers (e.g. jdtls) only index types in workspace/symbol,
// so member symbols like "Class.method" are unreachable without this.
type resolvedSymbol struct {
	Name          string
	Kind          string
	ContainerName string
	Location      protocol.Location
}

// resolveQualifiedSymbol resolves qualified symbol names that
// workspace/symbol could not match directly. It handles two shapes:
//
//  1. Fully qualified top-level symbols, e.g. "com.example.Foo":
//     matches workspace/symbol results for "Foo" whose container is
//     "com.example".
//  2. Members of a type, e.g. "Foo.bar" or "com.example.Foo.bar":
//     resolves the type via workspace/symbol, then locates "bar" with
//     textDocument/documentSymbol in the type's file.
func resolveQualifiedSymbol(ctx context.Context, client *lsp.Client, symbolName string) ([]resolvedSymbol, error) {
	parts := strings.Split(symbolName, ".")
	if len(parts) < 2 {
		return nil, nil
	}
	member := parts[len(parts)-1]
	container := strings.Join(parts[:len(parts)-1], ".")

	// Case 1: symbolName is a fully qualified top-level symbol.
	symbols, err := workspaceSymbols(ctx, client, member)
	if err != nil {
		return nil, err
	}
	var resolved []resolvedSymbol
	for _, sym := range symbols {
		if sym.GetName() != member {
			continue
		}
		kind, symContainer := symbolKindAndContainer(sym)
		if symContainer == container {
			resolved = append(resolved, resolvedSymbol{
				Name:          member,
				Kind:          kind,
				ContainerName: symContainer,
				Location:      sym.GetLocation(),
			})
		}
	}
	if len(resolved) > 0 {
		return resolved, nil
	}

	// Case 2: container is a type; locate the member via documentSymbol.
	simpleContainer := parts[len(parts)-2]
	containerPkg := strings.Join(parts[:len(parts)-2], ".")
	symbols, err = workspaceSymbols(ctx, client, simpleContainer)
	if err != nil {
		return nil, err
	}
	for _, sym := range symbols {
		if sym.GetName() != simpleContainer {
			continue
		}
		if containerPkg != "" {
			// Suffix-tolerant, consistent with the rest of this file:
			// jdtls reports a nested type's containerName as its fully
			// qualified enclosing scope (e.g. "com.example.Outer" for the
			// query "Outer.Inner.method"), so an exact "== containerPkg"
			// check would reject the legitimate match. flatContainerMatches
			// also accepts the exact and "Outer$Inner" forms, so gopls/pyright
			// flat results keep matching.
			if _, symContainer := symbolKindAndContainer(sym); !flatContainerMatches(symContainer, containerPkg) {
				continue
			}
		}
		loc := sym.GetLocation()
		members, err := findMembersInDocument(ctx, client, loc.URI, simpleContainer, member)
		if err != nil {
			toolsLogger.Error("documentSymbol fallback failed for %s: %v", loc.URI, err)
			continue
		}
		for _, m := range members {
			m.ContainerName = sym.GetName()
			resolved = append(resolved, m)
		}
	}
	// Dedup across matching workspace symbols: scopeMembersToContainer only
	// dedups within a single document, so a type indexed under duplicate
	// entries (same file) could otherwise yield the same member twice.
	return dedupSymbols(resolved), nil
}

// resolveQualifiedEntries runs the qualified-name fallback and renders each
// resolved symbol with render, accumulating the non-empty output. It is the
// shared scaffold for the definition and references fallbacks so their trigger
// and iteration semantics stay identical. A resolve error is propagated; a
// per-symbol render error is logged and skipped, since the fallback is a
// best-effort path over speculatively resolved symbols.
func resolveQualifiedEntries(ctx context.Context, client *lsp.Client, symbolName string, render func(sym resolvedSymbol) ([]string, error)) ([]string, error) {
	resolved, err := resolveQualifiedSymbol(ctx, client, symbolName)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, sym := range resolved {
		entries, err := render(sym)
		if err != nil {
			toolsLogger.Error("qualified-name fallback failed for %s: %v", sym.Name, err)
			continue
		}
		out = append(out, entries...)
	}
	return out, nil
}

// workspaceSymbols is a package-level variable so tests can stub the
// workspace/symbol query without a live LSP client. Production code always
// uses the default below. Because it is shared global state, tests that
// reassign it must restore it (via t.Cleanup) and must not run with
// t.Parallel().
var workspaceSymbols = func(ctx context.Context, client *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: query})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch symbol: %v", err)
	}
	return symbolResult.Results()
}

// symbolKindAndContainer extracts the symbol kind name and container name
// from either workspace/symbol result type.
func symbolKindAndContainer(sym protocol.WorkspaceSymbolResult) (string, string) {
	switch v := sym.(type) {
	case *protocol.SymbolInformation:
		return protocol.TableKindMap[v.Kind], v.ContainerName
	case *protocol.WorkspaceSymbol:
		return protocol.TableKindMap[v.Kind], v.ContainerName
	}
	return "", ""
}

// findMembersInDocument finds symbols named member that belong to the type
// named container, within the document symbol tree of uri. Scoping to the
// container prevents matching same-named members of unrelated types in the
// same file. Matches either the exact name or a signature-suffixed name like
// "member(String, int)" as jdtls reports methods.
// findMembersInDocument is a package-level variable so tests can stub the
// document-symbol fetch without a live LSP client. Production code always uses
// the default below. Because it is shared global state, tests that reassign it
// must restore it (via t.Cleanup) and must not run with t.Parallel().
var findMembersInDocument = func(ctx context.Context, client *lsp.Client, uri protocol.DocumentUri, container, member string) ([]resolvedSymbol, error) {
	path, err := uriToPath(uri)
	if err != nil {
		return nil, err
	}
	if err := client.OpenFile(ctx, path); err != nil {
		return nil, err
	}
	symResult, err := client.DocumentSymbol(ctx, protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		return nil, err
	}
	symbols, err := symResult.Results()
	if err != nil {
		return nil, err
	}
	return scopeMembersToContainer(symbols, uri, container, member), nil
}

// memberMatches reports whether a document-symbol name matches member, either
// exactly or as a signature-suffixed method name like "member(String, int)".
func memberMatches(name, member string) bool {
	return name == member || strings.HasPrefix(name, member+"(")
}

// containerMatches reports whether a type symbol name matches container,
// tolerating a generic suffix like "Foo<T>".
func containerMatches(name, container string) bool {
	return name == container || strings.HasPrefix(name, container+"<")
}

// resolvedMember builds a resolvedSymbol from a document symbol matched as a
// member. The location is anchored at a single position because downstream
// definition/references requests are position-based.
//
// For DocumentSymbol it uses SelectionRange (the identifier itself) and the
// document uri, since DocumentSymbol carries no URI of its own. For
// SymbolInformation it uses the symbol's own Location — both the URI (which may
// differ from the document we queried, e.g. generated/partial sources) and the
// range start. SymbolInformation has no selection range, so the range start is
// the best available anchor.
func resolvedMember(sym protocol.DocumentSymbolResult, uri protocol.DocumentUri) resolvedSymbol {
	var loc protocol.Location
	kind := ""
	switch v := sym.(type) {
	case *protocol.DocumentSymbol:
		pos := v.SelectionRange.Start
		loc = protocol.Location{URI: uri, Range: protocol.Range{Start: pos, End: pos}}
		kind = protocol.TableKindMap[v.Kind]
	case *protocol.SymbolInformation:
		pos := v.Location.Range.Start
		symURI := v.Location.URI
		if symURI == "" {
			symURI = uri
		}
		loc = protocol.Location{URI: symURI, Range: protocol.Range{Start: pos, End: pos}}
		kind = protocol.TableKindMap[v.Kind]
	}
	return resolvedSymbol{
		Name:     sym.GetName(),
		Kind:     kind,
		Location: loc,
	}
}

// scopeMembersToContainer finds members named member that belong to the type
// named container within symbols. It handles both hierarchical DocumentSymbol
// trees (members are direct children of the type node) and flat
// SymbolInformation lists (members carry a ContainerName). If the container
// type cannot be located at all, it falls back to matching member anywhere in
// the document so resolution is not lost for servers with unusual symbol
// shapes.
func scopeMembersToContainer(symbols []protocol.DocumentSymbolResult, uri protocol.DocumentUri, container, member string) []resolvedSymbol {
	var found []resolvedSymbol
	var anywhere []resolvedSymbol
	containerSeen := false

	// A single depth-first walk handles both shapes and, in the same pass,
	// gathers the document-wide matches used by the safety fallback so the
	// tree is never traversed twice:
	//   - Hierarchical DocumentSymbol: a type node named container holds its
	//     members as direct children. Recursion reaches nested types.
	//   - Flat SymbolInformation: members carry a ContainerName instead.
	walkDocumentSymbols(symbols, func(sym protocol.DocumentSymbolResult) {
		if memberMatches(sym.GetName(), member) {
			anywhere = append(anywhere, resolvedMember(sym, uri))
		}
		switch v := sym.(type) {
		case *protocol.DocumentSymbol:
			if containerMatches(v.Name, container) {
				containerSeen = true
				for i := range v.Children {
					if child := &v.Children[i]; memberMatches(child.Name, member) {
						found = append(found, resolvedMember(child, uri))
					}
				}
			}
		case *protocol.SymbolInformation:
			if flatContainerMatches(v.ContainerName, container) {
				containerSeen = true
				if memberMatches(v.Name, member) {
					found = append(found, resolvedMember(v, uri))
				}
			}
		}
	})

	// containerSeen alone is sufficient: found is only appended to inside
	// branches that have already set containerSeen, so len(found) > 0 implies it.
	if containerSeen {
		return dedupSymbols(found)
	}

	// Safety fallback: container not found; match member anywhere so we don't
	// regress resolution for servers with unusual document-symbol shapes. This
	// is a degraded path that can return members of unrelated types, so log it
	// at Warn rather than hiding it at Debug.
	toolsLogger.Warn("container %q not found in %s; matching member %q document-wide", container, uri, member)
	return dedupSymbols(anywhere)
}

// walkDocumentSymbols invokes visit for every symbol in the tree, depth-first,
// descending into the children of any hierarchical DocumentSymbol node. It is
// the single traversal shared by the container-scoped and document-wide member
// searches.
func walkDocumentSymbols(symbols []protocol.DocumentSymbolResult, visit func(protocol.DocumentSymbolResult)) {
	for _, sym := range symbols {
		visit(sym)
		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			children := make([]protocol.DocumentSymbolResult, len(ds.Children))
			for i := range ds.Children {
				children[i] = &ds.Children[i]
			}
			walkDocumentSymbols(children, visit)
		}
	}
}

// dedupSymbols removes entries that resolve to the same name at the same
// location, which can happen when overlapping symbols are reported more than
// once. Genuinely distinct symbols (different positions) are preserved.
func dedupSymbols(syms []resolvedSymbol) []resolvedSymbol {
	if len(syms) <= 1 {
		return syms
	}
	seen := make(map[string]bool, len(syms))
	out := make([]resolvedSymbol, 0, len(syms))
	for _, s := range syms {
		key := fmt.Sprintf("%s\x00%d\x00%d\x00%s",
			s.Location.URI, s.Location.Range.Start.Line, s.Location.Range.Start.Character, s.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// flatContainerMatches reports whether a SymbolInformation ContainerName
// (which may be qualified, e.g. "com.example.Foo" or "Outer$Inner") refers to
// the type named container.
func flatContainerMatches(containerName, container string) bool {
	return containerMatches(containerName, container) ||
		strings.HasSuffix(containerName, "."+container) ||
		strings.HasSuffix(containerName, "$"+container)
}
