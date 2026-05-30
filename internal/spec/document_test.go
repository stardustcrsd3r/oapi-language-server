package spec

import (
	"os"
	"path/filepath"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func loadPetstore(t *testing.T) *Document {
	t.Helper()
	path, _ := filepath.Abs("../../testdata/petstore.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse("file://"+path, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func TestIndexCounts(t *testing.T) {
	doc := loadPetstore(t)
	if !doc.IsSpec {
		t.Error("expected IsSpec=true")
	}
	if len(doc.Operations) != 3 {
		t.Errorf("operations = %d, want 3", len(doc.Operations))
	}
	// schemas: User, Pet; parameters: IdParam; responses: NotFound
	if len(doc.Components) != 4 {
		t.Errorf("components = %d, want 4", len(doc.Components))
	}
	// 3 $ref uses: User->Pet, op->User, op->Pet
	if len(doc.Refs) != 3 {
		t.Errorf("refs = %d, want 3", len(doc.Refs))
	}
}

func TestOperationFields(t *testing.T) {
	doc := loadPetstore(t)
	var found bool
	for _, op := range doc.Operations {
		if op.Method == "GET" && op.Path == "/users/{id}" {
			found = true
			if op.OperationID != "getUser" {
				t.Errorf("operationId = %q, want getUser", op.OperationID)
			}
			if op.Summary != "Get a user by ID" {
				t.Errorf("summary = %q", op.Summary)
			}
		}
	}
	if !found {
		t.Error("GET /users/{id} not indexed")
	}
}

func TestResolveAndContext(t *testing.T) {
	doc := loadPetstore(t)

	// Resolve internal pointer to the User schema key.
	rng, ok := doc.Resolve("/components/schemas/User")
	if !ok {
		t.Fatal("could not resolve User")
	}
	// User: is on line 31 (1-based) -> line 30 (0-based).
	if rng.Start.Line != 30 {
		t.Errorf("User key line = %d, want 30", rng.Start.Line)
	}

	// ContextAt on a $ref value should classify as a ref.
	var refRange protocol.Range
	for _, r := range doc.Refs {
		if r.Pointer == "/components/schemas/User" {
			refRange = r.Range
		}
	}
	mid := protocol.Position{Line: refRange.Start.Line, Character: refRange.Start.Character + 2}
	c := doc.ContextAt(mid)
	if c.Kind != ContextRef || c.Pointer != "/components/schemas/User" {
		t.Errorf("ContextAt on ref = %+v", c)
	}

	// ContextAt on the User component key should classify as a component.
	cc := doc.ContextAt(protocol.Position{Line: rng.Start.Line, Character: rng.Start.Character})
	if cc.Kind != ContextComponent || cc.Pointer != "/components/schemas/User" {
		t.Errorf("ContextAt on component = %+v", cc)
	}
}

func TestInternalRefsTo(t *testing.T) {
	doc := loadPetstore(t)
	pet := doc.InternalRefsTo("/components/schemas/Pet")
	if len(pet) != 2 { // op->Pet and User.properties.pet->Pet
		t.Errorf("refs to Pet = %d, want 2", len(pet))
	}
}
