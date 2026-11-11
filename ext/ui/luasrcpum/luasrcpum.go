// Package luasrcpum is a UI that shows candidates with pum.vim, as the UI
// "pum" bundled with goda-cmp.vim does, and also draws what the luasrc
// sources put in the UserData of their candidates (luasrc.Ref): the detail
// (a signature, a type) aligned to the right of the label, and the highlights
// that colorful-menu.nvim gives the label and the detail.
//
// goda-cmp's candidates (cmp.Item) have no highlights, while pum.vim takes
// "highlights" per candidate. A UI runs in the same process as the sources,
// so this one adds them to the candidates just before handing them to
// pum.vim. Candidates of other sources are shown as the UI "pum" shows them.
//
// Everything but showing (selecting, confirming, updating the info) is left
// to the UI "pum".
package luasrcpum

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda-cmp.vim/ext/ui/pum"
	"github.com/Shougo/goda.vim/sdk"
	"golang.org/x/text/width"

	"github.com/delphinus/goda-cmp-ext-luasrc.vim/ext/source/luasrc"
)

// Params are the parameters of the UI.
type Params struct {
	// Insert inserts the first item when the popup opens (the UI "pum"'s).
	Insert bool `json:"insert"`
	// AbbrMaxWidth is the display width the label and the detail are fitted
	// in. The detail is cut with "…" first, then the label. Set the same
	// value to max_columns.abbr of pum.vim. Default: 50
	AbbrMaxWidth int `json:"abbrMaxWidth"`
	// DetailMinGap is the least number of spaces between the label and the
	// detail. Default: 3
	DetailMinGap int `json:"detailMinGap"`
	// DetailMinWidth: the detail is not shown when less of it fits.
	// Default: 8
	DetailMinWidth int `json:"detailMinWidth"`
	// DetailHighlight is the highlight group of a detail without highlights
	// of its own. Default: "Comment"
	DetailHighlight string `json:"detailHighlight"`
}

func (p Params) withDefaults() Params {
	if p.AbbrMaxWidth <= 0 {
		p.AbbrMaxWidth = 50
	}
	if p.DetailMinGap <= 0 {
		p.DetailMinGap = 3
	}
	if p.DetailMinWidth <= 0 {
		p.DetailMinWidth = 8
	}
	if p.DetailHighlight == "" {
		p.DetailHighlight = "Comment"
	}
	return p
}

type ui struct {
	p     Params
	inner cmp.UI
	infos *infos
}

// infos is the info handed to pum.vim per candidate.
//
// The UI "pum" calls pum#update_current_item() on every UpdateInfo, even when
// the info has not changed. That fires PumCompleteChanged, which goda-cmp
// takes for selecting the item again, resolving it and updating its info
// again: selecting a candidate of a source with Resolve never stops. An info
// that has not changed is not handed on.
type infos struct {
	mu    sync.Mutex
	byRef map[string]string
}

// New returns the UI, named "luasrcpum". The parameters may be given (at most
// one Params).
func New(p ...Params) cmp.UI {
	var np Params
	if len(p) > 0 {
		np = p[0]
	}
	return newUI(np, &infos{byRef: map[string]string{}})
}

func newUI(p Params, in *infos) ui {
	return ui{p: p, inner: pum.New(pum.Params{Insert: p.Insert}), infos: in}
}

func (ui) Name() string { return "luasrcpum" }

// WithParams sets the parameters from Vim script (uiParams).
func (u ui) WithParams(p cmp.Params) (any, error) {
	np, err := cmp.PatchParams(u.p, p)
	if err != nil {
		return nil, err
	}
	return newUI(np, u.infos), nil
}

// highlight is one of the highlights of a pum.vim item. col is the byte
// position in the column, from 1, and width is in bytes.
type highlight struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	HLGroup string `json:"hl_group"`
	Col     int    `json:"col"`
	Width   int    `json:"width"`
}

func hl(group string, start, end int) highlight {
	// name is the name of a prop_type (Vim): one per group.
	return highlight{"abbr", "goda_luasrcpum_" + group, group, start + 1, end - start}
}

func (u ui) Show(ctx context.Context, h sdk.Host, r cmp.ShowRequest) error {
	p := u.p.withDefaults()
	u.infos.mu.Lock()
	clear(u.infos.byRef)
	for _, it := range r.Items {
		u.infos.byRef[it.Ref] = it.Info
	}
	u.infos.mu.Unlock()
	w := abbrWidth(r.Items, p)
	items := make([]any, len(r.Items))
	for i, it := range r.Items {
		m, err := decorate(it, w, p)
		if err != nil {
			return err
		}
		items[i] = m
	}
	// The arguments of the UI "pum"'s Show, with the items replaced.
	_, err := h.Call(ctx, "goda_cmp#ui#pum#show", map[string]any{
		"seq":    r.Context.Seq,
		"tick":   r.Context.Tick,
		"mode":   r.Context.Mode,
		"manual": r.Context.Manual,
		"col":    r.Col,
		"items":  items,
		"final":  r.Final,
		"insert": p.Insert,
	})
	return err
}

// abbrWidth is the width of the column abbr: the details are aligned to its
// right edge. It is the width all the candidates fit in, up to AbbrMaxWidth.
func abbrWidth(items []cmp.Item, p Params) int {
	w := 0
	for _, it := range items {
		n := displayWidth(it.Display())
		if r, ok := it.UserData.(luasrc.Ref); ok && r.Detail != "" {
			n += p.DetailMinGap + displayWidth(r.Detail)
		}
		w = max(w, n)
	}
	return min(w, p.AbbrMaxWidth)
}

// decorate makes the pum.vim item of a candidate.
func decorate(it cmp.Item, w int, p Params) (any, error) {
	r, ok := it.UserData.(luasrc.Ref)
	if !ok {
		return it, nil
	}
	b, err := json.Marshal(it)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	label := it.Display()
	abbr := label
	hls := spans(r.AbbrHighlights, 0, len(label))
	if a, start := layout(label, r.Detail, w, p); start >= 0 {
		abbr = a
		// Up to the "…" when the detail was cut.
		end := len(abbr) - start
		if strings.HasSuffix(abbr, "…") && !strings.HasSuffix(r.Detail, "…") {
			end -= len("…")
		}
		if len(r.DetailHighlights) > 0 {
			hls = append(hls, spans(r.DetailHighlights, start, start+end)...)
		} else {
			hls = append(hls, hl(p.DetailHighlight, start, len(abbr)))
		}
	}
	if displayWidth(abbr) > p.AbbrMaxWidth {
		abbr = truncate(abbr, p.AbbrMaxWidth)
		hls = clip(hls, len(abbr)-len("…"))
	}
	m["abbr"] = abbr
	m["highlights"] = hls
	return m, nil
}

// spans turns luasrc.Span into highlights of the column abbr, from its byte
// offset. Those after end are cut.
func spans(ss []luasrc.Span, offset, end int) []highlight {
	var out []highlight
	for _, s := range ss {
		e := min(offset+s.End, end)
		if offset+s.Start < e {
			out = append(out, hl(s.Group, offset+s.Start, e))
		}
	}
	return out
}

// layout puts detail to the right of label, the whole being of display width
// w, and returns it with the byte position the detail starts at. The detail
// is cut with "…" when it does not fit. Without the detail, it returns label
// and -1.
func layout(label, detail string, w int, p Params) (string, int) {
	if detail == "" {
		return label, -1
	}
	lw, dw := displayWidth(label), displayWidth(detail)
	room := w - lw - p.DetailMinGap
	if room < min(dw, p.DetailMinWidth) {
		return label, -1
	}
	if dw > room {
		detail = truncate(detail, room)
		dw = displayWidth(detail)
	}
	lead := label + strings.Repeat(" ", w-lw-dw)
	return lead + detail, len(lead)
}

// clip cuts the highlights after the byte position end.
func clip(hls []highlight, end int) []highlight {
	out := hls[:0]
	for _, h := range hls {
		start := h.Col - 1
		if start >= end {
			continue
		}
		h.Width = min(h.Width, end-start)
		out = append(out, h)
	}
	return out
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			w += 2
		default:
			w++
		}
	}
	return w
}

func truncate(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
		return s
	}
	const ellipsis = "…"
	budget := maxWidth - displayWidth(ellipsis)
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := displayWidth(string(r))
		if w+rw > budget {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + ellipsis
}

// The rest is left to the UI "pum".

func (u ui) Hide(ctx context.Context, h sdk.Host) error { return u.inner.Hide(ctx, h) }

func (u ui) SkipCompletion(ctx context.Context, h sdk.Host, c *cmp.Context) (bool, error) {
	return u.inner.(cmp.Skipper).SkipCompletion(ctx, h, c)
}

func (u ui) Visible(ctx context.Context, h sdk.Host) (bool, error) {
	return u.inner.(cmp.Visibler).Visible(ctx, h)
}

func (u ui) UpdateInfo(ctx context.Context, h sdk.Host, it cmp.Item) error {
	u.infos.mu.Lock()
	same := u.infos.byRef[it.Ref] == it.Info
	u.infos.byRef[it.Ref] = it.Info
	u.infos.mu.Unlock()
	if same {
		return nil
	}
	return u.inner.(cmp.InfoUpdater).UpdateInfo(ctx, h, it)
}

func (u ui) Modes() []string { return u.inner.(cmp.ModesUI).Modes() }

var (
	_ cmp.UI           = ui{}
	_ cmp.Skipper      = ui{}
	_ cmp.Visibler     = ui{}
	_ cmp.InfoUpdater  = ui{}
	_ cmp.ModesUI      = ui{}
	_ cmp.Configurable = ui{}
)
