package vm

import (
	"fmt"
	"testing"
)

func TestAssignPathSingleFieldPropagatesEachSnapshotOnce(t *testing.T) {
	for _, width := range []int{32, 256} {
		t.Run(fmt.Sprintf("sibling_%d", width), func(t *testing.T) {
			machine, target := assignPathAliasWorkFixture(t, width)
			expected, expectedTarget := assignPathAliasWorkFixture(t, width)
			previous := snapshotAlias(expectedTarget)
			expectedRecorder := NewPerfRecorder()
			expected.SetPerfRecorder(expectedRecorder)
			expected.setObjectFieldValue(&expectedTarget, "Value", String("after"))
			expected.propagateAliasSnapshotToScope(expected.Globals, previous, expectedTarget)
			expected.propagateAliasSnapshotToStatics(previous, expectedTarget)
			want := expectedRecorder.Snapshot()

			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			if err := machine.assignPath(target, []string{"Value"}, String("after")); err != nil {
				t.Fatal(err)
			}
			got := recorder.Snapshot()
			if got.ScopeAlias.Calls != 1 || got.StaticAlias.Calls != 1 {
				t.Errorf("single-field propagation calls: scope=%d static=%d, want one each", got.ScopeAlias.Calls, got.StaticAlias.Calls)
			}
			if got.ScopeAlias.Roots != want.ScopeAlias.Roots || got.ScopeAlias.RecursiveVisits != want.ScopeAlias.RecursiveVisits {
				t.Errorf("single-field scope work: roots=%d recursive=%d, want one pass roots=%d recursive=%d", got.ScopeAlias.Roots, got.ScopeAlias.RecursiveVisits, want.ScopeAlias.Roots, want.ScopeAlias.RecursiveVisits)
			}
			if want.ScopeAlias.RecursiveVisits <= uint64(width) {
				t.Fatalf("fixture did not traverse the large sibling: recursive=%d width=%d", want.ScopeAlias.RecursiveVisits, width)
			}
		})
	}
}

func TestAssignPathFieldPropagationPreservesAliases(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested_%v", nested), func(t *testing.T) {
			machine, target := assignPathAliasWorkFixture(t, 32)
			root, path := target, []string{"Value"}
			if nested {
				root = Object("AliasWorkNode")
				root.Fields["Child"] = machine.cloneValuePreserveRefs(target)
				machine.Globals["owner"] = machine.cloneValuePreserveRefs(root)
				assignPathAliasWorkStatic(machine, "Owner", root)
				path = []string{"Child", "Value"}
			}
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			if err := machine.assignPath(root, path, String("after")); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"root", "direct"} {
				assertAssignPathAliasWorkValue(t, name, machine.Globals[name], "after")
			}
			assertAssignPathAliasWorkValue(t, "nested alias", machine.Globals["holder"].Fields["Child"], "after")
			assertAssignPathAliasWorkValue(t, "independent clone", machine.Globals["clone"], "before")
			statics := machine.Classes["AliasWorkRegistry"].StaticFields
			assertAssignPathAliasWorkValue(t, "direct static", statics["Direct"].Value, "after")
			assertAssignPathAliasWorkValue(t, "nested static", statics["Nested"].Value.Fields["Child"], "after")
			if nested {
				assertAssignPathAliasWorkValue(t, "owner alias", machine.Globals["owner"].Fields["Child"], "after")
				assertAssignPathAliasWorkValue(t, "owner static", statics["Owner"].Value.Fields["Child"], "after")
				got := recorder.Snapshot()
				if got.ScopeAlias.Calls != 2 || got.StaticAlias.Calls != 2 {
					t.Fatalf("distinct leaf/root propagation calls: scope=%d static=%d, want two each", got.ScopeAlias.Calls, got.StaticAlias.Calls)
				}
			}
		})
	}
}

func TestAssignPathFieldPropagationKeepsRuntimeClonesIsolated(t *testing.T) {
	template, _ := assignPathAliasWorkFixture(t, 32)
	first, sibling := template.CloneRuntime(nil), template.CloneRuntime(nil)
	target := first.Classes["AliasWorkRegistry"].StaticFields["Direct"].Value
	first.Globals["alias"] = first.cloneValuePreserveRefs(target)
	if err := first.assignPath(target, []string{"Value"}, String("after")); err != nil {
		t.Fatal(err)
	}
	assertAssignPathAliasWorkValue(t, "runtime alias", first.Globals["alias"], "after")
	assertAssignPathAliasWorkValue(t, "runtime static", first.Classes["AliasWorkRegistry"].StaticFields["Direct"].Value, "after")
	assertAssignPathAliasWorkValue(t, "template static", template.Classes["AliasWorkRegistry"].StaticFields["Direct"].Value, "before")
	assertAssignPathAliasWorkValue(t, "sibling runtime static", sibling.Classes["AliasWorkRegistry"].StaticFields["Direct"].Value, "before")
}

func assignPathAliasWorkFixture(t *testing.T, width int) (*VM, Value) {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "AliasWorkNode",
		Fields: map[string]Field{
			"Value": {Name: "Value", Type: "String", Access: "public"},
			"Child": {Name: "Child", Type: "AliasWorkNode", Access: "public"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "AliasWorkRegistry", StaticFields: map[string]Field{}}); err != nil {
		t.Fatal(err)
	}
	target := Object("AliasWorkNode")
	target.Fields["Value"] = String("before")
	machine.Globals["root"] = machine.cloneValuePreserveRefs(target)
	machine.Globals["direct"] = machine.cloneValuePreserveRefs(target)
	holder := Object("AliasWorkNode")
	holder.Fields["Child"] = machine.cloneValuePreserveRefs(target)
	machine.Globals["holder"] = machine.cloneValuePreserveRefs(holder)
	machine.Globals["clone"] = cloneValue(target)
	assignPathAliasWorkStatic(machine, "Direct", target)
	assignPathAliasWorkStatic(machine, "Nested", holder)

	sibling := Map()
	sibling.Type = "Map<String,Object>"
	for i := range width {
		key := String(fmt.Sprintf("item_%d", i))
		child := Object("AliasWorkNode")
		child.Fields["Value"] = String("unrelated")
		sibling.Map[mapKey(key)] = child
		sibling.MapKeys[mapKey(key)] = key
		sibling.MapOrder = append(sibling.MapOrder, mapKey(key))
	}
	machine.Globals["sibling"] = sibling
	previous := snapshotAlias(target)
	if machine.valueContainsAliasRefCached(sibling, previous, make(map[uint64]bool)) {
		t.Fatal("unrelated sibling unexpectedly contained target")
	}
	var probe scopeAliasProbe
	if machine.valueContainsAliasRefCachedWithProbe(sibling, previous, make(map[uint64]bool), &probe) || probe.containmentCacheHits != 1 {
		t.Fatalf("unrelated sibling did not warm its negative containment cache: %#v", probe)
	}
	// Field mutation invalidates this cache epoch. The measured assignment must
	// perform its required traversal once, including the unrelated large root.
	return machine, target
}

func assignPathAliasWorkStatic(machine *VM, name string, value Value) {
	class := machine.Classes["AliasWorkRegistry"]
	class.StaticFields[name] = Field{
		Name: name, Type: "AliasWorkNode", Static: true,
		Value: machine.cloneValuePreserveRefs(value), InitialValue: machine.cloneValuePreserveRefs(value),
	}
	machine.Classes["AliasWorkRegistry"] = class
	machine.staticValueRefs, machine.staticValueRefFields = nil, nil
}

func assertAssignPathAliasWorkValue(t *testing.T, label string, value Value, want string) {
	t.Helper()
	if got := value.Fields["Value"]; got.Kind != ValueString || got.Text != want {
		t.Fatalf("%s Value = %#v, want %q", label, got, want)
	}
}
