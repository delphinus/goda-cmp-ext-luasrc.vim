package luasrc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Shougo/goda.vim/sdk"
)

// fakeHost answers a Notify of a Lua module with onNotify (the id the Lua
// side would reply to).
type fakeHost struct {
	onNotify func(id int64)
}

func (fakeHost) Meta() sdk.Meta { return sdk.Meta{} }

func (fakeHost) Call(context.Context, string, ...any) (any, error) {
	return nil, errors.New("Call is not used")
}

func (h fakeHost) Notify(_ context.Context, _ string, args ...any) error {
	// args: the expression of luaeval, then _A (the arguments of the Lua
	// function, with the id).
	if a, ok := args[1].(map[string]any); ok && h.onNotify != nil {
		h.onNotify(a["id"].(int64))
	}
	return nil
}

func (fakeHost) Eval(context.Context, string) (any, error) { return nil, nil }
func (fakeHost) Cmd(context.Context, string) error         { return nil }

func sendReply(t *testing.T, b *Bridge, id int64, payload any) {
	t.Helper()
	if _, err := b.Handlers()["reply"](context.Background(), fakeHost{}, []any{id, payload}); err != nil {
		t.Error(err)
	}
}

func TestCallTakesTheReply(t *testing.T) {
	b := NewBridge()
	h := fakeHost{onNotify: func(id int64) {
		// The reply comes later, as a notification of its own.
		go sendReply(t, b, id, map[string]any{"info": "the info"})
	}}
	var r struct {
		Info string `json:"info"`
	}
	if err := b.call(context.Background(), h, "m", "resolve", map[string]any{}, &r); err != nil {
		t.Fatal(err)
	}
	if r.Info != "the info" {
		t.Errorf("info = %q", r.Info)
	}
	if len(b.waiting) != 0 {
		t.Errorf("still waiting for %v", b.waiting)
	}
}

// A reply that comes after call stopped waiting is dropped, and does not go to
// the next call.
func TestLateReplyIsDropped(t *testing.T) {
	b := NewBridge()
	var first int64
	h := fakeHost{onNotify: func(id int64) { first = id }}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var r map[string]any
	if err := b.call(ctx, h, "m", "complete", map[string]any{}, &r); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	sendReply(t, b, first, map[string]any{"late": true})
	if len(b.waiting) != 0 {
		t.Errorf("still waiting for %v", b.waiting)
	}

	h = fakeHost{onNotify: func(id int64) {
		go sendReply(t, b, id, map[string]any{"late": false})
	}}
	r = nil
	if err := b.call(context.Background(), h, "m", "complete", map[string]any{}, &r); err != nil {
		t.Fatal(err)
	}
	if r["late"] != false {
		t.Errorf("reply = %v, want the one of the second call", r)
	}
}

func TestReplyNeedsIDAndPayload(t *testing.T) {
	b := NewBridge()
	if _, err := b.Handlers()["reply"](context.Background(), fakeHost{}, []any{int64(1)}); err == nil {
		t.Error("reply without a payload: no error")
	}
}
