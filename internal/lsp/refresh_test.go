package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestSaveRefreshRecognizesNotification(t *testing.T) {
	h := NewHandler(sampleIndex(t))
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": "file:///saved.cls"}})
	if _, err := h.handle("textDocument/didSave", params); err != nil {
		t.Fatalf("didSave rejected: %#v", err)
	}
}

func TestRefreshCoalesces(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	started := make(chan int, 3)
	release := make(chan struct{})
	var calls atomic.Int32
	h.SetRefresher(func(ctx context.Context, request RefreshRequest) (Base, error) {
		n := int(calls.Add(1))
		started <- n
		if n == 1 {
			select {
			case <-release:
			case <-ctx.Done():
				return Base{}, ctx.Err()
			}
		}
		return Base{Index: idx, Analyze: func(typesys.Index) sema.Result { return sema.Result{} }}, nil
	})
	// Make initial analysis cheap and deterministic before the worker begins.
	h.gen.Load().analyzer = func(typesys.Index) sema.Result { return sema.Result{} }
	c := startServe(t, h)
	msg := func(n int) map[string]any {
		return map[string]any{"method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]any{"uri": fmt.Sprintf("file:///save%d.cls", n)}}}
	}
	c.send(msg(0))
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh did not start")
	}
	for i := 1; i <= 5; i++ {
		c.send(msg(i))
	}
	// Initialize is handled synchronously after the preceding notifications.
	c.send(map[string]any{"id": 99, "method": "initialize", "params": map[string]any{}})
	c.mustNext(3 * time.Second)
	close(release)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("coalesced refresh did not start")
	}
	c.send(map[string]any{"id": 1, "method": "shutdown"})
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("refresh calls = %d, want 2", n)
	}
}

func TestRefreshStaleGenerationRetriesOnce(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	h.gen.Load().analyzer = func(typesys.Index) sema.Result { return sema.Result{} }
	calls := make(chan struct{}, 10)
	h.SetRefresher(func(context.Context, RefreshRequest) (Base, error) {
		select {
		case calls <- struct{}{}:
		default:
		}
		return Base{Index: idx, Analyze: func(typesys.Index) sema.Result { return sema.Result{} }}, nil
	})
	h.SetGenerationValidator(func(typesys.Index) bool { return false })
	c := startServe(t, h)
	c.send(map[string]any{"method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///save.cls"}}})
	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(3 * time.Second):
			t.Fatal("missing refresh attempt")
		}
	}
	select {
	case <-calls:
		t.Error("stale refresh retried more than once")
	case <-time.After(200 * time.Millisecond):
	}
	c.send(map[string]any{"id": 1, "method": "shutdown"})
	<-c.served
}

func TestRefreshDoesNotPublishInvalidGeneration(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	before := h.gen.Load()
	before.analyzer = func(typesys.Index) sema.Result { return sema.Result{} }
	h.SetRefresher(func(context.Context, RefreshRequest) (Base, error) {
		return Base{Index: idx, Analyze: func(typesys.Index) sema.Result { return sema.Result{} }}, nil
	})
	validated := make(chan struct{}, 2)
	h.SetGenerationValidator(func(typesys.Index) bool { validated <- struct{}{}; return false })
	c := startServe(t, h)
	c.send(map[string]any{"method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]any{"uri": "file:///save.cls"}}})
	for i := 0; i < 2; i++ {
		select {
		case <-validated:
		case <-time.After(3 * time.Second):
			t.Fatal("missing validation attempt")
		}
	}
	c.send(map[string]any{"id": 1, "method": "shutdown"})
	if err := <-c.served; err != nil {
		t.Fatal(err)
	}
	if h.gen.Load() != before {
		t.Fatal("invalid generation became visible")
	}
}

func TestConcurrentRequestsDuringRefresh(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	h.SetRefresher(func(context.Context, RefreshRequest) (Base, error) {
		return Base{Index: idx, Analyze: func(typesys.Index) sema.Result { return sema.Result{} }}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				h.Hover(HoverParams{})
				h.Completion(CompletionParams{})
				h.References(ReferenceParams{})
			}
		}()
	}
	for i := 0; i < 10; i++ {
		if err := h.Refresh(context.Background(), []string{"changed.cls"}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}
