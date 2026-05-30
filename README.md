# oapi-language-server

A [Language Server](https://microsoft.github.io/language-server-protocol/) for
OpenAPI / Swagger YAML specs.

It provides the navigation that general YAML tooling doesn't understand: jumping
between `$ref` declarations and their definitions (internal **and** across
files), and listing operations and components through your editor's symbol
picker.

It does **not** do schema validation, completion or hover — those are best left
to a general YAML language server. Running one (e.g.
[`yaml-language-server`](https://github.com/redhat-developer/yaml-language-server))
alongside this one is recommended for the full experience, but not required:
`oapi-lsp` works on its own for `$ref` navigation and symbols.

## Features

- **Go to definition** on a `$ref` — internal `#/components/…` or cross-file
  `./schemas/User.yaml#/…`.
- **Find references** — from a definition key (or a `$ref`), every `$ref` that
  targets it, across the whole workspace.
- **Document symbols** — a *Paths* tree (`METHOD /path`) and a *Components* tree,
  for the outline and symbol picker.
- **Workspace symbols** — fuzzy-find every operation and component across all
  specs in the workspace.

Multi-file `$ref` works out of the box; referenced files need not be specs
themselves. A file is treated as a spec when one of its first lines has a
top-level `openapi:` or `swagger:` key.

## Quick start (Neovim, recommended)

With [lazy.nvim](https://github.com/folke/lazy.nvim), this single spec installs
the binary, rebuilds it on every plugin update, and attaches it to OpenAPI
buffers — nothing else to install or wire up. Requires a Go toolchain on
`PATH`.

```lua
-- ~/.config/nvim/lua/plugins/oapi-lsp.lua
return {
  "stardustcrsd3r/oapi-language-server",
  build = "go install ./cmd/oapi-lsp",
  ft = "yaml",
  config = function()
    vim.api.nvim_create_autocmd("FileType", {
      pattern = "yaml",
      callback = function(args)
        local head = vim.api.nvim_buf_get_lines(args.buf, 0, 20, false)
        local is_spec = false
        for _, l in ipairs(head) do
          if l:match("^%s*openapi%s*:") or l:match("^%s*swagger%s*:") then
            is_spec = true
            break
          end
        end
        if not is_spec then
          return
        end
        vim.lsp.start({
          name = "oapi-lsp",
          cmd = { vim.fn.expand("$HOME/go/bin/oapi-lsp") },
          root_dir = vim.fs.root(args.buf, { ".git" }) or vim.fn.getcwd(),
        }, { bufnr = args.buf })
      end,
    })
  end,
}
```

Keep your `yaml-language-server` setup as-is; both attach to the same buffer.

## Manual install

```sh
go install github.com/stardustcrsd3r/oapi-language-server/cmd/oapi-lsp@latest
```

This puts an `oapi-lsp` binary in `$(go env GOPATH)/bin` (usually `~/go/bin`) —
make sure that's on your `PATH`, then point any LSP client's command at
`oapi-lsp` for YAML files.

## Other editors

`oapi-lsp` speaks LSP over stdio, so any LSP client works — launch `oapi-lsp`
and route `yaml` documents to it. The features show up under each editor's
standard commands:

- **VS Code** — *Go to Definition*, *Find References*, *Go to Symbol*
  (`Ctrl+Shift+O`) / *Workspace Symbol* (`Ctrl+T`), via a generic LSP-bridge
  extension pointing at `oapi-lsp`.
- **Helix / Zed** — register a language server named `oapi-lsp` with command
  `oapi-lsp` for the `yaml` language.

## Symbol pickers (Neovim)

Operations and components are served as standard `textDocument/documentSymbol`
and `workspace/symbol` results, so any LSP-aware symbol picker works with no
extra configuration:

- **Native** (`vim.lsp.buf.document_symbol` / `workspace_symbol`, `gO`)
- **Telescope** (`lsp_document_symbols` / `lsp_dynamic_workspace_symbols`)
- **fzf-lua** (`lsp_document_symbols` / `lsp_live_workspace_symbols`)
- **snacks.nvim** (`Snacks.picker.lsp_symbols` / `lsp_workspace_symbols`)
- **mini.pick**, and any other picker that consumes LSP symbols

## Mason

`oapi-lsp` is not in the [Mason](https://github.com/mason-org/mason.nvim)
registry yet, so it can't be installed with `:MasonInstall` today. The
lazy.nvim spec above already gives you Mason-like convenience (install + update
in one place). A registry entry may be added later.

## Development

```sh
go build ./...
go test ./...
```

Layout:

| Path                | Responsibility                                                   |
| ------------------- | --------------------------------------------------------------- |
| `internal/spec`     | YAML parsing, the OpenAPI index, and the multi-file workspace.   |
| `internal/yamlpos`  | Byte-offset → LSP position (UTF-16) conversion and hit-testing.  |
| `internal/lsp`      | glsp handlers wiring the index to LSP methods.                   |
| `cmd/oapi-lsp`      | Entry point (stdio server).                                      |

Built on [`goccy/go-yaml`](https://github.com/goccy/go-yaml) and
[`tliron/glsp`](https://github.com/tliron/glsp).
