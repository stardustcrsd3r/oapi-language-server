// Command oapi-lsp is a language server for OpenAPI/Swagger YAML specs.
//
// It provides go-to-definition and find-references on $ref (internal and
// cross-file), plus document and workspace symbols for operations and
// components. It is meant to run alongside yaml-language-server, which handles
// schema validation, completion and hover.
package main

import (
	"fmt"
	"os"

	"github.com/stardustcrsd3r/oapi-language-server/internal/lsp"
	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" || arg == "version" {
			fmt.Println(lsp.Name, lsp.Version)
			return
		}
	}

	// Quiet by default; logs would otherwise corrupt the stdio JSON-RPC stream
	// only if written to stdout — commonlog writes to stderr, so this is just
	// to keep things tidy.
	commonlog.Configure(0, nil)

	if err := lsp.NewServer().Run(); err != nil {
		fmt.Fprintln(os.Stderr, "oapi-lsp:", err)
		os.Exit(1)
	}
}
