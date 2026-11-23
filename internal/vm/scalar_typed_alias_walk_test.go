package vm

import "testing"

func scalarTypedWalkFixture(elementType string, n int) (root, nested Value, refs map[uint64]aliasRuntimeBacking) {
	nested = List(Int(1))
	nested.Ref = newValueRef()
	refs = map[uint64]aliasRuntimeBacking{nested.Ref: aliasBackingOf(nested)}
	grown := nested
	grown.List = append(append([]Value(nil), nested.List...), Int(2))
	pixels := make([]Value, n)
	for i := range pixels {
		pixels[i] = Int(int64(i % 256))
	}
	holder := List(pixels...)
	holder.Ref = newValueRef()
	holder.Type = "List<" + elementType + ">"
	root = Object("Canvas")
	root.Ref = newValueRef()
	root.Fields["pixels"] = holder
	root.Fields["series"] = grown
	return root, nested, refs
}

// A scalar-typed list is not walked element by element, and a stale nested
// collection beside it is still found.
func TestStaleNestedAliasWalkSkipsScalarTypedList(t *testing.T) {
	root, _, refs := scalarTypedWalkFixture("Integer", 5000)
	var visits uint64
	if !valueContainsStaleNestedCollectionAlias(root, root.Ref, refs, map[uint64]bool{}, &visits) {
		t.Fatal("stale nested collection beside a scalar list was missed")
	}
	if visits > 10 {
		t.Fatalf("visited %d values; scalar-typed list was walked", visits)
	}
}

// Untyped and Object-typed lists keep the element walk.
func TestStaleNestedAliasWalkKeepsObjectTypedList(t *testing.T) {
	for _, elementType := range []string{"Object", ""} {
		nested := List(Int(1))
		nested.Ref = newValueRef()
		refs := map[uint64]aliasRuntimeBacking{nested.Ref: aliasBackingOf(nested)}
		grown := nested
		grown.List = []Value{Int(1), Int(2)}
		holder := List(Int(0), grown)
		holder.Ref = newValueRef()
		if elementType != "" {
			holder.Type = "List<" + elementType + ">"
		}
		root := Object("Holder")
		root.Ref = newValueRef()
		root.Fields["items"] = holder
		if !valueContainsStaleNestedCollectionAlias(root, root.Ref, refs, map[uint64]bool{}, nil) {
			t.Fatalf("stale collection inside List<%s> was missed", elementType)
		}
	}
}

func TestScalarTypedCollectionCannotHoldAliasKinds(t *testing.T) {
	for typeName, want := range map[string]bool{
		"List<Integer>": true, "List<String>": true, "Set<Id>": true,
		"List<Object>": false, "List<List<Integer>>": false, "List<Account>": false, "": false,
	} {
		list := List(Int(1))
		list.Type = typeName
		if got := scalarTypedCollectionCannotHoldAlias(list); got != want {
			t.Errorf("%q: got %v want %v", typeName, got, want)
		}
	}
	if scalarTypedCollectionCannotHoldAlias(Map()) {
		t.Error("map must not qualify")
	}
}

func BenchmarkStaleNestedAliasWalkScalarTypedList(b *testing.B) {
	root, _, refs := scalarTypedWalkFixture("Integer", 512*384)
	seen := map[uint64]bool{}
	b.ReportAllocs()
	for b.Loop() {
		clear(seen)
		if !valueContainsStaleNestedCollectionAlias(root, root.Ref, refs, seen, nil) {
			b.Fatal("stale alias missed")
		}
	}
}
