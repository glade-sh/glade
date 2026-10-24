package vm

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestPrimitiveSetAddContainsLinearAllocationRoutes(t *testing.T) {
	for _, element := range []string{"String", "Id"} {
		t.Run(element, func(t *testing.T) {
			const count = 2048
			values := make([]Value, count)
			for i := range values {
				values[i] = String(fmt.Sprintf("key-%06d", i))
				if element == "Id" {
					values[i] = String(fmt.Sprintf("001%012d", i))
					values[i].Type = "Id"
				}
			}
			machine := New(nil)
			set := Set()
			set.Type = "Set<" + element + ">"
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			for _, value := range values {
				hash, err := machine.captureTypedSetInsertionHash(set.Type, value)
				if err != nil {
					t.Fatal(err)
				}
				index, err := machine.setIndexOfValueWithHash(set, value, hash, &Result{})
				if err != nil || index >= 0 {
					t.Fatalf("new add: index=%d error=%v", index, err)
				}
				set = appendSetEntry(set, value, hash)
			}
			for _, value := range values {
				found, err := machine.setContainsValue(set, value, &Result{})
				if !found || err != nil {
					t.Fatalf("contains: found=%v error=%v", found, err)
				}
			}
			runtime.ReadMemStats(&after)
			bytes := after.TotalAlloc - before.TotalAlloc
			t.Logf("%d add + contains: bytes=%d elapsed=%s", count, bytes, time.Since(start))
			if bytes > 16<<20 {
				t.Fatalf("primitive Set add/contains allocated %d bytes; want <=16 MiB", bytes)
			}
			runtime.KeepAlive(set)
		})
	}
}

func TestPrimitiveSetAppendDivergentSnapshotRoute(t *testing.T) {
	set := Set()
	set.Type = "Set<String>"
	set = appendSetEntry(set, String("first"), setInsertionHash{})
	old := set
	current := appendSetEntry(set, String("second"), setInsertionHash{})
	branch := appendSetEntry(old, String("branch"), setInsertionHash{})
	if len(old.Set) != 1 || current.Set[1].Text != "second" || branch.Set[1].Text != "branch" {
		t.Fatal("divergent append overwrote a surviving snapshot")
	}
	machine := New(nil)
	for _, tc := range []struct {
		set  Value
		key  string
		want bool
	}{{current, "second", true}, {current, "branch", false}, {branch, "branch", true}, {old, "second", false}} {
		found, err := machine.setContainsValue(tc.set, String(tc.key), &Result{})
		if found != tc.want || err != nil {
			t.Fatalf("%s: found=%v want=%v error=%v", tc.key, found, tc.want, err)
		}
	}
}

func TestNestedReceiverUnrelatedMutationWarmReturnRoute(t *testing.T) {
	machine := New(nil)
	receiver := Object("WalkReader")
	entries := Map()
	for i := 0; i < 500; i++ {
		child := Object("WalkEntry")
		child.Fields["tags"] = List(String("seed"))
		entries.Map[fmt.Sprint(i)] = child
	}
	receiver.Fields["entries"] = entries
	other := List()
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(other.Ref)
	if !machine.nestedCollectionsUnchangedSince(receiver, 0, nil) {
		t.Fatal("unrelated mutation marked receiver changed")
	}
	var visits uint64
	for i := 0; i < 200; i++ {
		since := machine.collectionMutationSeq
		machine.collectionMutationSeq++
		machine.recordCollectionMutation(other.Ref)
		if !machine.nestedCollectionsUnchangedSince(receiver, since, &visits) {
			t.Fatal("unrelated mutation marked receiver changed")
		}
	}
	t.Logf("200 warm method-return prechecks: visits=%d", visits)
	if visits >= 200*10 {
		t.Fatalf("warm unrelated return visited %d values; want <2000", visits)
	}
	nested := entries.Map["0"].Fields["tags"]
	since := machine.collectionMutationSeq
	machine.collectionMutationSeq++
	machine.recordCollectionMutation(nested.Ref)
	if machine.nestedCollectionsUnchangedSince(receiver, since, nil) {
		t.Fatal("cache hid nested mutation")
	}
}

func BenchmarkPrimitiveSetAddContains(b *testing.B) {
	for _, element := range []string{"String", "Id"} {
		b.Run(element, func(b *testing.B) {
			values := make([]Value, 2048)
			for i := range values {
				values[i] = String(fmt.Sprintf("%015d", i))
				if element == "Id" {
					values[i].Type = "Id"
				}
			}
			machine := New(nil)
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				set := Set()
				set.Type = "Set<" + element + ">"
				for _, value := range values {
					hash, _ := machine.captureTypedSetInsertionHash(set.Type, value)
					index, _ := machine.setIndexOfValueWithHash(set, value, hash, &Result{})
					if index < 0 {
						set = appendSetEntry(set, value, hash)
					}
				}
				for _, value := range values {
					machine.setContainsValue(set, value, &Result{})
				}
			}
		})
	}
}
