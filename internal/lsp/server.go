// Package lsp wires the OpenAPI navigation index into a glsp language server.
package lsp

import (
	"encoding/json"

	"github.com/stardustcrsd3r/oapi-language-server/internal/spec"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspserver "github.com/tliron/glsp/server"
)

// Name is the server name reported to clients.
const Name = "oapi-language-server"

// Version is the server version reported to clients.
var Version = "0.1.0"

// Server holds the parsed-document workspace and the glsp handler.
type Server struct {
	ws      *spec.Workspace
	handler *protocol.Handler
}

// NewServer constructs a Server with all handlers wired.
func NewServer() *Server {
	s := &Server{ws: spec.NewWorkspace(nil)}
	s.handler = &protocol.Handler{
		Initialize:                 s.initialize,
		Initialized:                s.initialized,
		Shutdown:                   s.shutdown,
		SetTrace:                   s.setTrace,
		TextDocumentDidOpen:        s.didOpen,
		TextDocumentDidChange:      s.didChange,
		TextDocumentDidClose:       s.didClose,
		TextDocumentDefinition:     s.definition,
		TextDocumentReferences:     s.references,
		TextDocumentDocumentSymbol: s.documentSymbol,
		WorkspaceSymbol:            s.workspaceSymbol,
	}
	return s
}

// Run starts the server over stdio and blocks until the connection closes.
func (s *Server) Run() error {
	return glspserver.NewServer(s.handler, Name, false).RunStdio()
}

func (s *Server) initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	var roots []string
	for _, f := range params.WorkspaceFolders {
		roots = append(roots, spec.URIToPath(f.URI))
	}
	if len(roots) == 0 && params.RootURI != nil {
		roots = append(roots, spec.URIToPath(*params.RootURI))
	}
	s.ws = spec.NewWorkspace(roots)
	go s.ws.Index()

	capabilities := s.handler.CreateServerCapabilities()
	capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull
	capabilities.DefinitionProvider = true
	capabilities.ReferencesProvider = true
	capabilities.DocumentSymbolProvider = true
	capabilities.WorkspaceSymbolProvider = true

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name: Name, Version: &Version,
		},
	}, nil
}

func (s *Server) initialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (s *Server) shutdown(ctx *glsp.Context) error { return nil }

func (s *Server) setTrace(ctx *glsp.Context, params *protocol.SetTraceParams) error {
	return nil
}

// --- document lifecycle ---

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.store(params.TextDocument.URI, []byte(params.TextDocument.Text))
	return nil
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	if text, ok := wholeText(params.ContentChanges); ok {
		s.store(params.TextDocument.URI, text)
	}
	return nil
}

func (s *Server) didClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.ws.Close(spec.URIToPath(params.TextDocument.URI))
	return nil
}

// store parses src and replaces the document for uri in the workspace.
func (s *Server) store(uri string, src []byte) {
	doc, _ := spec.Parse(uri, src)
	s.ws.Put(spec.URIToPath(uri), doc)
}

func (s *Server) doc(uri string) *spec.Document {
	return s.ws.Get(spec.URIToPath(uri))
}

// wholeText extracts the full document text from a Full-sync change batch.
func wholeText(changes []any) ([]byte, bool) {
	if len(changes) == 0 {
		return nil, false
	}
	b, err := json.Marshal(changes[len(changes)-1])
	if err != nil {
		return nil, false
	}
	var ev struct {
		Range *protocol.Range `json:"range"`
		Text  string          `json:"text"`
	}
	if err := json.Unmarshal(b, &ev); err != nil || ev.Range != nil {
		return nil, false
	}
	return []byte(ev.Text), true
}
