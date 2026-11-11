# goda-cmp-ext-luasrc.vim

[English](README.md)

> [!WARNING]
> β版 (v0.0.x) です。goda-cmp.vim 自体が開発の途中で、同梱の拡張を別のリポジトリへ移す作業も進んでいるため、API・import のパス・このリポジトリの名前が変わることがあります。

候補を Lua のモジュールに頼む [goda-cmp.vim](https://github.com/Shougo/goda-cmp.vim) のソース。Neovim 専用。

Lua のモジュールが 2 つ付いている:

- **nvimlsp**: Neovim がアタッチした LSP クライアントに頼む。
- **blink**: [blink.cmp](https://github.com/saghen/blink.cmp) 用に書かれたソースを、blink.cmp を設定せずに呼ぶ。

## 必要なもの

- Neovim 0.11 以降
- goda.vim と goda-cmp.vim、設定をビルドする Go
- [pum.vim](https://github.com/Shougo/pum.vim): `preview.lua` が描くプレビューに使う (任意)
- [colorful-menu.nvim](https://github.com/xzbdmw/colorful-menu.nvim): LSP の候補の表示に使う (任意)

## インストール

Lua のモジュールを含むので、Go のモジュールとしてだけでなく、プラグインマネージャで入れる。lazy.nvim なら:

```lua
{ "delphinus/goda-cmp-ext-luasrc.vim", dependencies = { "Shougo/goda-cmp.vim" } }
```

そのディレクトリを `g:goda#workspace` に足し (goda-pm.vim は自分のプラグインについてこれを行う)、`goda#build()` が入れた Go のコードで設定をビルドするようにする:

```lua
vim.g["goda#workspace"] = {
  vim.fs.joinpath(require("lazy.core.config").options.root, "goda-cmp-ext-luasrc.vim"),
  -- goda.vim、goda-cmp.vim、...
}
```

そのうえで、設定のモジュールで Go のモジュールを require する:

```sh
go get github.com/delphinus/goda-cmp-ext-luasrc.vim@latest
```

## 設定

設定の `main.go` でブリッジを作り、それで作ったソースを足し、ブリッジ自身も `Main` に渡す。ブリッジは、ソースが Lua とやり取りするためのプラグイン。

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

`bridge.Source(name, module)` は、Lua のモジュール `module` を呼ぶソース `name` を作る。1 つのモジュールが複数のソースを受け持てる (モジュールにはソースの名前が渡される)。

### nvimlsp

バッファの LSP クライアントに `textDocument/completion` を送る。goda-cmp 同梱の lsp ソースはサーバを自分で起動するが、こちらは Neovim がアタッチしたクライアントを使うので:

- サーバは 2 本でなく 1 本だけ動く。
- `vim.lsp.config()` の設定がそのまま使われる。
- `completionItem/resolve` でドキュメントを埋め、確定のときに `additionalTextEdits` (import の追加など) を適用する。

スニペットは展開しない。プレースホルダはその文字列に置き換える。クライアントの capabilities で `snippetSupport` を切ると、より良い候補が返る。

### blink

blink.cmp 用のソースの多くは、blink.cmp の kind の表と小さな非同期のライブラリくらいにしか頼っていない。そのため、blink.cmp のモジュールを `package.path` に載せるだけで呼べる。blink.cmp とソースをインストールし、blink.cmp は設定せずに、それらの場所と受け持つソースを `setup()` に教える:

```lua
local root = require("lazy.core.config").options.root
require("goda_cmp_luasrc.blink").setup {
  -- package.path に足すディレクトリ。
  paths = vim.tbl_map(function(name)
    return vim.fs.joinpath(root, name)
  end, { "blink.cmp", "blink-emoji.nvim", "blink-ripgrep.nvim" }),
  -- キーは main.go で bridge.Source() に渡したソースの名前。
  sources = {
    emoji = { module = "blink-emoji", static = true, insert_text = true },
    ripgrep = { module = "blink-ripgrep", opts = {} },
  },
}
```

ソースのフィールド:

| フィールド | 意味 |
|---|---|
| `module` | blink.cmp のソースのモジュール |
| `opts` | その opts (blink.cmp の `providers.*.opts`) |
| `static` | 候補が打った文字で変わらない。打ち足しているあいだは聞き直さない |
| `insert_text` | `newText` ではなく `insertText` を挿入する。確定のときに resolve で候補を差し替えるソース (emoji、nerdfont) 向け |
| `transform` | blink.cmp の `transform_items` |
| `icon` | 候補ごとの `fun(kind): icon, hl_group` |
| `at_trigger` | カーソルが起動文字の直後にあるように見せる (blink-cmp-git) |

ソースに渡す context は、blink.cmp のものを最小限まねたもの (`line`、`cursor`、`bounds`、`trigger`)。blink.cmp にそれ以上を求めるソースは動かないことがある。

### プレビュー

ドキュメントは、pum.vim のプレビューウィンドウに Markdown として出す。detail (シグネチャなど) をバッファの言語のコードブロックにし、その後にドキュメントを続ける。`preview.lua` は、blink.cmp のドキュメントのウィンドウや LSP の hover と同じく treesitter で描き、区切りの `---` を線にする。

### colorful-menu.nvim

インストールされていれば、LSP の候補の表示を colorful-menu.nvim で組み立てる。表示と、右に寄せる部分 (シグネチャ、型) は、それぞれの色と一緒に `UserData` (`luasrc.Ref`) に分けて置く。並べて色を付ける UI が使える。

## 仕組み

ソースは Lua の中で動き、Go はそれを非同期に待つ:

1. `Gather` が Lua のモジュールの `complete()` を Notify で呼ぶ。
2. Lua が候補を集め、返事を Lua の側に置く。
3. `Gather` は、返事が置かれるまで `take()` を Call で呼ぶ。

Lua から `goda#notify()` で返事を送らないのは、goda.vim がエディタからの通知を順に処理し、goda-cmp が通知のハンドラの中でソースを待つことがあるため。そのあいだは返事が届かない。Go の側からの Call は、その順番に並ばない。

候補の元のデータは Lua の側に置いたままにし、Go は要求の id と何番目かだけを持つ。

## Lua のモジュールの書き方

モジュールは 3 つの関数を持つ。返事は `require("goda_cmp_luasrc").reply()` で返す。

```
complete({id, source, bufnr, lnum, input, keywordStart, manual})
  → reply(id, {request, items, incomplete, cacheable})
resolve({id, source, request, index}) → reply(id, {info})
done({source, bufnr, request, index})   (返事は無い)
```

`goda_cmp_luasrc` には補助の関数もある。`convert()` は LSP の `CompletionItem` を候補に変え、`info()` はその Markdown を作り、`remember()` / `entry()` は resolve と確定に使う元のデータを持っておく。`nvimlsp.lua` と `blink.lua` が例になる。

## ライセンス

MIT。LICENSE を参照。
