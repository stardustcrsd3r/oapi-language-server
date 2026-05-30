package spec

import "strings"

// Ref is a parsed $ref value, split into its file and JSON-pointer parts.
//
//	"#/components/schemas/User" -> {Pointer: "/components/schemas/User"}
//	"./schemas.yaml#/User"      -> {File: "./schemas.yaml", Pointer: "/User"}
//	"./schemas/User.yaml"       -> {File: "./schemas/User.yaml"}
type Ref struct {
	Value   string
	File    string // external file part; empty for internal refs
	Pointer string // JSON pointer incl. leading '/'; empty if none
}

// ParseRef splits a raw $ref string into file + pointer parts.
func ParseRef(value string) Ref {
	value = strings.TrimSpace(value)
	hash := strings.IndexByte(value, '#')
	if hash < 0 {
		// No '#': the whole value is a file reference.
		return Ref{Value: value, File: value}
	}
	return Ref{Value: value, File: value[:hash], Pointer: value[hash+1:]}
}

// IsInternal reports whether the ref points within the current document.
func (r Ref) IsInternal() bool { return r.File == "" }

// PointerSegments splits a JSON pointer into its decoded path segments
// (RFC 6901: ~1 -> '/', ~0 -> '~').
func PointerSegments(pointer string) []string {
	pointer = strings.TrimPrefix(pointer, "/")
	if pointer == "" {
		return nil
	}
	parts := strings.Split(pointer, "/")
	for i := range parts {
		parts[i] = decodeToken(parts[i])
	}
	return parts
}

func decodeToken(t string) string {
	// Order matters: ~1 before ~0 per RFC 6901.
	t = strings.ReplaceAll(t, "~1", "/")
	return strings.ReplaceAll(t, "~0", "~")
}
