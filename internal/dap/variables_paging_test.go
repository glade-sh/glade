package dap

import (
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func pagedVariablesFixture(size int) (*Handler, int) {
	items := make([]vm.Value, 0, size)
	for i := 0; i < size; i++ {
		inner := vm.Object("Item")
		inner.Fields["n"] = vm.Int(int64(i))
		items = append(items, inner)
	}
	h := NewHandler(Snapshot{Vars: map[string]vm.Value{"big": vm.List(items...)}})
	locals := h.Handle(request(1, CommandVariables, map[string]any{"variablesReference": localsReference}))
	big := locals[0].(Response).Body.(map[string]any)["variables"].([]Variable)[0]
	return h, big.VariablesReference
}

func TestVariablesPageFormatsOnlyRequestedChildren(t *testing.T) {
	h, ref := pagedVariablesFixture(5000)
	before := h.nextVarRef
	page := h.Handle(request(2, CommandVariables, map[string]any{"variablesReference": ref, "start": 10, "count": 2}))
	variables := page[0].(Response).Body.(map[string]any)["variables"].([]Variable)
	if len(variables) != 2 || variables[0].Name != "10" || variables[1].Name != "11" {
		t.Fatalf("page = %#v", variables)
	}
	if variables[0].VariablesReference == 0 || variables[1].VariablesReference == 0 {
		t.Fatalf("expandable children lost their references: %#v", variables)
	}
	if grew := h.nextVarRef - before; grew != 2 {
		t.Fatalf("references allocated for a 2-item page = %d", grew)
	}
	// An out-of-range and a negative start keep their previous meaning.
	for _, tc := range []struct{ start, count, want int }{{-5, 3, 3}, {4999, 10, 1}, {6000, 10, 0}, {4998, 0, 2}} {
		got := h.Handle(request(3, CommandVariables, map[string]any{"variablesReference": ref, "start": tc.start, "count": tc.count}))
		list := got[0].(Response).Body.(map[string]any)["variables"].([]Variable)
		if len(list) != tc.want {
			t.Fatalf("start=%d count=%d returned %d variables, want %d", tc.start, tc.count, len(list), tc.want)
		}
	}
	first := h.Handle(request(4, CommandVariables, map[string]any{"variablesReference": ref, "start": -5, "count": 1}))
	if name := first[0].(Response).Body.(map[string]any)["variables"].([]Variable)[0].Name; name != strconv.Itoa(0) {
		t.Fatalf("negative start first name = %q", name)
	}
}
