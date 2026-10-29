package vm

import "testing"

// Rejection does not retain an index. Validate every child before allocating
// positions, including an unsafe child at the end of an otherwise flat list.
// This guard changes allocation only; fallback mutation semantics stay covered
// by alias_flat_test.go and alias_index_writer_routes_test.go.
func TestFlatObjectAliasIndexRejectedRootsDoNotAllocatePositions(t *testing.T) {
	for _, position := range []int{0, 128, 255} {
		root := List()
		for i := 0; i < 256; i++ {
			row := Object("OwnedRow")
			row.Fields["name"] = String("owned")
			root.List = append(root.List, row)
		}
		root.List[position].Fields["nested"] = List(Object("OwnedNested"))
		before := root.List[position].Fields["nested"].List[0].Ref
		allocations := testing.AllocsPerRun(10, func() {
			if _, ok := buildFlatObjectAliasIndex(root, nil); ok {
				t.Fatal("nested list accepted as flat")
			}
		})
		if allocations > 1 {
			t.Errorf("unsafe position %d: allocations %.0f, want <=1 before rejection", position, allocations)
		}
		if got := root.List[position].Fields["nested"].List[0].Ref; got != before {
			t.Fatal("rejection mutated the root")
		}
	}
}
