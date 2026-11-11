# goda-cmp-ext-luasrc.vim

[日本語](README.ja.md)

> [!WARNING]
> This is a beta (v0.0.x). goda-cmp.vim itself is under active development, so
> the API, the import paths and even the name of this repository may change.

https://github.com/user-attachments/assets/4f6cc85f-eae6-4c5d-a911-93a10b816287

*gopls through the nvimlsp source, shown with the UI luasrcpum and colorful-menu.nvim (recorded with `demo/record.sh`).*

A source for [goda-cmp.vim](https://github.com/Shougo/goda-cmp.vim) that asks a
Lua module for the candidates. Neovim only.

Two Lua modules come with it:

- **nvimlsp** asks the LSP clients Neovim has attached.
- **blink** calls sources written for [blink.cmp](https://github.com/saghen/blink.cmp)
  without setting blink.cmp up.

## Requirements

- Neovim 0.11 or newer
- goda.vim and goda-cmp.vim, and Go to build your configuration
- [pum.vim](https://github.com/Shougo/pum.vim) for the preview drawn by
  `preview.lua` (optional), and with
  [goda-cmp-ui-pum.vim](https://github.com/Shougo/goda-cmp-ui-pum.vim) for the
  UI luasrcpum
- [colorful-menu.nvim](https://github.com/xzbdmw/colorful-menu.nvim) for the
  labels of LSP candidates (optional)

## Installation

The plugin has Lua modules, so install it with your plugin manager, not only
as a Go module. With lazy.nvim:

```lua
{ "delphinus/goda-cmp-ext-luasrc.vim", dependencies = { "Shougo/goda-cmp.vim" } }
```

Add its directory to `g:goda#workspace` (goda-pm.vim does it for its plugins)
so that `goda#build()` builds your configuration with the Go code installed:

```lua
vim.g["goda#workspace"] = {
  vim.fs.joinpath(require("lazy.core.config").options.root, "goda-cmp-ext-luasrc.vim"),
  -- goda.vim, goda-cmp.vim, ...
}
```

Then require the Go module in your configuration module:

```sh
go get github.com/delphinus/goda-cmp-ext-luasrc.vim@latest
```

## Configuration

In the `main.go` of your configuration, make a bridge, add the sources made
with it, and pass the bridge to `Main` too: it is the plugin that the sources
talk to Lua through.

```go
import (
	"github.com/Shougo/goda-cmp.vim/cmp"

	"github.com/delphinus/goda-cmp-ext-luasrc.vim/ext/source/luasrc"
)

func main() {
	bridge := luasrc.NewBridge()

	b := cmp.New()
	b.AddSource(bridge.Source("nvimlsp", luasrc.NvimLSPModule))
	for _, name := range []string{"emoji", "ripgrep"} {
		b.AddSource(bridge.Source(name, luasrc.BlinkModule))
	}
	// ...
	b.Main(bridge)
}
```

`bridge.Source(name, module)` makes the source `name`, which calls the Lua
module `module`. One module may serve several sources: it is given the name.

### nvimlsp

Sends `textDocument/completion` to the LSP clients of the buffer. The lsp
source of goda-cmp-ext starts the servers by itself; this one uses the
clients Neovim attached, so

- one server runs, not two,
- the settings of `vim.lsp.config()` apply as they are,
- `completionItem/resolve` fills the documentation, and the
  `additionalTextEdits` (adding an import, etc.) are applied on confirmation.

Snippets are not expanded: placeholders are replaced with their text. Turning
`snippetSupport` off in the client capabilities gives better candidates.

### blink

Many sources for blink.cmp rely on little more than its table of kinds and its
small async library, so they can be called with only the modules of blink.cmp
on `package.path`. Install blink.cmp and the sources, but do not set blink.cmp
up, and tell `setup()` where they are and which sources to serve:

```lua
local root = require("lazy.core.config").options.root
require("goda_cmp_luasrc.blink").setup {
  -- Directories added to package.path.
  paths = vim.tbl_map(function(name)
    return vim.fs.joinpath(root, name)
  end, { "blink.cmp", "blink-emoji.nvim", "blink-ripgrep.nvim" }),
  -- The keys are the source names given to bridge.Source() in main.go.
  sources = {
    emoji = { module = "blink-emoji", static = true, insert_text = true },
    ripgrep = { module = "blink-ripgrep", opts = {} },
  },
}
```

The fields of a source:

| Field | Meaning |
|---|---|
| `module` | The module of the blink.cmp source |
| `opts` | Its opts (`providers.*.opts` of blink.cmp) |
| `static` | The candidates do not change with the typed text: do not ask again while typing on |
| `insert_text` | Insert `insertText` instead of `newText`, for sources that replace the item in resolve on confirmation (emoji, nerdfont) |
| `transform` | `transform_items` of blink.cmp |
| `icon` | `fun(kind): icon, hl_group` per candidate |
| `at_trigger` | Make the cursor look right after the trigger character (blink-cmp-git) |

The context given to a source is a minimal imitation of that of blink.cmp
(`line`, `cursor`, `bounds`, `trigger`). Sources that need more of blink.cmp
may not work.

### Preview

The documentation is shown as Markdown in the preview window of pum.vim: the
detail (a signature, etc.) as a code block in the language of the buffer, then
the documentation. `preview.lua` draws it with treesitter, as the documentation
window of blink.cmp and the LSP hover do, and draws the separator `---` as a
line.

### colorful-menu.nvim

If it is installed, the labels of LSP candidates are built with
colorful-menu.nvim. The label and the part for the right side (a signature, a
type) are kept apart in the `UserData` (`luasrc.Ref`) with their highlights,
for a UI that lays them out and colors them.

### The UI luasrcpum

The UIs of goda-cmp.vim (goda-cmp-ui-pum.vim and the others) do not draw
highlights. `ext/ui/luasrcpum`
is a UI that shows candidates with pum.vim as the UI "pum" does, and also
draws what is in `luasrc.Ref`: the label with the highlights of
colorful-menu.nvim, and the signature or type aligned to the right. Candidates
of other sources are shown as the UI "pum" shows them; selecting, confirming
and the preview are left to it.

```go
b.AddUI(luasrcpum.New())
b.PatchGlobal(cmp.Config{UI: "luasrcpum"})
```

| Param | Default | |
|---|---|---|
| `insert` | `false` | the UI "pum"'s |
| `abbrMaxWidth` | `50` | the display width the label and the detail are fitted in |
| `detailMinGap` | `3` | the least number of spaces between the label and the detail |
| `detailMinWidth` | `8` | the detail is not shown when less of it fits |
| `detailHighlight` | `"Comment"` | the highlight of a detail without highlights of its own |

pum.vim computes the positions of the highlights from the width of the label
before it widens or cuts it to fit `min_width` and `max_width`, so the colors
shift. Leave those unset, and set `max_columns.abbr` of pum.vim to
`abbrMaxWidth`: the label is cut by this UI.

## How it works

The sources run in Lua, and Go waits for them asynchronously:

1. `Gather` calls `complete()` of the Lua module with Notify.
2. Lua gathers the candidates and leaves the reply on the Lua side.
3. `Gather` calls `take()` with Call until the reply is there.

Lua does not send the reply with `goda#notify()`, because goda.vim handles the
notifications from the editor in order, and goda-cmp may wait for the sources
in the handler of a notification: the reply would not arrive until then. A
Call from the Go side does not get in that line.

The original data of the candidates stays on the Lua side; Go keeps only the
request id and the index.

## Writing a Lua module

A module has three functions. Reply with `require("goda_cmp_luasrc").reply()`.

```
complete({id, source, bufnr, lnum, input, keywordStart, manual})
  → reply(id, {request, items, incomplete, cacheable})
resolve({id, source, request, index}) → reply(id, {info})
done({source, bufnr, request, index})   (no reply)
```

`goda_cmp_luasrc` also has helpers: `convert()` turns an LSP `CompletionItem`
into an item, `info()` makes its Markdown, and `remember()` / `entry()` keep the
original data to resolve and confirm. `nvimlsp.lua` and `blink.lua` are
examples.

## Demo

`demo/record.sh` records the demo of the UI luasrcpum with a minimal
configuration (`demo/init.lua`, `demo/config`): Neovim, gopls and the plugins
it clones, typing into `demo/sample/main.go`. It needs asciinema, agg and
ffmpeg, and writes `demo/out/`.

## License

MIT, see LICENSE.
