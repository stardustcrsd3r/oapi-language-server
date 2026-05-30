package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stardustcrsd3r/oapi-language-server/internal/spec"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// These tests exercise the handlers directly (the glsp dispatch layer is just
// JSON-RPC plumbing). We feed documents through the lifecycle handlers exactly
// as the dispatcher would, then call the feature handlers.

func openDoc(t *testing.T, s *Server, path string) string {
	t.Helper()
	abs, _ := filepath.Abs(path)
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
	return uri
}

func refPos(t *testing.T, s *Server, uri, pointer string) protocol.Position {
	t.Helper()
	doc := s.doc(uri)
	for _, r := range doc.Refs {
		if r.Pointer == pointer {
			return protocol.Position{Line: r.Range.Start.Line, Character: r.Range.Start.Character + 2}
		}
	}
	t.Fatalf("no ref to %s", pointer)
	return protocol.Position{}
}

func TestDefinitionInternal(t *testing.T) {
	s := NewServer()
	uri := openDoc(t, s, "../../testdata/petstore.yaml")

	res, err := s.definition(&glsp.Context{}, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     refPos(t, s, uri, "/components/schemas/User"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	locs, ok := res.([]protocol.Location)
	if !ok || len(locs) != 1 {
		t.Fatalf("definition = %#v", res)
	}
	if locs[0].Range.Start.Line != 30 { // User: key, 0-based
		t.Errorf("definition line = %d, want 30", locs[0].Range.Start.Line)
	}
}

func TestReferencesInternal(t *testing.T) {
	s := NewServer()
	uri := openDoc(t, s, "../../testdata/petstore.yaml")

	// Cursor on the Pet component key.
	doc := s.doc(uri)
	var petKey protocol.Position
	for _, c := range doc.Components {
		if c.Name == "Pet" {
			petKey = c.KeyRange.Start
		}
	}
	locs, err := s.references(&glsp.Context{}, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     petKey,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(locs) != 2 {
		t.Errorf("references to Pet = %d, want 2", len(locs))
	}
}

func TestDocumentSymbolTree(t *testing.T) {
	s := NewServer()
	uri := openDoc(t, s, "../../testdata/petstore.yaml")

	res, err := s.documentSymbol(&glsp.Context{}, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		t.Fatal(err)
	}
	syms, ok := res.([]protocol.DocumentSymbol)
	if !ok || len(syms) != 2 {
		t.Fatalf("documentSymbol top-level = %#v", res)
	}
	if syms[0].Name != "Paths" || len(syms[0].Children) != 3 {
		t.Errorf("Paths children = %d, want 3", len(syms[0].Children))
	}
	if syms[1].Name != "Components" {
		t.Errorf("second symbol = %q, want Components", syms[1].Name)
	}
}

func TestCrossFileDefinitionAndReferences(t *testing.T) {
	s := NewServer()
	root, _ := filepath.Abs("../../testdata/multifile")
	s.ws = spec.NewWorkspace([]string{root})
	s.ws.Index()
	uri := openDoc(t, s, "../../testdata/multifile/openapi.yaml")

	// Definition: $ref './schemas/User.yaml#/User' jumps into the other file.
	res, err := s.definition(&glsp.Context{}, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     refPos(t, s, uri, "/User"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	locs, ok := res.([]protocol.Location)
	if !ok || len(locs) != 1 {
		t.Fatalf("cross-file definition = %#v", res)
	}
	if filepath.Base(spec.URIToPath(locs[0].URI)) != "User.yaml" {
		t.Errorf("definition file = %s, want User.yaml", spec.URIToPath(locs[0].URI))
	}

	// References from the User definition should find all 3 cross-file refs.
	userDoc := s.ws.GetOrLoad(filepath.Join(root, "schemas", "User.yaml"))
	refs, err := s.references(&glsp.Context{}, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: userDoc.URI},
			Position:     mustComponentKey(t, userDoc, "/User"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 {
		t.Errorf("cross-file references = %d, want 3", len(refs))
	}
}

func TestWorkspaceSymbolFromFragment(t *testing.T) {
	s := NewServer()
	root, _ := filepath.Abs("../../testdata/multifile")
	s.ws = spec.NewWorkspace([]string{root})
	s.ws.Index()

	syms, err := s.workspaceSymbol(&glsp.Context{}, &protocol.WorkspaceSymbolParams{Query: "User"})
	if err != nil {
		t.Fatal(err)
	}

	// "User" is a top-level key in schemas/User.yaml (not a spec, not under
	// components:), reachable only via $ref. It must still show up, located in
	// User.yaml — this is the regression the reachability walk fixes.
	var found *protocol.SymbolInformation
	for i := range syms {
		if syms[i].Name == "User" && filepath.Base(spec.URIToPath(syms[i].Location.URI)) == "User.yaml" {
			found = &syms[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no workspace symbol 'User' in User.yaml; got %#v", syms)
	}
}

// symbolIn reports whether syms contains one named name whose location file
// base matches fileBase.
func symbolIn(syms []protocol.SymbolInformation, name, fileBase string) bool {
	for _, s := range syms {
		if s.Name == name && filepath.Base(spec.URIToPath(s.Location.URI)) == fileBase {
			return true
		}
	}
	return false
}

func TestWorkspaceSymbolsSplitSpec(t *testing.T) {
	s := NewServer()
	root, _ := filepath.Abs("../../testdata/split")
	s.ws = spec.NewWorkspace([]string{root})
	s.ws.Index()

	syms, err := s.workspaceSymbol(&glsp.Context{}, &protocol.WorkspaceSymbolParams{})
	if err != nil {
		t.Fatal(err)
	}

	// Operations behind a path-item $ref, expanded from the fragment.
	if !symbolIn(syms, "GET /users", "users.yaml") {
		t.Errorf("missing 'GET /users' in users.yaml; got %v", names(syms))
	}
	if !symbolIn(syms, "POST /users", "users.yaml") {
		t.Errorf("missing 'POST /users' in users.yaml; got %v", names(syms))
	}
	// Whole-file schema ref: named after the file, located there.
	if !symbolIn(syms, "User", "User.yaml") {
		t.Errorf("missing 'User' in User.yaml; got %v", names(syms))
	}
	// Orphan components: block, never $ref'd, still surfaced.
	if !symbolIn(syms, "Money", "common.yaml") {
		t.Errorf("missing orphan 'Money' in common.yaml; got %v", names(syms))
	}
	// The fragment file must not leak as a schema-like symbol named "users".
	if symbolIn(syms, "users", "users.yaml") {
		t.Errorf("path-item fragment leaked as schema symbol 'users'")
	}

	// operationId + summary show as secondary text, and operationId is searchable.
	var getUsers *protocol.SymbolInformation
	for i := range syms {
		if syms[i].Name == "GET /users" {
			getUsers = &syms[i]
		}
	}
	if getUsers == nil || getUsers.ContainerName == nil ||
		*getUsers.ContainerName != "listUsers — List users" {
		t.Errorf("GET /users secondary text = %v, want 'listUsers — List users'", getUsers)
	}
	hits, _ := s.workspaceSymbol(&glsp.Context{}, &protocol.WorkspaceSymbolParams{Query: "listUsers"})
	if !symbolIn(hits, "GET /users", "users.yaml") {
		t.Errorf("operationId 'listUsers' not searchable; got %v", names(hits))
	}
}

func TestDocumentSymbolFragment(t *testing.T) {
	s := NewServer()
	uri := openDoc(t, s, "../../testdata/split/schemas/User.yaml")

	res, err := s.documentSymbol(&glsp.Context{}, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		t.Fatal(err)
	}
	syms, ok := res.([]protocol.DocumentSymbol)
	if !ok || len(syms) != 1 || syms[0].Name != "User" {
		t.Fatalf("fragment documentSymbol = %#v, want single 'User'", res)
	}
}

func names(syms []protocol.SymbolInformation) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.Name
	}
	return out
}

func mustComponentKey(t *testing.T, doc *spec.Document, pointer string) protocol.Position {
	t.Helper()
	rng, ok := doc.Resolve(pointer)
	if !ok {
		t.Fatalf("cannot resolve %s", pointer)
	}
	return rng.Start
}
