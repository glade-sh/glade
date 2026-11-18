package dap

import (
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
		if len(body.Variables) != 1 || body.Variables[0].Value != previewPrefix(vm.List(values...).String()) {
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

func TestPreviewValueEveryKind(t *testing.T) {
	page := vm.Object("PageReference")
	page.Fields["url"] = vm.String("/short")
	page.Fields["unused"] = vm.String(strings.Repeat("x", 5000))
	object := vm.Object("Example")
	object.Fields["value"] = vm.String("short")
	object.Fields["unused"] = vm.String(strings.Repeat("x", 5000))
	record := vm.Object("Account")
	record.Fields["Name"] = vm.String("Acme")
	values := map[string]vm.Value{
		"null":                  vm.Null,
		"integer":               vm.Int(42),
		"decimal":               {Kind: vm.ValueDecimal, Text: "1.25"},
		"boolean":               {Kind: vm.ValueBool, Bool: true},
		"string":                vm.String("hello"),
		"list":                  vm.List(vm.Int(1), vm.Null),
		"set":                   {Kind: vm.ValueSet, Set: []vm.Value{vm.String("a"), vm.Int(2)}},
		"map":                   {Kind: vm.ValueMap, Map: map[string]vm.Value{"a": vm.Int(1)}},
		"sobject":               record,
		"object omitted fields": object,
		"page omitted fields":   page,
		"nested omitted fields": vm.List(object, page),
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			want := value.String()
			if len(want) > 256 {
				t.Fatalf("test value does not fit: %d", len(want))
			}
			if got := previewValue(value); got != want {
				t.Fatalf("preview = %q, want %q", got, want)
			}
		})
	}
}

func TestPreviewValueRenderedBoundary(t *testing.T) {
	cyclic := vm.List(vm.Null)
	cyclic.List[0] = cyclic
	items := make([]vm.Value, 65)
	for i := range items {
		items[i] = vm.Int(1)
		items[i].Type = strings.Repeat("metadata", 100)
	}
	for name, value := range map[string]vm.Value{
		"many short elements": vm.List(items...),
		"at limit":            vm.String(strings.Repeat("x", 256)),
		"over limit":          vm.String(strings.Repeat("x", 257)),
		"multibyte at limit":  vm.String(strings.Repeat("é", 128)),
		"multibyte over":      vm.String("x" + strings.Repeat("é", 128)),
		"cyclic list":         cyclic,
	} {
		t.Run(name, func(t *testing.T) {
			got := previewValue(value)
			if name == "cyclic list" {
				if got != strings.Repeat("List[", 52)[:256]+"..." {
					t.Fatalf("preview = %q", got)
				}
				return
			}
			want := value.String()
			if len(want) <= 256 {
				if got != want {
					t.Fatalf("preview = %q, want exact %q", got, want)
				}
			} else if got != previewPrefix(want) || len(got) > 259 {
				t.Fatalf("preview = %q, want truncated value with marker", got)
			}
		})
	}
}

func TestPreviewValueOmitsUnusedObjectFields(t *testing.T) {
	object := vm.Object("Example")
	object.Fields["value"] = vm.String("short")
	object.Fields["unused"] = vm.String(strings.Repeat("x", 128<<10))
	if got, want := previewValue(object), object.String(); got != want {
		t.Fatalf("preview = %q, want %q", got, want)
	}
}

func TestPreviewValueRespectsObjectFormatterPrecedence(t *testing.T) {
	for _, typ := range []string{"PageReference", "Database.QueryLocator", "Invocable.Action"} {
		t.Run(typ, func(t *testing.T) {
			object := vm.Object(typ)
			object.Fields["value"] = vm.String("ignored")
			if got, want := previewValue(object), object.String(); got != want {
				t.Fatalf("preview = %q, want %q", got, want)
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
		if want := "{List[" + strings.Repeat("<>, ", 63)[:250] + "..."; got != want {
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

// previewPrefix is the expected over-bound preview: the first 256 bytes of
// the rendered text, without a split rune, then the marker.
func previewPrefix(text string) string {
	text = text[:256]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + "..."
}
