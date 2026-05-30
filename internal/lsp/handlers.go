package lsp

import (
	"strings"

	"github.com/stardustcrsd3r/oapi-language-server/internal/spec"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) definition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	doc := s.doc(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	c := doc.ContextAt(params.Position)
	if c.Kind != spec.ContextRef || c.Ref == nil {
		return nil, nil
	}
	if c.Ref.File == "" {
		if rng, ok := doc.Resolve(c.Ref.Pointer); ok {
			return []protocol.Location{{URI: doc.URI, Range: rng}}, nil
		}
		return nil, nil
	}
	loc, ok := s.ws.ResolveExternal(spec.URIToPath(doc.URI), spec.Ref{
		Value: c.Ref.Raw, File: c.Ref.File, Pointer: c.Ref.Pointer,
	})
	if !ok {
		return nil, nil
	}
	return []protocol.Location{loc}, nil
}

func (s *Server) references(ctx *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	doc := s.doc(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	c := doc.ContextAt(params.Position)

	// Canonical target: a (path, pointer) pair. For an internal context the
	// target lives in this file; for an external ref it lives elsewhere.
	pointer := c.Pointer
	targetPath := spec.URIToPath(doc.URI)
	if c.File != "" {
		targetPath = resolveFrom(spec.URIToPath(doc.URI), c.File)
	}
	if pointer == "" {
		// Not on a ref or a known component: fall back to the generic key path,
		// so references work from any definition key (incl. referenced files).
		pointer = doc.PointerAt(params.Position)
	}
	if pointer == "" {
		return nil, nil
	}

	var out []protocol.Location
	if td := s.ws.GetOrLoad(targetPath); td != nil {
		for _, rng := range td.InternalRefsTo(pointer) {
			out = append(out, protocol.Location{URI: td.URI, Range: rng})
		}
	}
	out = append(out, s.ws.ExternalRefsTo(targetPath, pointer)...)
	return out, nil
}

func (s *Server) documentSymbol(ctx *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	doc := s.doc(params.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	return buildDocumentSymbols(doc), nil
}

func (s *Server) workspaceSymbol(ctx *glsp.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	q := strings.ToLower(params.Query)
	var out []protocol.SymbolInformation
	for _, d := range s.ws.SpecDocs() {
		for _, op := range d.Operations {
			name := op.Method + " " + op.Path
			if matchQuery(q, name) {
				out = append(out, protocol.SymbolInformation{
					Name: name, Kind: protocol.SymbolKindMethod,
					Location: protocol.Location{URI: d.URI, Range: op.KeyRange},
				})
			}
		}
		for _, comp := range d.Components {
			if matchQuery(q, comp.Name) {
				out = append(out, protocol.SymbolInformation{
					Name: comp.Name, Kind: componentSymbolKind(comp.Kind),
					Location:      protocol.Location{URI: d.URI, Range: comp.KeyRange},
					ContainerName: strPtr(comp.Kind),
				})
			}
		}
	}
	return out, nil
}

// --- helpers ---

func buildDocumentSymbols(doc *spec.Document) []protocol.DocumentSymbol {
	var syms []protocol.DocumentSymbol

	if len(doc.Operations) > 0 {
		children := make([]protocol.DocumentSymbol, 0, len(doc.Operations))
		for _, op := range doc.Operations {
			children = append(children, protocol.DocumentSymbol{
				Name:           op.Method + " " + op.Path,
				Detail:         detailPtr(op.OperationID, op.Summary),
				Kind:           protocol.SymbolKindMethod,
				Range:          op.KeyRange,
				SelectionRange: op.KeyRange,
			})
		}
		syms = append(syms, container("Paths", children))
	}

	if len(doc.Components) > 0 {
		byKind := map[string][]protocol.DocumentSymbol{}
		var order []string
		for _, comp := range doc.Components {
			if _, ok := byKind[comp.Kind]; !ok {
				order = append(order, comp.Kind)
			}
			byKind[comp.Kind] = append(byKind[comp.Kind], protocol.DocumentSymbol{
				Name:           comp.Name,
				Kind:           componentSymbolKind(comp.Kind),
				Range:          comp.KeyRange,
				SelectionRange: comp.KeyRange,
			})
		}
		kinds := make([]protocol.DocumentSymbol, 0, len(order))
		for _, k := range order {
			kinds = append(kinds, container(k, byKind[k]))
		}
		syms = append(syms, container("Components", kinds))
	}

	return syms
}

func container(name string, children []protocol.DocumentSymbol) protocol.DocumentSymbol {
	rng := children[0].Range
	for _, c := range children[1:] {
		if before(c.Range.Start, rng.Start) {
			rng.Start = c.Range.Start
		}
		if before(rng.End, c.Range.End) {
			rng.End = c.Range.End
		}
	}
	return protocol.DocumentSymbol{
		Name:           name,
		Kind:           protocol.SymbolKindNamespace,
		Range:          rng,
		SelectionRange: rng,
		Children:       children,
	}
}

func before(a, b protocol.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}

func componentSymbolKind(kind string) protocol.SymbolKind {
	switch kind {
	case "schemas":
		return protocol.SymbolKindStruct
	case "parameters", "headers":
		return protocol.SymbolKindField
	case "securitySchemes":
		return protocol.SymbolKindKey
	default:
		return protocol.SymbolKindObject
	}
}

func detailPtr(operationID, summary string) *string {
	parts := make([]string, 0, 2)
	if operationID != "" {
		parts = append(parts, operationID)
	}
	if summary != "" {
		parts = append(parts, summary)
	}
	if len(parts) == 0 {
		return nil
	}
	return strPtr(strings.Join(parts, " — "))
}

func strPtr(s string) *string { return &s }

func matchQuery(query, name string) bool {
	return query == "" || strings.Contains(strings.ToLower(name), query)
}

func resolveFrom(fromPath, file string) string {
	return spec.ResolveRefPath(fromPath, file)
}
