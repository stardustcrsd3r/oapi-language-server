package spec

import protocol "github.com/tliron/glsp/protocol_3_16"

// Operation is a single METHOD /path entry under paths.
type Operation struct {
	Method      string // upper-cased HTTP method
	Path        string
	OperationID string
	Summary     string
	KeyRange    protocol.Range // range of the method key
}

// Component is a components.<kind>.<name> definition.
type Component struct {
	Kind     string
	Name     string
	Pointer  string         // e.g. "/components/schemas/User"
	KeyRange protocol.Range // range of the name key
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
