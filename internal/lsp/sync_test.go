package lsp

import (
	"os"
	"path/filepath"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp"
)

// Reproduces the "symbols land a few lines below / gd-gr find nothing" report:
// after an edit, the index must reflect the new text, not the opened text.
func TestDidChangeReindexes(t *testing.T) {
	s := NewServer()
	abs, _ := filepath.Abs("../../testdata/petstore.yaml")
	src, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	uri := "file://" + abs
	if err := s.didOpen(&glsp.Context{}, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: string(src)},
	}); err != nil {
		t.Fatal(err)
	}

	// User: is at line 30 (0-based) on open.
	if rng, ok := s.doc(uri).Resolve("/components/schemas/User"); !ok || rng.Start.Line != 30 {
		t.Fatalf("on open: User line = %v (ok=%v), want 30", rng.Start.Line, ok)
	}

	// Prepend 3 blank lines via a Full-sync didChange (whole new text).
	newText := "\n\n\n" + string(src)
	if err := s.didChange(&glsp.Context{}, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
		},
		ContentChanges: []any{
			protocol.TextDocumentContentChangeEventWhole{Text: newText},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// After the edit User: must now be at line 33.
	rng, ok := s.doc(uri).Resolve("/components/schemas/User")
	if !ok {
		t.Fatal("after change: cannot resolve User")
	}
	if rng.Start.Line != 33 {
		t.Errorf("after change: User line = %d, want 33 (index is stale)", rng.Start.Line)
	}
}

// Verifies exact column, catching the 1-based goccy offset bug.
func TestExactKeyColumn(t *testing.T) {
	// "    User:" -> U is at column 4 (0-based).
	src := "openapi: 3.0.3\ncomponents:\n  schemas:\n    User:\n      type: object\n"
	s := NewServer()
	uri := "file:///x.yaml"
	if err := s.didOpen(&glsp.Context{}, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: src},
	}); err != nil {
		t.Fatal(err)
	}
	rng, ok := s.doc(uri).Resolve("/components/schemas/User")
	if !ok {
		t.Fatal("cannot resolve User")
	}
	if rng.Start.Line != 3 {
		t.Errorf("User line = %d, want 3", rng.Start.Line)
	}
	if rng.Start.Character != 4 {
		t.Errorf("User character = %d, want 4 (off-by-one offset bug)", rng.Start.Character)
	}
	if rng.End.Character != 8 { // "User" is 4 chars: cols 4..8
		t.Errorf("User end character = %d, want 8", rng.End.Character)
	}
}
