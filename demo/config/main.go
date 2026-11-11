// Command demo is the goda-cmp configuration used by demo/record.sh: the
// nvimlsp source of luasrc, shown with the UI luasrcpum.
package main

import (
	"github.com/Shougo/goda-cmp-ext/ext/filter/matcherhead"
	"github.com/Shougo/goda-cmp.vim/cmp"

	"github.com/delphinus/goda-cmp-ext-luasrc.vim/ext/source/luasrc"
	"github.com/delphinus/goda-cmp-ext-luasrc.vim/ext/ui/luasrcpum"
)

func main() {
	bridge := luasrc.NewBridge()

	b := cmp.New()
	b.AddUI(luasrcpum.New())
	b.AddSource(bridge.Source("nvimlsp", luasrc.NvimLSPModule))
	b.AddFilter(matcherhead.New(matcherhead.Params{}))
	b.PatchGlobal(cmp.Config{
		UI:      "luasrcpum",
		Sources: []string{"nvimlsp"},
		SourceOptions: map[string]cmp.SourceOption{
			"_": {Matchers: []string{"matcherhead"}},
			"nvimlsp": {
				Mark:                   "LSP",
				ForceCompletionPattern: `\.\w*`,
			},
		},
	})
	b.Main(bridge)
}
