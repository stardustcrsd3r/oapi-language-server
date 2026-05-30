package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression for the "position lands N lines above" report on specs full of
// multibyte (Cyrillic) text. goccy reports token Offset/Column inconsistently
// (mixing rune and byte counts) but a reliable Line; positions are derived
// from Line + an in-line search, so they must stay correct regardless of
// non-ASCII content earlier in the file.
func TestCyrillicPositionsStayCorrect(t *testing.T) {
	var b strings.Builder
	b.WriteString("openapi: 3.0.3\ncomponents:\n  schemas:\n")
	for range 5 {
		b.WriteString("    # Описание справочника пользователя номер строки\n")
	}
	b.WriteString("    User:\n      type: object\n")
	src := b.String()

	// 0-based lines: 0 openapi, 1 components, 2 schemas, 3..7 comments, 8 User.
	doc, err := Parse("file:///c.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	rng, ok := doc.Resolve("/components/schemas/User")
	if !ok {
		t.Fatal("cannot resolve User")
	}
	if rng.Start.Line != 8 {
		t.Errorf("User line = %d, want 8", rng.Start.Line)
	}
	// "    User:" -> U at column 4.
	if rng.Start.Character != 4 {
		t.Errorf("User character = %d, want 4", rng.Start.Character)
	}
}

func line(lines []string, n int) string {
	if n < 0 || n >= len(lines) {
		return "<out of range>"
	}
	return strings.TrimSpace(lines[n])
}

// Drives a Cyrillic-heavy spec end to end: each indexed operation's method
// range must hit its method line and its path range must hit the parent path
// line (what the picker jumps to), and internal $refs must resolve.
func TestCyrillicSpecPositions(t *testing.T) {
	path, _ := filepath.Abs("../../testdata/cyrillic.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse("file://"+path, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Operations) == 0 {
		t.Fatal("no operations indexed")
	}
	lines := strings.Split(string(src), "\n")
	for _, op := range doc.Operations {
		// KeyRange points at the method key (get:/post:/…).
		mln := int(op.KeyRange.Start.Line)
		if mln >= len(lines) || !strings.Contains(strings.ToLower(lines[mln]), strings.ToLower(op.Method)+":") {
			t.Errorf("%s %s: method line %d = %q", op.Method, op.Path, mln, line(lines, mln))
		}
		// PathRange points at the parent path key (/...:).
		pln := int(op.PathRange.Start.Line)
		if pln >= len(lines) || !strings.Contains(lines[pln], op.Path) {
			t.Errorf("%s %s: path line %d = %q does not contain path", op.Method, op.Path, pln, line(lines, pln))
		}
	}
	for _, r := range doc.Refs {
		if r.File == "" && r.Pointer != "" {
			if _, ok := doc.Resolve(r.Pointer); !ok {
				t.Errorf("internal ref %q does not resolve", r.Pointer)
			}
		}
	}
}
