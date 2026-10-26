package vm

import (
	"fmt"
	"testing"
)

func readonlyCoercionFixture(size int, objectView bool) (Value, string) {
	items := List()
	items.Type = "List<Map<String,String>>"
	for i := 0; i < size; i++ {
		child := Map()
		child.Type = "Map<String,String>"
		key := mapKey(String("name"))
		child.Map[key] = String("leaf")
		child.MapKeys[key] = String("name")
		child.MapOrder = []string{key}
		items.List = append(items.List, child)
	}
	root := Map()
	root.Type = "Map<Object,Object>"
	key := mapKey(String("items"))
	root.Map[key] = items
	root.MapKeys[key] = String("items")
	root.MapOrder = []string{key}
	target := "Map<String,List<Map<String,String>>>"
	if objectView {
		target = "Map<String,Object>"
	}
	return root, target
}

func TestMapCoercionReadOnlyAllocationDoesNotScaleWithPayload(t *testing.T) {
	// Native alias observations: alias_graph_boundaries/api{62,67}/org.tsv,
	// direct and coercion rows, under internal/apextest/testdata/conformance.
	for _, objectView := range []bool{false, true} {
		t.Run(fmt.Sprintf("object-view=%v", objectView), func(t *testing.T) {
			machine := New(nil)
			allocations := func(size int) float64 {
				value, target := readonlyCoercionFixture(size, objectView)
				return testing.AllocsPerRun(10, func() {
					got, err := machine.coerceAssignable(target, value)
					if err != nil {
						t.Fatal(err)
					}
					key := value.MapOrder[0]
					if got.Ref != value.Ref || !sameMapBacking(got.Map, value.Map) ||
						!sameSliceBacking(got.Map[key].List, value.Map[key].List) {
						t.Fatal("read-only coercion replaced collection identity")
					}
				})
			}
			small, large := allocations(1), allocations(64)
			t.Logf("allocs/op: one child=%.0f, 64 children=%.0f", small, large)
			if large > small+4 {
				t.Fatalf("read-only coercion allocations scale with payload: %.0f -> %.0f (growth limit 4)", small, large)
			}
		})
	}
}

func BenchmarkMapCoercionReadOnlyPayload(b *testing.B) {
	for _, objectView := range []bool{false, true} {
		b.Run(fmt.Sprintf("object-view=%v", objectView), func(b *testing.B) {
			machine := New(nil)
			value, target := readonlyCoercionFixture(64, objectView)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := machine.coerceAssignable(target, value); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
