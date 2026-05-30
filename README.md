# oapi-language-server

An editor-agnostic [Language Server](https://microsoft.github.io/language-server-protocol/)
for OpenAPI / Swagger YAML specs.

It provides the navigation that `yaml-language-server` does not: jumping between
`$ref` declarations and their definitions (internal **and** across files), and
listing operations and components through your editor's normal symbol pickers.
It is designed to run **alongside** `yaml-language-server`, which keeps doing
schema validation, completion and hover.

---

## Why a separate server?

`yaml-language-server` is excellent at JSON-Schema validation, but its
go-to-definition only resolves YAML anchors/aliases — **not** OpenAPI `$ref`
JSON-pointers — and it has no find-references for them
([yaml-language-server#1016](https://github.com/redhat-developer/yaml-language-server/issues/1016)).

This server fills exactly that gap. Because every feature maps onto a standard
LSP method, your existing symbol picker — Telescope, fzf-lua, snacks, the native
picker, VS Code, Helix, Zed — consumes them for free, with no plugin to install.

---

## Features

| LSP method                    | Trigger (typical)        | What you get                                                                 |
| ----------------------------- | ------------------------ | --------------------------------------------------------------------------- |
| `textDocument/definition`     | `gd`, `<C-]>`            | Jump from a `$ref` to its definition — internal `#/…` **or** `./file.yaml#/…`. |
| `textDocument/references`     | `gr`                     | From a definition key (or a `$ref`), list every `$ref` to it — **across files**. |
| `textDocument/documentSymbol` | `gO`, symbol picker      | A *Paths* tree (`METHOD /path`, with `operationId`/`summary`) and a *Components* tree grouped by kind. |
| `workspace/symbol`            | workspace-symbol picker  | Fuzzy-find every operation and component across the whole workspace.         |

Notes:

- **Multi-file `$ref`** works out of the box. External files (e.g.
  `schemas/User.yaml`) are indexed even when they aren't specs themselves, so
  definition and references reach into and out of them.
- **Detection**: a YAML file is treated as a spec when one of its first lines
  has a top-level `openapi:` or `swagger:` key. Plain YAML is left alone.
- Validation, completion, hover and rename are intentionally **delegated to
  `yaml-language-server`** — run both together.

---

## Installation

You need a Go toolchain (1.21+).

```sh
go install github.com/stardustcrsd3r/oapi-language-server/cmd/oapi-lsp@latest
```

This puts an `oapi-lsp` binary in `$(go env GOPATH)/bin` — make sure that's on
your `PATH`. Verify:

```sh
oapi-lsp --version
```

Also install `yaml-language-server` for validation/completion (optional but
recommended):

```sh
npm install -g yaml-language-server
```

---

## Neovim setup (0.10+)

The server needs no plugin — just start it for YAML buffers. Pick **one** of the
approaches below.

### A. Plain `vim.lsp.start` (no dependencies)

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "yaml",
  callback = function()
    vim.lsp.start({
      name = "oapi-lsp",
      cmd = { "oapi-lsp" },
      root_dir = vim.fs.root(0, { ".git" }) or vim.fn.getcwd(),
    })
  end,
})
```

### B. With `nvim-lspconfig` (alongside `yamlls`)

```lua
-- Register a custom config once.
local configs = require("lspconfig.configs")
local lspconfig = require("lspconfig")

if not configs.oapi_lsp then
  configs.oapi_lsp = {
    default_config = {
      cmd = { "oapi-lsp" },
      filetypes = { "yaml" },
      root_dir = lspconfig.util.root_pattern(".git", "openapi.yaml", "openapi.yml"),
      single_file_support = true,
    },
  }
end

lspconfig.oapi_lsp.setup({})
lspconfig.yamlls.setup({})  -- validation/completion/hover
```

### Keymaps

These are the standard LSP keymaps — bind them in your `LspAttach` autocmd if you
don't already:

```lua
vim.api.nvim_create_autocmd("LspAttach", {
  callback = function(args)
    local opts = { buffer = args.buf }
    vim.keymap.set("n", "gd", vim.lsp.buf.definition, opts)  -- $ref -> definition
    vim.keymap.set("n", "gr", vim.lsp.buf.references, opts)  -- find all $refs
    vim.keymap.set("n", "gO", vim.lsp.buf.document_symbol, opts)
  end,
})
```

---

## Symbol pickers

Operations and components are served as standard `textDocument/documentSymbol`
and `workspace/symbol` results, so any LSP-aware symbol picker works with no
extra configuration — the data comes from the server. Compatible with:

- **Native** Neovim (`vim.lsp.buf.document_symbol` / `workspace_symbol`, `gO`)
- **Telescope** (`lsp_document_symbols` / `lsp_dynamic_workspace_symbols`)
- **fzf-lua** (`lsp_document_symbols` / `lsp_live_workspace_symbols`)
- **snacks.nvim** (`Snacks.picker.lsp_symbols` / `lsp_workspace_symbols`)
- **mini.pick**, and any other picker that consumes LSP symbols

---

## Other editors

Launch `oapi-lsp` over stdio and route `yaml` documents to it; run
`yaml-language-server` alongside. There are no configuration options.

- **VS Code**: use a generic LSP bridge extension pointing `cmd` at `oapi-lsp`
  for `yaml`. Symbols appear in *Go to Symbol* (`Ctrl+Shift+O`) and *Go to Symbol
  in Workspace* (`Ctrl+T`).
- **Helix / Zed**: add a language server named `oapi-lsp` with command
  `oapi-lsp` for the `yaml` language, in addition to `yaml-language-server`.

---

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

Built on [`goccy/go-yaml`](https://github.com/goccy/go-yaml) (AST with token
positions) and [`tliron/glsp`](https://github.com/tliron/glsp) (LSP framework).
