package vm

import (
	"strings"
	"testing"
)

func init() {
	// Every strict heap check in this package's tests must equal the full walk.
	heapVerifyAgainstFullWalk = true
}

// Static scalars are summed once and only rewritten fields change that sum:
// the strict per-statement check must track every static write route.
func TestStrictHeapTracksStaticWritesAcrossStatements(t *testing.T) {
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	statics := map[string]Field{
		"label": {Name: "label", Static: true, Type: "String", Value: String("a")},
		"count": {Name: "count", Static: true, Type: "Integer", Value: Int(1)},
		"items": {Name: "items", Static: true, Type: "List<Integer>", Value: List()},
	}
	if err := machine.RegisterClass(Class{Name: "HeapStaticHolder", StaticFields: statics}); err != nil {
		t.Fatal(err)
	}
	source := `
HeapStaticHolder.label = 'abcdefghij';
HeapStaticHolder.count = 5;
HeapStaticHolder.items.add(1);
HeapStaticHolder.items.add(2);
HeapStaticHolder.label += 'klmnopqrst';
Integer before = Limits.getHeapSize();
HeapStaticHolder.label = 'z';
Integer after = Limits.getHeapSize();
// before declares one Integer local the after read also counts: 19 chars - (6 + 8).
System.assert(before - after == 5, 'before ' + before + ' after ' + after);
`
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if got, want := machine.currentHeapSize(), machine.currentHeapSizeFullWalk(); got != want {
		t.Fatalf("heap %d != full walk %d", got, want)
	}
}

// The strict check must still stop on the statement that passes a local cap
// when the growth is a static container, with scalar statics cached.
func TestStrictHeapLocalCapStillStopsOnStaticGrowth(t *testing.T) {
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	caps := defaultLimitCaps()
	caps.HeapSize = 2000
	machine.SetLimitCaps(caps)
	statics := map[string]Field{
		"label": {Name: "label", Static: true, Type: "String", Value: String(strings.Repeat("s", 100))},
		"items": {Name: "items", Static: true, Type: "List<String>", Value: List()},
	}
	if err := machine.RegisterClass(Class{Name: "HeapCapHolder", StaticFields: statics}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`for (Integer i = 0; i < 100; i++) { HeapCapHolder.items.add('0123456789012345678901234567890123456789'); }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Too many heapSize") {
		t.Fatalf("err = %v, want heap LimitException", err)
	}
}
