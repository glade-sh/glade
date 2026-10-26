package vm

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

func TestSetAppendEntryAllocatesOneBufferPerSlice(t *testing.T) {
	for _, size := range []int{16, 64, 1024, 4096} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			original := Set()
			original.Set = make([]Value, size, size+8)
			original.setInsertionHashes = make([]setInsertionHash, size, size+8)
			for i := range original.Set {
				original.Set[i] = Int(int64(i))
				original.setInsertionHashes[i] = setInsertionHash{key: fmt.Sprint(i), tracked: true}
			}
			var got Value
			allocs := testing.AllocsPerRun(20, func() {
				got = appendSetEntry(original, Int(999), setInsertionHash{key: "new", tracked: true})
			})
			t.Logf("size=%d allocations=%.0f", size, allocs)
			if allocs > 2 {
				t.Errorf("append allocated %.0f buffers; want at most one element buffer and one history buffer", allocs)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for i := 0; i < 20; i++ {
				got = appendSetEntry(original, Int(999), setInsertionHash{key: "new", tracked: true})
			}
			runtime.ReadMemStats(&after)
			bytes := (after.TotalAlloc - before.TotalAlloc) / 20
			// Allow allocator page rounding for two final-length buffers.
			budget := uint64(size+1)*uint64(unsafe.Sizeof(Value{})+unsafe.Sizeof(setInsertionHash{})) + 2*8192
			t.Logf("size=%d bytes/append=%d budget=%d", size, bytes, budget)
			if bytes > budget {
				t.Errorf("append allocated %d bytes; want <=%d for two final-length buffers", bytes, budget)
			}
			if got.Ref != original.Ref || len(got.Set) != size+1 || got.Set[size].Int != 999 || got.setInsertionHashAt(size).key != "new" {
				t.Fatal("append changed identity or lost the new element/history")
			}
			// Both old buffers have spare capacity. Neither may be reused: a
			// failed store or a sibling snapshot must retain its old contents.
			got.Set[0] = Int(-1)
			got.setInsertionHashes[0] = setInsertionHash{key: "changed", tracked: true}
			if original.Set[0].Int != 0 || original.setInsertionHashAt(0).key != "0" || len(original.Set) != size {
				t.Fatal("append mutated the previous snapshot")
			}
		})
	}
}

// This compatibility adapter is another caller of the shared append helper;
// its Map-like put operation is local runtime plumbing, not native Set Apex.
func TestSetAppendDisabledTriggersPutRoutePreservesSnapshot(t *testing.T) {
	machine := New(nil)
	previous := Set(String("existing"))
	machine.Globals["disabledTriggers"] = previous
	got, handled, err := machine.callSetValueMember("disabledTriggers", previous, "put", []Value{String("new"), Int(1)}, &Result{})
	if err != nil || !handled || got.Int != 0 {
		t.Fatalf("put: %v %v %v", got, handled, err)
	}
	updated := machine.Globals["disabledTriggers"]
	if len(updated.Set) != 2 || len(previous.Set) != 1 || updated.Set[1].Text != "new" {
		t.Fatal("put lost the appended value or changed its previous snapshot")
	}
}

// The VM also accepts iterator objects in addAll. API 62 rejects a custom
// Iterable argument, so this is a local compatibility route, not native credit.
func TestSetAppendAddAllIteratorCompatibilityRoutePreservesSnapshot(t *testing.T) {
	machine := New(nil)
	previous := Set(Int(1))
	machine.Globals["values"] = previous
	iterator := machine.newCollectionIterator(List(Int(2), Int(3)))
	got, handled, err := machine.callSetValueMember("values", previous, "addAll", []Value{iterator}, &Result{})
	if err != nil || !handled || !got.Bool {
		t.Fatalf("iterator addAll: %v %v %v", got, handled, err)
	}
	updated := machine.Globals["values"]
	if len(updated.Set) != 3 || len(previous.Set) != 1 || updated.Set[2].Int != 3 {
		t.Fatal("iterator addAll lost an element or changed its previous snapshot")
	}
}

func TestSetAppendEntryEmptyAndUntrackedHistoryRoutes(t *testing.T) {
	for _, original := range []Value{Set(), Set(Int(7))} {
		got := appendSetEntry(original, Int(9), setInsertionHash{key: "new", tracked: true})
		if len(got.Set) != len(original.Set)+1 || len(got.setInsertionHashes) != len(got.Set) ||
			got.Ref != original.Ref || got.setInsertionHashAt(len(original.Set)).key != "new" {
			t.Fatal("append lost a value, identity or insertion history")
		}
		if len(original.Set) != 0 && (got.Set[0].Int != 7 || got.setInsertionHashAt(0).tracked) {
			t.Fatal("append changed a legacy element or fabricated its insertion hash")
		}
		if original.setInsertionHashes != nil {
			t.Fatal("append changed the previous untracked history")
		}
	}
}

func BenchmarkSetAppendEntryBuffers(b *testing.B) {
	for _, size := range []int{64, 1024} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			original := Set()
			original.Set = make([]Value, size)
			original.setInsertionHashes = make([]setInsertionHash, size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got := appendSetEntry(original, Int(1), setInsertionHash{key: "1", tracked: true})
				if len(got.Set) != size+1 || got.setInsertionHashAt(size).key != "1" {
					b.Fatal("append lost the new element/history")
				}
			}
		})
	}
}
