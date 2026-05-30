package spec

import (
	"path/filepath"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// RefTarget is a definition reachable via $ref from a spec, surfaced as a
// workspace symbol even when it lives in a file that is not itself a spec
// (e.g. a bare schemas/User.yaml fragment whose definitions are top-level keys,
// not entries under components:). Targets in files that are themselves specs
// are not returned — their symbols already come from the spec's own
// Operations/Components.
type RefTarget struct {
	Name  string         // last pointer segment, or the file stem for whole-file refs
	Kind  string         // inferred component kind ("schemas", …) or parent dir name
	URI   string         // file:// URI of the definition
	Range protocol.Range // range of the definition key (zero for whole-file refs)
}

// ReachableTargets walks the $ref graph starting from every known spec and
// returns the external definitions those specs point at, transitively. It is
// the source of workspace symbols for split specs whose schemas live in
// non-spec fragment files.
func (w *Workspace) ReachableTargets() []RefTarget {
	var queue []string
	for _, d := range w.SpecDocs() {
		queue = append(queue, uriToPath(d.URI))
	}

	visited := map[string]bool{} // files already walked
	seen := map[string]bool{}    // "<path>#<pointer>" targets already emitted
	var out []RefTarget

	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		absFrom, _ := filepath.Abs(from)
		if visited[absFrom] {
			continue
		}
		visited[absFrom] = true

		doc := w.GetOrLoad(absFrom)
		if doc == nil {
			continue
		}
		for _, r := range doc.Refs {
			if r.File == "" {
				continue // internal ref: covered by the source file's own symbols
			}
			tp := resolveRefPath(absFrom, r.File)
			if !visited[tp] {
				queue = append(queue, tp) // follow transitive refs in the fragment
			}
			key := tp + "#" + r.Pointer
			if seen[key] {
				continue
			}
			seen[key] = true
			if t, ok := w.makeTarget(tp, r.Pointer); ok {
				out = append(out, t)
			}
		}
	}
	return out
}

// makeTarget resolves a (path, pointer) ref destination into a navigable
// symbol. It returns false when the target file is itself a spec (already
// covered elsewhere), cannot be loaded, or the pointer is dangling.
func (w *Workspace) makeTarget(path, pointer string) (RefTarget, bool) {
	doc := w.GetOrLoad(path)
	if doc == nil || doc.IsSpec {
		return RefTarget{}, false
	}
	if pointer == "" && doc.IsPathItem() {
		// A whole-file path item — its symbols are the operations, expanded from
		// the referencing spec's PathRefs, not a single schema-like symbol.
		return RefTarget{}, false
	}
	var rng protocol.Range
	if pointer != "" {
		r, ok := doc.Resolve(pointer)
		if !ok {
			return RefTarget{}, false
		}
		rng = r
	}
	return RefTarget{
		Name:  targetName(path, pointer),
		Kind:  targetKind(path, pointer),
		URI:   doc.URI,
		Range: rng,
	}, true
}

// OrphanComponents returns components defined in non-spec files (an explicit
// components: block, e.g. a shared components.yaml) regardless of whether any
// $ref targets them, so a schema can be found before the first ref is written.
// Spec files are excluded — their components come from the spec symbol pass.
func (w *Workspace) OrphanComponents() []RefTarget {
	w.mu.RLock()
	docs := make([]*Document, 0, len(w.docs))
	for _, d := range w.docs {
		docs = append(docs, d)
	}
	w.mu.RUnlock()

	var out []RefTarget
	for _, d := range docs {
		if d.IsSpec {
			continue
		}
		for _, c := range d.Components {
			out = append(out, RefTarget{
				Name: c.Name, Kind: c.Kind, URI: d.URI, Range: c.KeyRange,
			})
		}
	}
	return out
}

// targetName is the last pointer segment, or the file's base name (without
// extension) for a whole-file reference.
func targetName(path, pointer string) string {
	if segs := PointerSegments(pointer); len(segs) > 0 {
		return segs[len(segs)-1]
	}
	return fileStem(path)
}

// fileStem is the base name of a path or file:// URI without its extension.
func fileStem(pathOrURI string) string {
	base := filepath.Base(uriToPath(pathOrURI))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// targetKind infers a component kind for the symbol's icon/container: from a
// .../components/<kind>/<name> pointer when present, else from the parent
// directory name (schemas/, parameters/, …).
func targetKind(path, pointer string) string {
	segs := PointerSegments(pointer)
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == "components" {
			return segs[i+1]
		}
	}
	return filepath.Base(filepath.Dir(path))
}
