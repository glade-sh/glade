package vm

import "testing"

// Most eligible rows have distinct identities. One position per identity must
// fit in the index itself; only real duplicate occurrences need a slice.
func TestFlatObjectAliasIndexDistinctPositionsHaveBoundedAllocations(t *testing.T) {
	root := List()
	for range 256 {
		row := Object("OwnedFlatRow")
		row.Fields["name"] = String("owned")
		root.List = append(root.List, row)
	}
	allocations := testing.AllocsPerRun(10, func() {
		if _, ok := buildFlatObjectAliasIndex(root, nil); !ok {
			t.Fatal("flat root rejected")
		}
	})
	if allocations > 8 {
		t.Fatalf("allocations %.0f, want <=8 for 256 distinct positions", allocations)
	}
}
