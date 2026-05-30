package spec

import (
	"bufio"
	"bytes"
	"regexp"
)

const defaultDetectLines = 50

var openapiKey = regexp.MustCompile(`^\s*(openapi|swagger)\s*:`)

// IsOpenAPI reports whether src looks like an OpenAPI/Swagger spec: a YAML
// document whose first detectLines lines contain a top-level openapi:/swagger:
// key.
func IsOpenAPI(src []byte, detectLines int) bool {
	if detectLines <= 0 {
		detectLines = defaultDetectLines
	}
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < detectLines && sc.Scan(); i++ {
		if openapiKey.Match(sc.Bytes()) {
			return true
		}
	}
	return false
}
