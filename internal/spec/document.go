package spec

import (
	"bytes"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/stardustcrsd3r/oapi-language-server/internal/yamlpos"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// Document is a parsed YAML file with an OpenAPI navigation index.
type Document struct {
	URI    string
	IsSpec bool // true when the file looks like an OpenAPI/Swagger spec

	src  []byte
	conv *yamlpos.Converter
	root ast.Node

	Operations []Operation
	Components []Component
	Refs       []RefUse

	targets map[string]protocol.Range // pointer -> definition key range
}

// Parse parses src and builds the navigation index. On a YAML syntax error it
// returns a Document with an empty index plus the error, so callers can keep a
// previous good index if they prefer.
func Parse(uri string, src []byte) (*Document, error) {
	d := &Document{
		URI:     uri,
		IsSpec:  IsOpenAPI(src, defaultDetectLines),
		src:     src,
		conv:    yamlpos.NewConverter(src),
		targets: map[string]protocol.Range{},
	}
	f, err := parser.ParseBytes(src, 0)
	if err != nil {
		return d, err
	}
	if len(f.Docs) == 0 || f.Docs[0].Body == nil {
		return d, nil
	}
	d.root = f.Docs[0].Body
	d.index()
	return d, nil
}

func (d *Document) index() {
	if paths := findKey(d.root, "paths"); paths != nil {
		for _, pmv := range mapValues(paths.Value) {
			path := keyString(pmv)
			if !strings.HasPrefix(path, "/") {
				continue
			}
			for _, mmv := range mapValues(pmv.Value) {
				method := keyString(mmv)
				if !httpMethods[strings.ToLower(method)] {
					continue
				}
				d.Operations = append(d.Operations, Operation{
					Method:      strings.ToUpper(method),
					Path:        path,
					OperationID: scalarChild(mmv.Value, "operationId"),
					Summary:     scalarChild(mmv.Value, "summary"),
					KeyRange:    d.nodeRange(mmv.Key),
					PathRange:   d.nodeRange(pmv.Key),
				})
			}
		}
	}

	if comps := findKey(d.root, "components"); comps != nil {
		for _, kmv := range mapValues(comps.Value) {
			kind := keyString(kmv)
			for _, nmv := range mapValues(kmv.Value) {
				name := keyString(nmv)
				pointer := "/components/" + kind + "/" + name
				rng := d.nodeRange(nmv.Key)
				d.Components = append(d.Components, Component{
					Kind: kind, Name: name, Pointer: pointer, KeyRange: rng,
				})
				d.targets[pointer] = rng
			}
		}
	}

	ast.Walk(visitFunc(func(n ast.Node) {
		mv, ok := n.(*ast.MappingValueNode)
		if !ok || keyString(mv) != "$ref" {
			return
		}
		s, ok := mv.Value.(*ast.StringNode)
		if !ok {
			return
		}
		ref := ParseRef(s.Value)
		d.Refs = append(d.Refs, RefUse{
			Raw: s.Value, File: ref.File, Pointer: ref.Pointer,
			Range: d.valueRange(mv.Value),
		})
	}), d.root)
}

// ContextAt resolves the navigation context at an LSP position.
func (d *Document) ContextAt(p protocol.Position) Context {
	for i := range d.Refs {
		if yamlpos.InRange(d.Refs[i].Range, p) {
			r := &d.Refs[i]
			return Context{Kind: ContextRef, Pointer: r.Pointer, File: r.File, Ref: r}
		}
	}
	for i := range d.Components {
		if yamlpos.InRange(d.Components[i].KeyRange, p) {
			return Context{Kind: ContextComponent, Pointer: d.Components[i].Pointer}
		}
	}
	return Context{Kind: ContextNone}
}

// PointerAt returns the JSON pointer of the mapping key at an LSP position, or
// "" if the cursor is not on a key. This lets find-references work from any
// definition key, including in referenced files that are not specs themselves
// (e.g. a top-level schema in schemas/User.yaml).
func (d *Document) PointerAt(p protocol.Position) string {
	path := d.keyPathAt(d.root, p)
	if len(path) == 0 {
		return ""
	}
	return "/" + strings.Join(path, "/")
}

func (d *Document) keyPathAt(n ast.Node, p protocol.Position) []string {
	for _, mv := range mapValues(n) {
		key := keyString(mv)
		if yamlpos.InRange(d.nodeRange(mv.Key), p) {
			return []string{key}
		}
		if mv.Value != nil {
			if sub := d.keyPathAt(mv.Value, p); sub != nil {
				return append([]string{key}, sub...)
			}
		}
	}
	return nil
}

// Resolve returns the definition range for an internal JSON pointer.
func (d *Document) Resolve(pointer string) (protocol.Range, bool) {
	if r, ok := d.targets[pointer]; ok {
		return r, true
	}
	if mv := resolvePointer(d.root, PointerSegments(pointer)); mv != nil {
		return d.nodeRange(mv.Key), true
	}
	return protocol.Range{}, false
}

// InternalRefsTo returns the ranges of internal $refs in this document that
// point at the given pointer.
func (d *Document) InternalRefsTo(pointer string) []protocol.Range {
	var out []protocol.Range
	for _, r := range d.Refs {
		if r.File == "" && r.Pointer == pointer {
			out = append(out, r.Range)
		}
	}
	return out
}

// tokenByteRange computes the [start,end) byte offsets of a token's value by
// locating it on its (1-based) source line. goccy reports a reliable Line but
// an unreliable Column/Offset for multibyte (non-ASCII) text — it mixes rune
// and byte counts — so we search the line for the value text rather than trust
// the reported column.
func (d *Document) tokenByteRange(n ast.Node) (int, int) {
	tok := n.GetToken()
	line := tok.Position.Line - 1 // goccy Line is 1-based
	lineStart := d.conv.LineStart(line)
	lineText := d.conv.LineText(line)
	val := []byte(tok.Value)

	col := bytes.Index(lineText, val)
	if col < 0 {
		// Fallback: span the line from its first non-space character.
		indent := len(lineText) - len(bytes.TrimLeft(lineText, " \t"))
		return lineStart + indent, lineStart + len(lineText)
	}
	return lineStart + col, lineStart + col + len(val)
}

func (d *Document) nodeRange(n ast.Node) protocol.Range {
	start, end := d.tokenByteRange(n)
	return d.conv.Range(start, end)
}

// valueRange is like nodeRange but expands to include surrounding quote
// characters, so the cursor still hits when placed on a quote.
func (d *Document) valueRange(n ast.Node) protocol.Range {
	start, end := d.tokenByteRange(n)
	if start > 0 && isQuote(d.src[start-1]) {
		start--
	}
	if end < len(d.src) && isQuote(d.src[end]) {
		end++
	}
	return d.conv.Range(start, end)
}

func isQuote(b byte) bool { return b == '\'' || b == '"' }

// --- AST helpers ---

type visitFunc func(ast.Node)

func (v visitFunc) Visit(n ast.Node) ast.Visitor { v(n); return v }

func mapValues(n ast.Node) []*ast.MappingValueNode {
	switch t := n.(type) {
	case *ast.MappingNode:
		return t.Values
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{t}
	case *ast.AnchorNode:
		return mapValues(t.Value)
	}
	return nil
}

func keyString(mv *ast.MappingValueNode) string {
	if mv == nil || mv.Key == nil {
		return ""
	}
	return mv.Key.GetToken().Value
}

func findKey(n ast.Node, key string) *ast.MappingValueNode {
	for _, mv := range mapValues(n) {
		if keyString(mv) == key {
			return mv
		}
	}
	return nil
}

func scalarChild(n ast.Node, key string) string {
	mv := findKey(n, key)
	if mv == nil || mv.Value == nil {
		return ""
	}
	return mv.Value.GetToken().Value
}

func resolvePointer(root ast.Node, segs []string) *ast.MappingValueNode {
	if len(segs) == 0 {
		return nil
	}
	cur := root
	var last *ast.MappingValueNode
	for _, seg := range segs {
		mv := findKey(cur, seg)
		if mv == nil {
			return nil
		}
		last, cur = mv, mv.Value
	}
	return last
}
