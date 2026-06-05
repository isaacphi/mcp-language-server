package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/stretchr/testify/assert"
)

const testURI = protocol.DocumentUri("file:///test.java")

// docSym builds a hierarchical DocumentSymbol. Range.Start uses startLine while
// SelectionRange.Start (what resolvedMember reports) uses selLine so tests can
// assert which node matched.
func docSym(name string, kind protocol.SymbolKind, selLine uint32, children ...protocol.DocumentSymbol) protocol.DocumentSymbol {
	return protocol.DocumentSymbol{
		Name:           name,
		Kind:           kind,
		Range:          protocol.Range{Start: protocol.Position{Line: selLine}},
		SelectionRange: protocol.Range{Start: protocol.Position{Line: selLine, Character: 4}},
		Children:       children,
	}
}

// tree wraps top-level DocumentSymbols as a []DocumentSymbolResult.
func tree(syms ...protocol.DocumentSymbol) []protocol.DocumentSymbolResult {
	out := make([]protocol.DocumentSymbolResult, len(syms))
	for i := range syms {
		out[i] = &syms[i]
	}
	return out
}

// flat wraps SymbolInformation values as a []DocumentSymbolResult.
func flat(syms ...protocol.SymbolInformation) []protocol.DocumentSymbolResult {
	out := make([]protocol.DocumentSymbolResult, len(syms))
	for i := range syms {
		out[i] = &syms[i]
	}
	return out
}

func symInfo(name string, kind protocol.SymbolKind, container string, line uint32) protocol.SymbolInformation {
	return protocol.SymbolInformation{
		Name:          name,
		Kind:          kind,
		ContainerName: container,
		Location:      protocol.Location{URI: testURI, Range: protocol.Range{Start: protocol.Position{Line: line}}},
	}
}

func TestResolveQualifiedSymbolCase2(t *testing.T) {
	// Case 2 dispatch for a 3-part nested name "Outer.Inner.method": the
	// candidate type "Inner" is reported by jdtls with a fully qualified
	// containerName ("com.example.Outer"), while the query's containerPkg is
	// just "Outer". The suffix-tolerant filter must accept it; an exact
	// equality check would reject the legitimate match (the false-negative
	// this branch fixes).
	t.Run("matches a qualified containerName against a simple containerPkg", func(t *testing.T) {
		const innerURI = protocol.DocumentUri("file:///Outer.java")

		origWS := workspaceSymbols
		origMembers := findMembersInDocument
		t.Cleanup(func() {
			workspaceSymbols = origWS
			findMembersInDocument = origMembers
		})

		var gotContainer, gotMember string
		// Case 1 queries the member ("method") and finds nothing, so resolution
		// falls through to Case 2, which queries the simple container ("Inner").
		workspaceSymbols = func(_ context.Context, _ *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
			if query != "Inner" {
				return nil, nil
			}
			return []protocol.WorkspaceSymbolResult{
				&protocol.WorkspaceSymbol{
					BaseSymbolInformation: protocol.BaseSymbolInformation{
						Name:          "Inner",
						Kind:          protocol.Class,
						ContainerName: "com.example.Outer",
					},
					Location: protocol.Or_WorkspaceSymbol_location{Value: protocol.Location{URI: innerURI}},
				},
			}, nil
		}
		findMembersInDocument = func(_ context.Context, _ *lsp.Client, uri protocol.DocumentUri, container, member string) ([]resolvedSymbol, error) {
			gotContainer, gotMember = container, member
			assert.Equal(t, innerURI, uri)
			return []resolvedSymbol{{
				Name:     "method",
				Kind:     "Method",
				Location: protocol.Location{URI: uri, Range: protocol.Range{Start: protocol.Position{Line: 15}}},
			}}, nil
		}

		got, err := resolveQualifiedSymbol(context.Background(), nil, "Outer.Inner.method")
		assert.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, "method", got[0].Name)
		assert.Equal(t, uint32(15), got[0].Location.Range.Start.Line)
		assert.Equal(t, "Inner", got[0].ContainerName, "container is set to the resolved type's name")
		assert.Equal(t, "Inner", gotContainer, "member lookup is scoped to the simple container")
		assert.Equal(t, "method", gotMember)
	})

	t.Run("rejects a candidate whose containerPkg does not match", func(t *testing.T) {
		origWS := workspaceSymbols
		origMembers := findMembersInDocument
		t.Cleanup(func() {
			workspaceSymbols = origWS
			findMembersInDocument = origMembers
		})

		// First call: member lookup for Case 1 ("method") finds nothing.
		// Second call: container lookup for Case 2 ("Inner") returns a type
		// whose container is "com.example.Other" — not the requested "Outer".
		workspaceSymbols = func(_ context.Context, _ *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
			if query == "Inner" {
				return []protocol.WorkspaceSymbolResult{
					&protocol.WorkspaceSymbol{
						BaseSymbolInformation: protocol.BaseSymbolInformation{
							Name:          "Inner",
							Kind:          protocol.Class,
							ContainerName: "com.example.Other",
						},
						Location: protocol.Or_WorkspaceSymbol_location{Value: protocol.Location{URI: testURI}},
					},
				}, nil
			}
			return nil, nil
		}
		findMembersInDocument = func(context.Context, *lsp.Client, protocol.DocumentUri, string, string) ([]resolvedSymbol, error) {
			t.Fatal("findMembersInDocument must not be called for a non-matching containerPkg")
			return nil, nil
		}

		got, err := resolveQualifiedSymbol(context.Background(), nil, "Outer.Inner.method")
		assert.NoError(t, err)
		assert.Empty(t, got)
	})
}

// TestResolveQualifiedSymbolDedupesAcrossWorkspaceResults covers the
// dedupSymbols call after Case 2 collects members across every matching
// workspace/symbol result. scopeMembersToContainer only dedups within a single
// document, so a type indexed under two workspace-symbol entries that resolve
// to the same member location would otherwise be reported twice. This is the
// cross-result dedup the per-document tests do not exercise.
func TestResolveQualifiedSymbolDedupesAcrossWorkspaceResults(t *testing.T) {
	origWS := workspaceSymbols
	origMembers := findMembersInDocument
	t.Cleanup(func() {
		workspaceSymbols = origWS
		findMembersInDocument = origMembers
	})

	// Case 1 ("baz") finds nothing; Case 2 ("Foo") returns two duplicate type
	// entries (as a stale/duplicated index can), each resolving to the same
	// member location.
	const fooURI = protocol.DocumentUri("file:///Foo.java")
	workspaceSymbols = func(_ context.Context, _ *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
		if query != "Foo" {
			return nil, nil
		}
		entry := &protocol.WorkspaceSymbol{
			BaseSymbolInformation: protocol.BaseSymbolInformation{Name: "Foo", Kind: protocol.Class},
			Location:              protocol.Or_WorkspaceSymbol_location{Value: protocol.Location{URI: fooURI}},
		}
		return []protocol.WorkspaceSymbolResult{entry, entry}, nil
	}
	findMembersInDocument = func(_ context.Context, _ *lsp.Client, uri protocol.DocumentUri, _, _ string) ([]resolvedSymbol, error) {
		return []resolvedSymbol{{
			Name:     "baz",
			Kind:     "Method",
			Location: protocol.Location{URI: uri, Range: protocol.Range{Start: protocol.Position{Line: 10}}},
		}}, nil
	}

	got, err := resolveQualifiedSymbol(context.Background(), nil, "Foo.baz")
	assert.NoError(t, err)
	assert.Len(t, got, 1, "the same member resolved from two workspace results must be collapsed")
	assert.Equal(t, "baz", got[0].Name)
}

// TestResolveQualifiedEntries covers the resolve-and-render scaffold used by
// the definition qualified-name fallback (references.go calls
// resolveQualifiedSymbol directly with its own collect closure because its
// error policy differs): it resolves the symbol, renders each resolved entry,
// and accumulates the output. A resolve error is
// propagated; a per-entry render error is logged and skipped rather than
// aborting the whole fallback. The render func here is deliberately
// client-free so the wiring is exercised without a live LSP.
func TestResolveQualifiedEntries(t *testing.T) {
	t.Run("renders every resolved entry", func(t *testing.T) {
		origWS := workspaceSymbols
		origMembers := findMembersInDocument
		t.Cleanup(func() {
			workspaceSymbols = origWS
			findMembersInDocument = origMembers
		})

		const fooURI = protocol.DocumentUri("file:///Foo.java")
		workspaceSymbols = func(_ context.Context, _ *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
			if query != "Foo" {
				return nil, nil
			}
			return []protocol.WorkspaceSymbolResult{
				&protocol.WorkspaceSymbol{
					BaseSymbolInformation: protocol.BaseSymbolInformation{Name: "Foo", Kind: protocol.Class},
					Location:              protocol.Or_WorkspaceSymbol_location{Value: protocol.Location{URI: fooURI}},
				},
			}, nil
		}
		findMembersInDocument = func(_ context.Context, _ *lsp.Client, uri protocol.DocumentUri, _, _ string) ([]resolvedSymbol, error) {
			return []resolvedSymbol{
				{Name: "baz", Kind: "Method", ContainerName: "Foo", Location: protocol.Location{URI: uri, Range: protocol.Range{Start: protocol.Position{Line: 10}}}},
			}, nil
		}

		var rendered []resolvedSymbol
		out, err := resolveQualifiedEntries(context.Background(), nil, "Foo.baz", func(sym resolvedSymbol) ([]string, error) {
			rendered = append(rendered, sym)
			return []string{"entry:" + sym.Name}, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"entry:baz"}, out)
		assert.Len(t, rendered, 1, "render is invoked once per resolved symbol")
		assert.Equal(t, "Foo", rendered[0].ContainerName)
	})

	t.Run("propagates a resolve error", func(t *testing.T) {
		origWS := workspaceSymbols
		t.Cleanup(func() { workspaceSymbols = origWS })

		workspaceSymbols = func(context.Context, *lsp.Client, string) ([]protocol.WorkspaceSymbolResult, error) {
			return nil, errors.New("symbol query failed")
		}

		out, err := resolveQualifiedEntries(context.Background(), nil, "Foo.baz", func(resolvedSymbol) ([]string, error) {
			t.Fatal("render must not run when resolution fails")
			return nil, nil
		})
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("skips entries whose render fails but keeps the rest", func(t *testing.T) {
		origWS := workspaceSymbols
		origMembers := findMembersInDocument
		t.Cleanup(func() {
			workspaceSymbols = origWS
			findMembersInDocument = origMembers
		})

		const fooURI = protocol.DocumentUri("file:///Foo.java")
		workspaceSymbols = func(_ context.Context, _ *lsp.Client, query string) ([]protocol.WorkspaceSymbolResult, error) {
			if query != "Foo" {
				return nil, nil
			}
			return []protocol.WorkspaceSymbolResult{
				&protocol.WorkspaceSymbol{
					BaseSymbolInformation: protocol.BaseSymbolInformation{Name: "Foo", Kind: protocol.Class},
					Location:              protocol.Or_WorkspaceSymbol_location{Value: protocol.Location{URI: fooURI}},
				},
			}, nil
		}
		findMembersInDocument = func(_ context.Context, _ *lsp.Client, uri protocol.DocumentUri, _, _ string) ([]resolvedSymbol, error) {
			return []resolvedSymbol{
				{Name: "bad", Location: protocol.Location{URI: uri, Range: protocol.Range{Start: protocol.Position{Line: 1}}}},
				{Name: "good", Location: protocol.Location{URI: uri, Range: protocol.Range{Start: protocol.Position{Line: 2}}}},
			}, nil
		}

		out, err := resolveQualifiedEntries(context.Background(), nil, "Foo.baz", func(sym resolvedSymbol) ([]string, error) {
			if sym.Name == "bad" {
				return nil, errors.New("render failed")
			}
			return []string{"entry:" + sym.Name}, nil
		})
		assert.NoError(t, err, "a per-entry render error must not abort the fallback")
		assert.Equal(t, []string{"entry:good"}, out)
	})
}

func TestScopeMembersToContainer(t *testing.T) {
	t.Run("scopes to the requested container in a multi-type file", func(t *testing.T) {
		// Foo.baz at line 10, Bar.baz at line 20 — resolving Foo.baz must not
		// return Bar.baz.
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("baz", protocol.Method, 10),
				docSym("other", protocol.Method, 11),
			),
			docSym("Bar", protocol.Class, 2,
				docSym("baz", protocol.Method, 20),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, "baz", got[0].Name)
		assert.Equal(t, "Method", got[0].Kind)
		assert.Equal(t, uint32(10), got[0].Location.Range.Start.Line)
	})

	t.Run("matches signature-suffixed method names", func(t *testing.T) {
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("baz(String, int)", protocol.Method, 10),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, "baz(String, int)", got[0].Name)
	})

	t.Run("resolves members of a nested type", func(t *testing.T) {
		symbols := tree(
			docSym("Outer", protocol.Class, 1,
				docSym("method", protocol.Method, 5), // Outer.method, must be excluded
				docSym("Inner", protocol.Class, 2,
					docSym("method", protocol.Method, 15),
				),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Inner", "method")

		assert.Len(t, got, 1)
		assert.Equal(t, uint32(15), got[0].Location.Range.Start.Line)
	})

	t.Run("tolerates a generic container suffix", func(t *testing.T) {
		symbols := tree(
			docSym("Foo<T>", protocol.Class, 1,
				docSym("baz", protocol.Method, 10),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, uint32(10), got[0].Location.Range.Start.Line)
	})

	t.Run("scopes flat SymbolInformation by ContainerName", func(t *testing.T) {
		symbols := flat(
			symInfo("Foo", protocol.Class, "", 1),
			symInfo("baz", protocol.Method, "Foo", 10),
			symInfo("baz", protocol.Method, "Bar", 20),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, uint32(10), got[0].Location.Range.Start.Line)
	})

	t.Run("scopes flat SymbolInformation with a qualified ContainerName", func(t *testing.T) {
		symbols := flat(
			symInfo("baz", protocol.Method, "com.example.Foo", 10),
			symInfo("baz", protocol.Method, "com.example.Bar", 20),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, uint32(10), got[0].Location.Range.Start.Line)
	})

	t.Run("falls back to document-wide match when container is absent", func(t *testing.T) {
		// Container "Zed" does not exist: preserve prior behaviour rather than
		// returning nothing.
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("baz", protocol.Method, 10),
			),
			docSym("Bar", protocol.Class, 2,
				docSym("baz", protocol.Method, 20),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Zed", "baz")

		assert.Len(t, got, 2)
	})

	t.Run("returns nothing when the container has no matching member", func(t *testing.T) {
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("other", protocol.Method, 10),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Empty(t, got)
	})

	t.Run("flat SymbolInformation keeps its own location URI", func(t *testing.T) {
		// A SymbolInformation may point at a different file than the document we
		// queried (generated/partial sources); resolvedMember must not overwrite
		// its URI with the queried document URI.
		const otherURI = protocol.DocumentUri("file:///other.java")
		symbols := flat(
			protocol.SymbolInformation{
				Name:          "baz",
				Kind:          protocol.Method,
				ContainerName: "Foo",
				Location:      protocol.Location{URI: otherURI, Range: protocol.Range{Start: protocol.Position{Line: 7}}},
			},
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
		assert.Equal(t, otherURI, got[0].Location.URI)
		assert.Equal(t, uint32(7), got[0].Location.Range.Start.Line)
	})

	t.Run("collapses duplicate members at the same location", func(t *testing.T) {
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("baz", protocol.Method, 10),
				docSym("baz", protocol.Method, 10),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 1)
	})

	t.Run("keeps distinct same-named members of nested same-named types", func(t *testing.T) {
		// Outer Foo.baz (line 10) and a nested type also named Foo with its own
		// baz (line 20) are genuinely distinct symbols and must both survive.
		symbols := tree(
			docSym("Foo", protocol.Class, 1,
				docSym("baz", protocol.Method, 10),
				docSym("Foo", protocol.Class, 2,
					docSym("baz", protocol.Method, 20),
				),
			),
		)

		got := scopeMembersToContainer(symbols, testURI, "Foo", "baz")

		assert.Len(t, got, 2)
	})
}
