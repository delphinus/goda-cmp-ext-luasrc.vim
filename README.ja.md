# goda-cmp-ext-luasrc.vim

[English](README.md)

> [!NOTE]
> goda-cmp.vim 自体が開発の途中のため、それに合わせてこのプラグインの API も変わることがあります。

https://github.com/user-attachments/assets/4f6cc85f-eae6-4c5d-a911-93a10b816287

*nvimlsp のソースで gopls の候補を、UI luasrcpum と colorful-menu.nvim で出したところ (`demo/record.sh` で録画)。*

候補を Lua のモジュールに頼む [goda-cmp.vim](https://github.com/Shougo/goda-cmp.vim) のソース。Neovim 専用。

Lua のモジュールが 2 つ付いている:

- **nvimlsp**: Neovim がアタッチした LSP クライアントに頼む。
- **blink**: [blink.cmp](https://github.com/saghen/blink.cmp) 用に書かれたソースを、blink.cmp を設定せずに呼ぶ。

## ほかの LSP のソース

言語サーバから補完するソースは、ほかに 2 つある:

| | nvimlsp (このプラグイン) | [airRnot1106/goda-cmp-source-nvimlsp](https://github.com/airRnot1106/goda-cmp-source-nvimlsp) | [goda-cmp-ext](https://github.com/Shougo/goda-cmp-ext) の `lsp` |
|---|---|---|---|
| サーバ | Neovim がアタッチしたクライアント | Neovim がアタッチしたクライアント | ソースが自分で起動する。設定は Go に書く |
| エディタ | Neovim 0.11 以降 | Neovim nightly (0.13) | Vim と Neovim |
| インストール | プラグインマネージャと Go のモジュール (Lua のモジュールを含むため) | Go のモジュールだけ (Lua はバイナリに埋め込まれている) | Go のモジュール |
| スニペット | 展開しない。プレースホルダはその文字列になる | `vim.snippet` で展開する | 展開しない |
| `completionItem/resolve`、`additionalTextEdits` | 常に行う | パラメータで有効にしたとき | 常に行う |
| その他 | blink のモジュール。colorful-menu.nvim と UI luasrcpum | | |

blink.cmp のソースや colorful-menu.nvim のラベルを使うならこのプラグイン、スニペットを展開したいなら airRnot1106/goda-cmp-source-nvimlsp、Vim で使うなら `lsp` のソースが向いている。

## 必要なもの

- Neovim 0.11 以降
- goda.vim と goda-cmp.vim (v0.4.1 以降)、設定をビルドする Go
- [pum.vim](https://github.com/Shougo/pum.vim): `preview.lua` が描くプレビューに使う (任意)。UI luasrcpum には、あわせて [goda-cmp-ui-pum.vim](https://github.com/Shougo/goda-cmp-ui-pum.vim) が要る
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

バッファの LSP クライアントに `textDocument/completion` を送る。goda-cmp-ext の lsp ソースはサーバを自分で起動するが、こちらは Neovim がアタッチしたクライアントを使うので:

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

### UI luasrcpum

goda-cmp.vim の UI (goda-cmp-ui-pum.vim など) は色を描かない。`ext/ui/luasrcpum` は、UI "pum" と同じく pum.vim で候補を出し、加えて `luasrc.Ref` の中身を描く UI。colorful-menu.nvim の色を付けた表示と、右に寄せたシグネチャや型を出す。ほかのソースの候補は UI "pum" と同じに出し、選択・確定・プレビューは UI "pum" に任せる。

```go
b.AddUI(luasrcpum.New())
b.PatchGlobal(cmp.Config{UI: "luasrcpum"})
```

| パラメータ | 既定値 | |
|---|---|---|
| `insert` | `false` | UI "pum" のもの |
| `abbrMaxWidth` | `50` | 表示と右に寄せる部分を収める表示幅 |
| `detailMinGap` | `3` | 表示と右に寄せる部分の間の最小の空白 |
| `detailMinWidth` | `8` | 右に寄せる部分がこれより狭くしか入らないときは出さない |
| `detailHighlight` | `"Comment"` | 色を持たない右に寄せる部分の色 |

pum.vim は、`min_width`・`max_width` に合わせて表示を広げたり切り詰めたりする前の幅で色の位置を計算するので、色がずれる。この 2 つは設定せず、pum.vim の `max_columns.abbr` を `abbrMaxWidth` と同じ値にする (表示はこの UI が切り詰める)。

## 仕組み

ソースは Lua の中で動き、Go はそれを非同期に待つ:

1. `Gather` が Lua のモジュールの `complete()` を Notify で呼ぶ。
2. Lua が候補を集め、返事を `goda#notify()` でプラグイン `luasrc` (ブリッジ) に送り、ブリッジがそれを `Gather` に渡す。

goda.vim はエディタからの通知を順に処理するので、返事はそれより前に送られた通知を待つ。goda-cmp は通知のハンドラの中でソースを待たない (v0.4.1 以降) ので、返事が待たされることはない。

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

## デモ

`demo/record.sh` は、最小の設定 (`demo/init.lua`、`demo/config`) で UI luasrcpum のデモを録画する。Neovim と gopls を使い、プラグインは自分で取得して、`demo/sample/main.go` に打ち込む。asciinema、agg、ffmpeg が要る。出力は `demo/out/`。

## ライセンス

MIT。LICENSE を参照。
