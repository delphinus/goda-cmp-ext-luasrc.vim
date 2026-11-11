package luasrcpum

import (
	"slices"
	"testing"

	"github.com/Shougo/goda-cmp.vim/cmp"

	"github.com/delphinus/goda-cmp-ext-luasrc.vim/ext/source/luasrc"
)

func TestLayout(t *testing.T) {
	p := Params{}.withDefaults()
	for _, tc := range []struct {
		label, detail string
		w             int
		want          string
		start         int
	}{
		{"Println", "", 20, "Println", -1},
		{"Println", "func()", 20, "Println       func()", 14},
		// The detail is cut to what fits.
		{"Println", "func(a ...any) (n int, err error)", 20, "Println   func(a ..…", 10},
		// Less than DetailMinWidth fits: no detail.
		{"Println", "func(a ...any)", 15, "Println", -1},
	} {
		got, start := layout(tc.label, tc.detail, tc.w, p)
		if got != tc.want || start != tc.start {
			t.Errorf("layout(%q, %q, %d) = %q, %d, want %q, %d", tc.label, tc.detail, tc.w, got, start, tc.want, tc.start)
		}
	}
}

func TestDecorate(t *testing.T) {
	p := Params{}.withDefaults()
	it := cmp.Item{
		Word: "Println",
		UserData: luasrc.Ref{
			Detail:           "func()",
			AbbrHighlights:   []luasrc.Span{{Group: "@function", Start: 0, End: 7}},
			DetailHighlights: []luasrc.Span{{Group: "@keyword", Start: 0, End: 4}},
		},
	}
	v, err := decorate(it, 20, p)
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if got := m["abbr"]; got != "Println       func()" {
		t.Errorf("abbr = %q", got)
	}
	want := []highlight{hl("@function", 0, 7), hl("@keyword", 14, 18)}
	if got := m["highlights"].([]highlight); !slices.Equal(got, want) {
		t.Errorf("highlights = %v, want %v", got, want)
	}

	// A detail without highlights gets DetailHighlight.
	it.UserData = luasrc.Ref{Detail: "func()"}
	v, _ = decorate(it, 20, p)
	want = []highlight{hl("Comment", 14, 20)}
	if got := v.(map[string]any)["highlights"].([]highlight); !slices.Equal(got, want) {
		t.Errorf("highlights = %v, want %v", got, want)
	}

	// Candidates of other sources are left as they are.
	plain := cmp.Item{Word: "foo"}
	if v, _ := decorate(plain, 20, p); v.(cmp.Item).Word != "foo" {
		t.Errorf("decorate(plain) = %v", v)
	}
}

func TestTruncateClip(t *testing.T) {
	p := Params{AbbrMaxWidth: 10}.withDefaults()
	it := cmp.Item{
		Word:     "averyveryverylongname",
		UserData: luasrc.Ref{AbbrHighlights: []luasrc.Span{{Group: "@variable", Start: 0, End: 21}}},
	}
	v, _ := decorate(it, 10, p)
	m := v.(map[string]any)
	if got := m["abbr"]; got != "averyvery…" {
		t.Errorf("abbr = %q", got)
	}
	want := []highlight{hl("@variable", 0, 9)}
	if got := m["highlights"].([]highlight); !slices.Equal(got, want) {
		t.Errorf("highlights = %v, want %v", got, want)
	}
}
