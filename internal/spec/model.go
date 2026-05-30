package spec

import protocol "github.com/tliron/glsp/protocol_3_16"

// Operation is a single METHOD /path entry under paths.
type Operation struct {
	Method      string // upper-cased HTTP method
	Path        string
	OperationID string
	Summary     string
	KeyRange    protocol.Range // range of the method key (get:/post:/…)
	PathRange   protocol.Range // range of the parent path key (/users/{id}:)
}

// Component is a components.<kind>.<name> definition.
type Component struct {
	Kind     string
	Name     string
	Pointer  string         // e.g. "/components/schemas/User"
	KeyRange protocol.Range // range of the name key
}

// PathRef is a path-item-level $ref (paths./users: {$ref: ./paths/users.yaml}),
// where the whole path item — and thus its operations — lives in another file.
type PathRef struct {
	Path      string
	File      string         // external file part; empty for internal refs
	Pointer   string         // JSON pointer incl. leading '/'; empty for whole-file
	PathRange protocol.Range // range of the path key (/users:)
}

// MethodEntry is one HTTP-method key of a path item, with its key range.
type MethodEntry struct {
	Method      string // upper-cased HTTP method
	OperationID string
	Summary     string
	Range       protocol.Range
}

// RefUse is a single $ref occurrence.
type RefUse struct {
	Raw     string
	File    string         // external file part; empty for internal refs
	Pointer string         // JSON pointer incl. leading '/'
	Range   protocol.Range // range of the $ref value
}

// ContextKind classifies what the cursor is sitting on.
type ContextKind int

const (
	ContextNone ContextKind = iota
	ContextRef
	ContextComponent
)

// Context is the navigation target resolved at a cursor position.
type Context struct {
	Kind    ContextKind
	Pointer string  // JSON pointer of the target/definition
	File    string  // external file (for a ref) if any
	Ref     *RefUse // populated when Kind == ContextRef
}
