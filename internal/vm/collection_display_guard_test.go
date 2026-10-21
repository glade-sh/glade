package vm

import (
	"errors"
	"testing"
)

// Crash guard only, native behavior uncaptured. These tests assert the Glade
// guard error, never a Salesforce answer: a self-referencing Map, List or Set
// must fail toString and string concatenation instead of overflowing the stack.
func selfReferencingCollections() map[string]Value {
	selfMap := Map()
	selfMap.Map["self"] = selfMap
	selfList := List(Null)
	selfList.List[0] = selfList
	selfSet := Set(Null)
	selfSet.Set[0] = selfSet
	return map[string]Value{"Map": selfMap, "List": selfList, "Set": selfSet}
}

func assertRecursiveCollectionDisplayError(t *testing.T, err error) {
	t.Helper()
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Message != recursiveCollectionDisplayMessage {
		t.Fatalf("want the crash-guard error, got %v", err)
	}
}

func TestCollectionDisplayGuardToString(t *testing.T) {
	for name, value := range selfReferencingCollections() {
		t.Run(name, func(t *testing.T) {
			_, handled, err := callObjectMember(value, "toString", nil)
			if !handled {
				t.Fatal("toString was not handled")
			}
			assertRecursiveCollectionDisplayError(t, err)
		})
	}
}

func TestCollectionDisplayGuardConcatenation(t *testing.T) {
	for name, value := range selfReferencingCollections() {
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			_, err := machine.evalBinary("+", String("text "), value, &Result{})
			assertRecursiveCollectionDisplayError(t, err)
		})
	}
}

// Control: nested and shared collections that do not contain themselves keep
// the text of the unguarded renderers.
func TestCollectionDisplayGuardLeavesNestedCollectionsUnchanged(t *testing.T) {
	machine := New(nil)
	inner := List(Int(1), String("a"))
	innerMap := Map()
	innerMap.Map["k"] = Int(1)
	outerMap := Map()
	outerMap.Map["inner"] = innerMap
	outerMap.Map["list"] = inner
	for name, value := range map[string]Value{
		"nested-list":  List(inner, Int(2)),
		"shared-list":  List(inner, inner),
		"nested-map":   outerMap,
		"nested-set":   Set(Int(3)),
		"scalar-list":  List(Int(1), Int(2)),
		"empty-nested": List(List(), Int(2)),
	} {
		t.Run(name, func(t *testing.T) {
			if collectionDisplayRecurses(value) {
				t.Fatal("a collection that does not contain itself was reported as recursive")
			}
			got, handled, err := callObjectMember(value, "toString", nil)
			if err != nil || !handled || got.Text != apexCollectionString(value) {
				t.Fatalf("toString=%q handled=%v err=%v; want %q", got.Text, handled, err, apexCollectionString(value))
			}
			want := ""
			var wantErr error
			switch value.Kind {
			case ValueList:
				want, wantErr = machine.displayList(value.List, &Result{})
			case ValueSet:
				want, wantErr = machine.displaySet(value.Set, &Result{})
			}
			if wantErr != nil {
				t.Fatal(wantErr)
			}
			text, err := machine.displayString(value, &Result{})
			if err != nil || (value.Kind != ValueMap && text != want) {
				t.Fatalf("display=%q err=%v; want %q", text, err, want)
			}
		})
	}
}
