// Package luasrc is a goda-cmp source asking a Lua module for the candidates.
// It works in Neovim only.
//
// The LSP clients of Neovim (lua/goda_cmp_luasrc/nvimlsp.lua) and sources
// written for blink.cmp (lua/goda_cmp_luasrc/blink.lua) run only in Lua. This
// is the way to call them from Go and wait for the reply, which goes
// asynchronously in this order:
//
//  1. Gather calls complete() of the Lua module with Notify.
//  2. Lua gathers the candidates and leaves the reply on the Lua side
//     (reply() of lua/goda_cmp_luasrc/init.lua).
//  3. Gather calls take() with Call until the reply is there.
//
// Lua does not send the reply with goda#notify(), because goda.vim (since
// v0.4.2) makes the notifications and requests from the editor wait until the
// previous notifications are done. The complete of goda-cmp may wait for the
// sources in the handler of a notification, so a reply sent after it would
// not arrive until complete is over, and the completion would stall until the
// timeout. A Call from this side does not get in that line.
//
// The original data of the candidates (needed to resolve and confirm them)
// stays on the Lua side; this side keeps only (the request id, the index).
//
// A Lua module has these three functions:
//
//	complete({id, source, bufnr, lnum, input, keywordStart, manual})
//	  → reply(id, {request, items, incomplete, cacheable})
//	resolve({id, source, request, index}) → reply(id, {info})
//	done({source, bufnr, request, index})   (no reply)
package luasrc

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda.vim/sdk"
)

// Bridge is the plugin "luasrc", used to make the sources and passed to
// b.Main() too. It numbers the requests.
type Bridge struct {
	mu     sync.Mutex
	nextID int64
}

// NewBridge makes a Bridge.
func NewBridge() *Bridge {
	return &Bridge{}
}

// Name is the name of the plugin.
func (*Bridge) Name() string { return "luasrc" }

// Handlers is empty: this side goes to take the replies (call).
func (*Bridge) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{}
}

// pollInterval is how often to look whether a reply is on the Lua side.
const pollInterval = 5 * time.Millisecond

// luasrcModule is the Lua module keeping the replies
// (lua/goda_cmp_luasrc/init.lua).
const luasrcModule = "goda_cmp_luasrc"

// The Lua modules bundled with this plugin, to pass to Bridge.Source.
const (
	// NvimLSPModule asks the LSP clients of Neovim.
	NvimLSPModule = "goda_cmp_luasrc.nvimlsp"
	// BlinkModule calls sources written for blink.cmp, set up with
	// require("goda_cmp_luasrc.blink").setup().
	BlinkModule = "goda_cmp_luasrc.blink"
)

// call passes args to the function fn of the Lua module module, and puts its
// reply into dst.
func (b *Bridge) call(ctx context.Context, h sdk.Host, module, fn string, args map[string]any, dst any) error {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.mu.Unlock()

	args["id"] = id
	if err := notify(ctx, h, module, fn, args); err != nil {
		return err
	}
	// When it stops waiting (on the timeout, etc.), have the late reply
	// dropped. ctx is over, so it is not used.
	forget := func() {
		_ = h.Notify(context.Background(), "luaeval", fmt.Sprintf("require'%s'.forget(_A)", luasrcModule), id)
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		// Notify runs before the next Call, so take() never comes before
		// complete() is called.
		v, err := h.Call(ctx, "luaeval", fmt.Sprintf("require'%s'.take(_A)", luasrcModule), id)
		if err != nil {
			forget()
			return err
		}
		if v != nil {
			return sdk.Decode(v, dst)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			forget()
			return ctx.Err()
		}
	}
}

func notify(ctx context.Context, h sdk.Host, module, fn string, args map[string]any) error {
	return h.Notify(ctx, "luaeval", fmt.Sprintf("require'%s'.%s(_A)", module, fn), args)
}

// Source makes the source name, which asks the Lua module module for the
// candidates.
func (b *Bridge) Source(name, module string) cmp.Source {
	return &source{b: b, name: name, module: module}
}

// Ref is the UserData of a candidate. It points to the original data of the
// candidate left on the Lua side. The other fields are for a UI that reads
// them.
type Ref struct {
	Request int64
	Index   int
	// Icon and IconGroup are the icon and its highlight group the Lua side
	// chose for the candidate (the state of an issue or a PR of git, etc.).
	// When empty, a UI may use those of the source.
	Icon      string
	IconGroup string
	// Offset is the score offset of the candidate (score_offset of blink.cmp).
	Offset int
	// Detail is the part to put on the right of the candidate (the signature
	// of a function, a type, etc.).
	Detail string
	// AbbrHighlights and DetailHighlights are the highlights of the label
	// (Abbr) and of Detail (those colorful-menu.nvim made with treesitter).
	AbbrHighlights, DetailHighlights []Span

	resolved bool
}

// Span is a highlight group on [Start, End) (byte positions) of a string.
type Span struct {
	Group string `json:"group"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// ScoreOffset is the score offset of the candidate, for a filter that adds it
// up across the sources.
func (r Ref) ScoreOffset() int { return r.Offset }

type item struct {
	Word string `json:"word"`
	Abbr string `json:"abbr"`
	Kind string `json:"kind"`
	Menu string `json:"menu"`
	Info string `json:"info"`
	// LabelDetail, AbbrHighlights and DetailHighlights go to the fields of
	// Ref (Detail and the same names).
	LabelDetail      string `json:"labelDetail"`
	AbbrHighlights   []Span `json:"abbrHighlights"`
	DetailHighlights []Span `json:"detailHighlights"`
	FilterText       string `json:"filterText"`
	SortText         string `json:"sortText"`
	// Start is where the word begins (a byte position in Input). Some
	// candidates replace from before the keyword (postfix completions of
	// gopls, : of emoji, etc.).
	Start int `json:"start"`
	Index int `json:"index"`

	Icon        string `json:"icon"`
	IconGroup   string `json:"iconGroup"`
	ScoreOffset int    `json:"scoreOffset"`
}

type completion struct {
	Request    int64  `json:"request"`
	Items      []item `json:"items"`
	Incomplete bool   `json:"incomplete"`
	// Cacheable means the candidates do not change while typing on (they only
	// need narrowing down). Lua is not asked again meanwhile.
	Cacheable bool `json:"cacheable"`
}

// cached is a result reused without asking again.
type cached struct {
	bufnr, lnum int
	input       string
	g           cmp.Gathered
}

type source struct {
	b            *Bridge
	name, module string

	mu    sync.Mutex
	cache *cached
}

func (s *source) Name() string { return s.name }

// typedOn is the text typed on with which the result may be reused (keyword
// characters only).
var typedOn = regexp.MustCompile(`^[\w-]*$`)

func (s *source) fromCache(c *cmp.Context) (cmp.Gathered, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.cache
	if k == nil || k.bufnr != c.Bufnr || k.lnum != c.Lnum || !strings.HasPrefix(c.Input, k.input) ||
		!typedOn.MatchString(c.Input[len(k.input):]) {
		s.cache = nil
		return cmp.Gathered{}, false
	}
	return k.g, true
}

// GatherStream emits the result of Gather once. As a StreamSource, a slow
// source (git listing with gh, etc.) is not dropped from the completion after
// 200 milliseconds (hideTimeout), and is added to the menu when it arrives.
func (s *source) GatherStream(ctx context.Context, h sdk.Host, c *cmp.Context, emit func(cmp.Gathered) error) error {
	g, err := s.Gather(ctx, h, c)
	if err != nil {
		return err
	}
	return emit(g)
}

func (s *source) Gather(ctx context.Context, h sdk.Host, c *cmp.Context) (cmp.Gathered, error) {
	if g, ok := s.fromCache(c); ok {
		return g, nil
	}
	ks := c.KeywordStart()
	var r completion
	err := s.b.call(ctx, h, s.module, "complete", map[string]any{
		"source":       s.name,
		"bufnr":        c.Bufnr,
		"lnum":         c.Lnum,
		"input":        c.Input,
		"keywordStart": ks,
		"manual":       c.Manual,
	}, &r)
	if err != nil {
		return cmp.Gathered{Start: ks}, err
	}

	// complete() replaces one range for all the candidates, so align them to
	// the leftmost position, and prepend the text from there to the position
	// of each candidate (as ext/source/lsp does).
	start := ks
	for _, it := range r.Items {
		if it.Start >= 0 && it.Start <= len(c.Input) {
			start = min(start, it.Start)
		}
	}
	items := make([]cmp.Item, 0, len(r.Items))
	for _, it := range r.Items {
		ci := cmp.Item{
			Word:       it.Word,
			Abbr:       it.Abbr,
			Kind:       it.Kind,
			Menu:       it.Menu,
			Info:       it.Info,
			FilterText: it.FilterText,
			SortText:   it.SortText,
			Dup:        1,
			UserData: Ref{
				Request: r.Request, Index: it.Index,
				Icon: it.Icon, IconGroup: it.IconGroup, Offset: it.ScoreOffset,
				Detail: it.LabelDetail, AbbrHighlights: it.AbbrHighlights, DetailHighlights: it.DetailHighlights,
			},
		}
		if it.Start > start && it.Start <= len(c.Input) {
			pre := c.Input[start:it.Start]
			if ci.Abbr == "" {
				ci.Abbr = ci.Word
			}
			ci.Word = pre + ci.Word
			if ci.FilterText != "" {
				ci.FilterText = pre + ci.FilterText
			}
		}
		items = append(items, ci)
	}
	g := cmp.Gathered{Start: start, Items: items, Incomplete: r.Incomplete && !r.Cacheable}
	if r.Cacheable {
		s.mu.Lock()
		s.cache = &cached{bufnr: c.Bufnr, lnum: c.Lnum, input: c.Input, g: g}
		s.mu.Unlock()
	}
	return g, nil
}

// Resolve fetches the details (mostly the documentation) of the selected
// candidate.
func (s *source) Resolve(ctx context.Context, h sdk.Host, a cmp.DoneArgs) (cmp.Item, error) {
	d, ok := a.Item.UserData.(Ref)
	if !ok || d.resolved {
		return a.Item, nil
	}
	var r struct {
		Info string `json:"info"`
	}
	err := s.b.call(ctx, h, s.module, "resolve", map[string]any{
		"source": s.name, "request": d.Request, "index": d.Index,
	}, &r)
	if err != nil {
		return a.Item, err
	}
	it := a.Item
	if r.Info != "" {
		it.Info = r.Info
	}
	d.resolved = true
	it.UserData = d
	return it, nil
}

// Preview shows the information of the candidate as Markdown in the preview
// window (the preview of pum.vim). The documentation of LSP is Markdown, and
// the rest (the body of a git issue, the annotation of a dictionary, etc.)
// reads fine as Markdown too.
//
// The preview is called synchronously with goda#request(), so asking Lua here
// and waiting for the reply would hang Neovim, which could not process the
// reply. Lua is not called: the Info Resolve fetched when the candidate was
// selected is used as it is.
func (s *source) Preview(_ context.Context, _ sdk.Host, a cmp.DoneArgs) (cmp.Preview, error) {
	if strings.TrimSpace(a.Item.Info) == "" {
		return cmp.Preview{Kind: cmp.PreviewEmpty}, nil
	}
	return cmp.Preview{
		Kind:     cmp.PreviewMarkdown,
		Contents: strings.Split(a.Item.Info, "\n"),
		// pum.vim runs it in the preview window. It draws with treesitter, as
		// the documentation window of blink.cmp does
		// (lua/goda_cmp_luasrc/preview.lua).
		Command: "lua require'goda_cmp_luasrc.preview'.render()",
	}, nil
}

// OnCompleteDone has the Lua side finish the confirmed candidate (adding an
// import of LSP, etc.).
func (s *source) OnCompleteDone(ctx context.Context, h sdk.Host, a cmp.DoneArgs) error {
	d, ok := a.Item.UserData.(Ref)
	if !ok {
		return nil
	}
	return notify(ctx, h, s.module, "done", map[string]any{
		"source":  s.name,
		"bufnr":   a.Context.Bufnr,
		"request": d.Request,
		"index":   d.Index,
	})
}

var (
	_ cmp.Source        = (*source)(nil)
	_ cmp.StreamSource  = (*source)(nil)
	_ cmp.Resolver      = (*source)(nil)
	_ cmp.CompleteDoner = (*source)(nil)
	_ cmp.Previewer     = (*source)(nil)
	_ sdk.Plugin        = (*Bridge)(nil)
)
