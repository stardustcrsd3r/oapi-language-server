package spec

import (
	"path/filepath"
	"testing"
)

func TestMultifileResolution(t *testing.T) {
	root, _ := filepath.Abs("../../testdata/multifile")
	ws := NewWorkspace([]string{root})
	ws.Index()

	mainPath := filepath.Join(root, "openapi.yaml")
	userPath := filepath.Join(root, "schemas", "User.yaml")

	// External definition: ./schemas/User.yaml#/User from the main file.
	loc, ok := ws.ResolveExternal(mainPath, Ref{File: "./schemas/User.yaml", Pointer: "/User"})
	if !ok {
		t.Fatal("could not resolve external ref")
	}
	if URIToPath(loc.URI) != userPath {
		t.Errorf("resolved URI = %s, want %s", URIToPath(loc.URI), userPath)
	}
	if loc.Range.Start.Line != 0 { // User: is the first line of User.yaml
		t.Errorf("User key line = %d, want 0", loc.Range.Start.Line)
	}

	// Cross-file references: 3 $refs in openapi.yaml point at User.yaml#/User.
	refs := ws.ExternalRefsTo(userPath, "/User")
	if len(refs) != 3 {
		t.Errorf("external refs to User = %d, want 3", len(refs))
	}
}
