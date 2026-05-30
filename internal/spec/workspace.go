package spec

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Workspace holds every parsed YAML document keyed by absolute path. Open
// documents (fed by the editor) take precedence over on-disk versions.
type Workspace struct {
	roots []string

	mu   sync.RWMutex
	docs map[string]*Document // abs path -> document
	open map[string]bool      // abs path -> opened in editor
}

// NewWorkspace creates a workspace rooted at the given directories.
func NewWorkspace(roots []string) *Workspace {
	return &Workspace{
		roots: roots,
		docs:  map[string]*Document{},
		open:  map[string]bool{},
	}
}

// Index walks the workspace roots and parses every YAML file it finds. Safe to
// call in a goroutine; it skips files already open in the editor.
func (w *Workspace) Index() {
	for _, root := range w.roots {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if de.IsDir() {
				switch de.Name() {
				case ".git", "node_modules", "vendor", ".idea", ".vscode":
					return filepath.SkipDir
				}
				return nil
			}
			if !isYAML(path) {
				return nil
			}
			abs, _ := filepath.Abs(path)
			w.mu.RLock()
			_, known := w.docs[abs]
			w.mu.RUnlock()
			if known {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			doc, _ := Parse(pathToURI(abs), src)
			w.mu.Lock()
			if !w.open[abs] {
				w.docs[abs] = doc
			}
			w.mu.Unlock()
			return nil
		})
	}
}

// Put stores (or replaces) an editor-open document.
func (w *Workspace) Put(path string, doc *Document) {
	abs, _ := filepath.Abs(path)
	w.mu.Lock()
	w.docs[abs] = doc
	w.open[abs] = true
	w.mu.Unlock()
}

// Close marks a document as no longer open; its last-parsed version is kept for
// cross-file resolution.
func (w *Workspace) Close(path string) {
	abs, _ := filepath.Abs(path)
	w.mu.Lock()
	delete(w.open, abs)
	w.mu.Unlock()
}

// Get returns a parsed document by path, if known.
func (w *Workspace) Get(path string) *Document {
	abs, _ := filepath.Abs(path)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docs[abs]
}

// GetOrLoad returns a document, parsing it from disk on demand.
func (w *Workspace) GetOrLoad(path string) *Document {
	abs, _ := filepath.Abs(path)
	w.mu.RLock()
	doc := w.docs[abs]
	w.mu.RUnlock()
	if doc != nil {
		return doc
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	doc, _ = Parse(pathToURI(abs), src)
	w.mu.Lock()
	if existing := w.docs[abs]; existing != nil {
		doc = existing
	} else {
		w.docs[abs] = doc
	}
	w.mu.Unlock()
	return doc
}

// SpecDocs returns all known documents that look like OpenAPI specs.
func (w *Workspace) SpecDocs() []*Document {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*Document, 0, len(w.docs))
	for _, d := range w.docs {
		if d.IsSpec {
			out = append(out, d)
		}
	}
	return out
}

// ResolveExternal resolves an external $ref from fromPath to a Location.
func (w *Workspace) ResolveExternal(fromPath string, ref Ref) (protocol.Location, bool) {
	target := resolveRefPath(fromPath, ref.File)
	doc := w.GetOrLoad(target)
	if doc == nil {
		return protocol.Location{}, false
	}
	if ref.Pointer == "" {
		// Whole-file reference: jump to the top of the file.
		return protocol.Location{URI: pathToURI(target)}, true
	}
	rng, ok := doc.Resolve(ref.Pointer)
	if !ok {
		return protocol.Location{}, false
	}
	return protocol.Location{URI: pathToURI(target), Range: rng}, true
}

// ExternalRefsTo returns Locations of $refs across the workspace that point at
// (targetPath, pointer) via an external file reference.
func (w *Workspace) ExternalRefsTo(targetPath, pointer string) []protocol.Location {
	absTarget, _ := filepath.Abs(targetPath)
	w.mu.RLock()
	docs := make([]*Document, 0, len(w.docs))
	for _, d := range w.docs {
		docs = append(docs, d)
	}
	w.mu.RUnlock()

	var out []protocol.Location
	for _, d := range docs {
		from := uriToPath(d.URI)
		for _, r := range d.Refs {
			if r.File == "" || r.Pointer != pointer {
				continue
			}
			if rp := resolveRefPath(from, r.File); rp == absTarget {
				out = append(out, protocol.Location{URI: d.URI, Range: r.Range})
			}
		}
	}
	return out
}

// ResolveRefPath resolves an external ref file (relative to fromPath) to an
// absolute path.
func ResolveRefPath(fromPath, file string) string { return resolveRefPath(fromPath, file) }

func resolveRefPath(fromPath, file string) string {
	if filepath.IsAbs(file) {
		abs, _ := filepath.Abs(file)
		return abs
	}
	abs, _ := filepath.Abs(filepath.Join(filepath.Dir(fromPath), file))
	return abs
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// pathToURI converts an absolute filesystem path to a file:// URI.
func pathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file://" + abs
}

// uriToPath converts a file:// URI back to a filesystem path.
func uriToPath(uri string) string {
	s := strings.TrimPrefix(uri, "file://")
	if unescaped, err := url.PathUnescape(s); err == nil {
		return unescaped
	}
	return s
}

// URIToPath is the exported form of uriToPath for use by the LSP layer.
func URIToPath(uri string) string { return uriToPath(uri) }
