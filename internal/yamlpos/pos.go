// Package yamlpos converts byte offsets in YAML source into LSP positions.
//
// LSP positions are zero-based, and the character field counts UTF-16 code
// units (not bytes or runes). goccy/go-yaml reports token positions as
// zero-based byte offsets, so we translate offset -> {line, utf16-character}.
package yamlpos

import (
	"sort"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Converter maps byte offsets in a source buffer to LSP positions.
type Converter struct {
	src        []byte
	lineStarts []int // byte offset of the first byte of each line
}

// NewConverter builds a line index over src.
func NewConverter(src []byte) *Converter {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &Converter{src: src, lineStarts: starts}
}

// Position returns the LSP position for a byte offset.
func (c *Converter) Position(off int) protocol.Position {
	if off < 0 {
		off = 0
	}
	if off > len(c.src) {
		off = len(c.src)
	}
	// Largest line index whose start is <= off.
	idx := sort.Search(len(c.lineStarts), func(i int) bool {
		return c.lineStarts[i] > off
	}) - 1
	if idx < 0 {
		idx = 0
	}
	char := utf16Len(c.src[c.lineStarts[idx]:off])
	return protocol.Position{Line: uint32(idx), Character: uint32(char)}
}

// Range returns the LSP range spanning [start, end) byte offsets.
func (c *Converter) Range(start, end int) protocol.Range {
	return protocol.Range{Start: c.Position(start), End: c.Position(end)}
}

func utf16Len(b []byte) int {
	n := 0
	for _, r := range string(b) {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// InRange reports whether p lies within r (inclusive of both ends).
func InRange(r protocol.Range, p protocol.Position) bool {
	return !before(p, r.Start) && !before(r.End, p)
}

func before(a, b protocol.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}
