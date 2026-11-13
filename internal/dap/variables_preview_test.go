package dap

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestVariablesLargeListIsBounded(t *testing.T) {
	values := make([]vm.Value, 200000)
	for i := range values {
		values[i] = vm.Int(int64(i))
	}
	h := NewHandler(Snapshot{Vars: map[string]vm.Value{"items": vm.List(values...)}})
	done := make(chan Response, 1)
	go func() {
		done <- h.handleVariables(request(1, CommandVariables, map[string]any{"variablesReference": localsReference}))
	}()
	select {
	case response := <-done:
		var body struct {
			Variables []Variable `json:"variables"`
		}
		decodeBody(t, response, &body)
		if len(body.Variables) != 1 || len(body.Variables[0].Value) > 300 || !strings.HasPrefix(body.Variables[0].Value, "List(200000)") {
			t.Fatalf("unexpected preview: %#v", body.Variables)
		}
	case <-time.After(time.Second):
		t.Fatal("large list preview exceeded one second")
	}
}

func TestPreviewValueKeepsSmallValues(t *testing.T) {
	object := vm.Object("Example")
	object.Fields["name"] = vm.String("sample")
	values := []vm.Value{vm.Int(7), vm.String("hello"), vm.List(vm.Int(1), vm.Int(2), vm.Int(3)), {Kind: vm.ValueMap, Map: map[string]vm.Value{"one": vm.Int(1)}}, object}
	for _, value := range values {
		if got, want := previewValue(value), value.String(); got != want {
			t.Errorf("preview = %q, want %q", got, want)
		}
	}
}

func TestPreviewValueRenderedBoundary(t *testing.T) {
	items := make([]vm.Value, 65)
	for i := range items {
		items[i] = vm.Int(1)
	}
	for name, value := range map[string]vm.Value{
		"many short elements": vm.List(items...),
		"at limit":            vm.String(strings.Repeat("x", 256)),
		"over limit":          vm.String(strings.Repeat("x", 257)),
	} {
		t.Run(name, func(t *testing.T) {
			want := value.String()
			got := previewValue(value)
			if len(want) <= 256 {
				if got != want {
					t.Fatalf("preview = %q, want exact %q", got, want)
				}
			} else if got != want[:256]+"..." {
				t.Fatalf("preview = %q, want truncated value with marker", got)
			}
		})
	}
}

func TestVariablesConcurrentPauses(t *testing.T) {
	h := NewHandler(Snapshot{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			h.ApplyPause(vm.DebugPause{Vars: map[string]vm.Value{"items": vm.List(vm.Int(int64(i)))}})
		}
	}()
	for i := 0; i < 100; i++ {
		h.handleVariables(request(1, CommandVariables, map[string]any{"variablesReference": localsReference}))
	}
	<-done
}

func TestPreviewBoundsLargeMapKey(t *testing.T) {
	key := vm.List(make([]vm.Value, 200000)...)
	value := vm.Value{Kind: vm.ValueMap, Map: map[string]vm.Value{"key": vm.Int(1)}, MapKeys: map[string]vm.Value{"key": key}}
	done := make(chan string, 1)
	go func() { done <- previewValue(value) }()
	select {
	case got := <-done:
		if len(got) > 300 || !strings.HasPrefix(got, "Map(1)") {
			t.Fatalf("unbounded map key preview: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("large map key preview exceeded one second")
	}
}

func TestPreviewBoundsNestedScalarAllocation(t *testing.T) {
	text := vm.String(strings.Repeat("x", 128<<10))
	values := make([]vm.Value, 63)
	for i := range values {
		values[i] = text
	}
	value := vm.List(values...)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := previewValue(value)
	runtime.ReadMemStats(&after)
	if len(got) > 259 {
		t.Fatalf("preview length = %d", len(got))
	}
	if n := after.TotalAlloc - before.TotalAlloc; n > 2<<20 {
		t.Fatalf("preview allocated %d bytes", n)
	}
}
