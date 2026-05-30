package lsp

import (
	"strconv"
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
	seen := map[string]bool{}

	// addDef emits a schema-like symbol, deduped by (file, name, position) so the
	// reachability and orphan passes don't double-list the same definition.
	addDef := func(name, kind, uri string, rng protocol.Range) {
		if !matchQuery(q, name) {
			return
		}
		key := uri + "\x00" + name + "\x00" +
			strconv.Itoa(int(rng.Start.Line)) + ":" + strconv.Itoa(int(rng.Start.Character))
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, protocol.SymbolInformation{
			Name: name, Kind: componentSymbolKind(kind),
			Location:      protocol.Location{URI: uri, Range: rng},
			ContainerName: strPtr(kind),
		})
	}

	// addOp emits an operation symbol with operationId/summary as secondary text
	// (ContainerName, which pickers render dimmed). operationId is searchable too.
	addOp := func(method, path, operationID, summary, uri string, rng protocol.Range) {
		name := method + " " + path
		if !matchQuery(q, name) && !matchQuery(q, operationID) {
			return
		}
		sym := protocol.SymbolInformation{
			Name: name, Kind: protocol.SymbolKindMethod,
			Location: protocol.Location{URI: uri, Range: rng},
		}
		if det := opDetail(operationID, summary); det != "" {
			sym.ContainerName = strPtr(det)
		}
		out = append(out, sym)
	}

	for _, d := range s.ws.SpecDocs() {
		for _, op := range d.Operations {
			addOp(op.Method, op.Path, op.OperationID, op.Summary, d.URI, op.PathRange)
		}
		// Operations behind a path-item-level $ref live in another file; expand
		// them so METHOD /path still shows up, located in the fragment.
		for _, pr := range d.PathRefs {
			target := spec.URIToPath(d.URI)
			if pr.File != "" {
				target = spec.ResolveRefPath(spec.URIToPath(d.URI), pr.File)
			}
			tdoc := s.ws.GetOrLoad(target)
			if tdoc == nil {
				continue
			}
			for _, me := range tdoc.PathItemMethods(pr.Pointer) {
				addOp(me.Method, pr.Path, me.OperationID, me.Summary, tdoc.URI, me.Range)
			}
		}
		for _, comp := range d.Components {
			addDef(comp.Name, comp.Kind, d.URI, comp.KeyRange)
		}
	}

	// Definitions in non-spec fragment files (e.g. schemas/User.yaml) are not in
	// any SpecDocs Components: reach them via the $ref graph, and surface
	// unreferenced shared components: blocks too.
	for _, t := range s.ws.ReachableTargets() {
		addDef(t.Name, t.Kind, t.URI, t.Range)
	}
	for _, t := range s.ws.OrphanComponents() {
		addDef(t.Name, t.Kind, t.URI, t.Range)
	}
	return out, nil
}

// --- helpers ---

func buildDocumentSymbols(doc *spec.Document) []protocol.DocumentSymbol {
	var syms []protocol.DocumentSymbol

	if len(doc.Operations) > 0 || len(doc.PathRefs) > 0 {
		children := make([]protocol.DocumentSymbol, 0, len(doc.Operations)+len(doc.PathRefs))
		for _, op := range doc.Operations {
			children = append(children, protocol.DocumentSymbol{
				Name:           op.Method + " " + op.Path,
				Detail:         detailPtr(op.OperationID, op.Summary),
				Kind:           protocol.SymbolKindMethod,
				Range:          op.PathRange,
				SelectionRange: op.PathRange,
			})
		}
		// Path items behind a $ref: show the path so the outline is complete; the
		// methods live in another file (jump via go-to-definition on the $ref).
		for _, pr := range doc.PathRefs {
			children = append(children, protocol.DocumentSymbol{
				Name:           pr.Path,
				Kind:           protocol.SymbolKindMethod,
				Range:          pr.PathRange,
				SelectionRange: pr.PathRange,
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

	// A non-spec fragment file (e.g. schemas/User.yaml) has no Paths/Components;
	// outline its top-level definitions so the symbol picker isn't empty.
	if len(syms) == 0 && !doc.IsSpec {
		defs := doc.TopLevelDefs()
		switch {
		case len(defs) == 1:
			c := defs[0]
			syms = append(syms, protocol.DocumentSymbol{
				Name: c.Name, Kind: componentSymbolKind(c.Kind),
				Range: c.KeyRange, SelectionRange: c.KeyRange,
			})
		case len(defs) > 1:
			children := make([]protocol.DocumentSymbol, 0, len(defs))
			for _, c := range defs {
				children = append(children, protocol.DocumentSymbol{
					Name: c.Name, Kind: componentSymbolKind(c.Kind),
					Range: c.KeyRange, SelectionRange: c.KeyRange,
				})
			}
			syms = append(syms, container("Definitions", children))
		}
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

// opDetail is the secondary text for an operation: "operationId — summary"
// (either part omitted when absent), or "" when neither is set.
func opDetail(operationID, summary string) string {
	parts := make([]string, 0, 2)
	if operationID != "" {
		parts = append(parts, operationID)
	}
	if summary != "" {
		parts = append(parts, summary)
	}
	return strings.Join(parts, " — ")
}

func detailPtr(operationID, summary string) *string {
	if d := opDetail(operationID, summary); d != "" {
		return strPtr(d)
	}
	return nil
}

func strPtr(s string) *string { return &s }

func matchQuery(query, name string) bool {
	return query == "" || strings.Contains(strings.ToLower(name), query)
}

func resolveFrom(fromPath, file string) string {
	return spec.ResolveRefPath(fromPath, file)
}
