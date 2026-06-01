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
- **Document symbols** — a *Paths* tree (`METHOD /path`) and a *Components* tree
  for the outline and symbol picker. A referenced fragment file that isn't a
  spec itself (e.g. a bare `schemas/User.yaml`) is outlined by its definitions.
- **Workspace symbols** — fuzzy-find every operation and component across the
  workspace.

## Split specs

Multi-file specs work out of the box — referenced files need not be specs
themselves:

- **Schemas in separate files** show up in the symbol pickers, whether they live
  in a shared `components:` file or as a bare `schemas/User.yaml` fragment.
- **Operations behind a path-item `$ref`** (`/users: { $ref: ./paths/users.yaml }`)
  are listed as `METHOD /path`, pointing into the referenced file.
- **Shared component files** are surfaced even before anything references them,
  so you can jump to a schema while you're still wiring it up.

A file is treated as a spec when one of its first lines has a top-level
`openapi:` or `swagger:` key; everything reachable from a spec via `$ref` is
part of the same navigable workspace.

## Install

`oapi-lsp` is a single static binary — no runtime needed. Get it one of two ways:

**Prebuilt binary** (no Go) — download the archive for your OS/arch from the
[releases page](https://github.com/stardustcrsd3r/oapi-language-server/releases),
extract `oapi-lsp`, and put it on your `PATH` (e.g. `~/.local/bin`). Update by
downloading a newer release and replacing the file.

**From source** (needs a Go toolchain):

```sh
go install github.com/stardustcrsd3r/oapi-language-server/cmd/oapi-lsp@latest
```

Lands in `$(go env GOPATH)/bin` (usually `~/go/bin`) — keep that on `PATH`.
Update by re-running it; pin a version with `@v0.1.0` instead of `@latest`.

Either way you end up with an `oapi-lsp` command; point any LSP client at it for
YAML files.

## Neovim

### Binary on `PATH` — any (or no) plugin manager

Once `oapi-lsp` is on `PATH` (from either install above), no plugin is needed —
this config starts it on OpenAPI/Swagger files:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "yaml",
  callback = function(args)
    for _, l in ipairs(vim.api.nvim_buf_get_lines(args.buf, 0, 20, false)) do
      if l:match("^%s*openapi%s*:") or l:match("^%s*swagger%s*:") then
        vim.lsp.start({
          name = "oapi-lsp",
          cmd = { "oapi-lsp" }, -- or an absolute path to the binary
          root_dir = vim.fs.root(args.buf, { ".git" }) or vim.fn.getcwd(),
        }, { bufnr = args.buf })
        return
      end
    end
  end,
})
```

### lazy.nvim — auto-download the binary (no Go)

The `build` hook fetches the matching release binary on install and on every
`:Lazy update` (force a refresh with `:Lazy build oapi-language-server`):

```lua
-- ~/.config/nvim/lua/plugins/oapi-lsp.lua
local install_dir = vim.fn.stdpath("data") .. "/oapi-lsp"
local bin = install_dir .. (vim.fn.has("win32") == 1 and "/oapi-lsp.exe" or "/oapi-lsp")

return {
  "stardustcrsd3r/oapi-language-server",
  ft = "yaml",
  build = function()
    local u = vim.uv.os_uname()
    local goos = ({ Linux = "linux", Darwin = "darwin", Windows_NT = "windows" })[u.sysname]
    local arch = ({ x86_64 = "amd64", arm64 = "arm64", aarch64 = "arm64" })[u.machine]
    local ext = goos == "windows" and "zip" or "tar.gz"
    local asset = ("oapi-lsp_%s_%s.%s"):format(goos, arch, ext)
    local url = "https://github.com/stardustcrsd3r/oapi-language-server"
      .. "/releases/latest/download/" .. asset
    vim.fn.mkdir(install_dir, "p")
    local archive = install_dir .. "/" .. asset
    vim.fn.system({ "curl", "-fsSL", "-o", archive, url })
    vim.fn.system(ext == "zip"
      and { "unzip", "-o", archive, "-d", install_dir }
      or { "tar", "-xzf", archive, "-C", install_dir })
    os.remove(archive)
  end,
  config = function()
    vim.api.nvim_create_autocmd("FileType", {
      pattern = "yaml",
      callback = function(args)
        for _, l in ipairs(vim.api.nvim_buf_get_lines(args.buf, 0, 20, false)) do
          if l:match("^%s*openapi%s*:") or l:match("^%s*swagger%s*:") then
            vim.lsp.start({
              name = "oapi-lsp",
              cmd = { bin },
              root_dir = vim.fs.root(args.buf, { ".git" }) or vim.fn.getcwd(),
            }, { bufnr = args.buf })
            return
          end
        end
      end,
    })
  end,
}
```

### lazy.nvim — build with Go

Lets Go build the binary on install and on every `:Lazy update` (needs a Go
toolchain), so it tracks the plugin:

```lua
-- ~/.config/nvim/lua/plugins/oapi-lsp.lua
local bin = vim.fn.expand("$HOME/go/bin/oapi-lsp")

return {
  "stardustcrsd3r/oapi-language-server",
  ft = "yaml",
  build = "go install ./cmd/oapi-lsp",
  config = function()
    vim.api.nvim_create_autocmd("FileType", {
      pattern = "yaml",
      callback = function(args)
        for _, l in ipairs(vim.api.nvim_buf_get_lines(args.buf, 0, 20, false)) do
          if l:match("^%s*openapi%s*:") or l:match("^%s*swagger%s*:") then
            vim.lsp.start({
              name = "oapi-lsp",
              cmd = { bin },
              root_dir = vim.fs.root(args.buf, { ".git" }) or vim.fn.getcwd(),
            }, { bufnr = args.buf })
            return
          end
        end
      end,
    })
  end,
}
```

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

## License

[MIT](LICENSE).
