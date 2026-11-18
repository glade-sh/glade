package vm

import (
	"fmt"
	"maps"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

// staticRefIndexBase builds a frozen index of refs 1..refCount spread over
// locCount locations. Every third ref sits in several locations, so the
// location sets exercise both the inline entry and the overflow map.
func staticRefIndexBase(refCount, locCount int) (map[uint64]bool, map[uint64]staticFieldRefSet, []staticFieldRef) {
	locations := make([]staticFieldRef, locCount)
	for i := range locations {
		locations[i] = staticFieldRef{ClassName: fmt.Sprintf("C%d", i), FieldName: "F"}
	}
	refs := make(map[uint64]bool, refCount)
	fields := make(map[uint64]staticFieldRefSet, refCount)
	for ref := uint64(1); ref <= uint64(refCount); ref++ {
		set := staticFieldRefSet{}
		set.add(locations[int(ref)%locCount])
		if ref%3 == 0 {
			set.add(locations[int(ref+1)%locCount])
			set.add(locations[int(ref+2)%locCount])
		}
		refs[ref] = true
		fields[ref] = set
	}
	return refs, fields, locations
}

func cloneStaticRefIndex(refs map[uint64]bool, fields map[uint64]staticFieldRefSet) (map[uint64]bool, map[uint64]staticFieldRefSet) {
	outFields := make(map[uint64]staticFieldRefSet, len(fields))
	for ref, set := range fields {
		set.many = maps.Clone(set.many)
		outFields[ref] = set
	}
	return maps.Clone(refs), outFields
}

// sharedStaticRefVM returns a VM that holds the index as a shared clone does.
func sharedStaticRefVM(refs map[uint64]bool, fields map[uint64]staticFieldRefSet) *VM {
	machine := New(nil)
	machine.staticValueRefs = refs
	machine.staticValueRefFields = fields
	machine.staticValueRefsShared = true
	return machine
}

// plainStaticRefVM returns a VM that owns a private full copy: the behavior
// every shared VM had before the index was layered.
func plainStaticRefVM(refs map[uint64]bool, fields map[uint64]staticFieldRefSet) *VM {
	machine := New(nil)
	machine.staticValueRefs, machine.staticValueRefFields = cloneStaticRefIndex(refs, fields)
	return machine
}

func staticRefState(machine *VM) (map[uint64]bool, map[uint64]map[staticFieldRef]bool) {
	refs, fields := machine.staticRefIndexSnapshot()
	return maps.Clone(refs), normalizedStaticRefLocations(fields)
}

func assertStaticRefStatesEqual(t *testing.T, label string, got, want *VM) {
	t.Helper()
	gotRefs, gotFields := staticRefState(got)
	wantRefs, wantFields := staticRefState(want)
	if !reflect.DeepEqual(gotRefs, wantRefs) {
		t.Fatalf("%s: refs differ: got %d entries, want %d", label, len(gotRefs), len(wantRefs))
	}
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("%s: locations differ", label)
	}
}

func TestStaticRefOwnershipAllocatesInProportionToChanges(t *testing.T) {
	refs, fields, locations := staticRefIndexBase(6000, 50)
	machine := sharedStaticRefVM(refs, fields)
	replacement := Object("Account")
	replacement.Ref = 1_000_001
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	machine.forgetStaticValueRefInField(3, locations[3])
	machine.collectStaticFieldValueRefsInField(replacement, locations[7])
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	// The base index is several hundred KiB. A write that changes two refs
	// must not pay for a copy of it.
	if allocated > 32<<10 {
		t.Fatalf("one forget and one collect allocated %d bytes on a 6000-ref shared index, want at most 32768", allocated)
	}
	if machine.staticRefIndexKnown(1_000_001) != true || machine.staticRefIndexKnown(3) != true {
		t.Fatal("the writes did not take effect")
	}
}

func (vm *VM) staticRefIndexKnown(ref uint64) bool {
	index := vm.staticRefIndex()
	return index.known(ref)
}

func TestStaticRefOverlayMatchesPlainMapUnderRandomOperations(t *testing.T) {
	const refCount, locCount = 40, 6
	for seed := int64(1); seed <= 60; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			baseRefs, baseFields, locations := staticRefIndexBase(refCount, locCount)
			frozenRefs, frozenFields := cloneStaticRefIndex(baseRefs, baseFields)
			layered := sharedStaticRefVM(baseRefs, baseFields)
			sibling := sharedStaticRefVM(baseRefs, baseFields)
			plain := plainStaticRefVM(baseRefs, baseFields)
			rng := rand.New(rand.NewSource(seed))
			for step := 0; step < 120; step++ {
				location := locations[rng.Intn(locCount)]
				// Refs above refCount are new; deletes and re-adds of one
				// ref across steps cover tombstones.
				ref := uint64(1 + rng.Intn(refCount+10))
				child := Object("Account")
				child.Ref = uint64(1 + rng.Intn(refCount+10))
				value := List(child)
				value.Ref = ref
				var op string
				switch rng.Intn(5) {
				case 0, 1:
					op = "forget"
					layered.forgetStaticValueRefInField(ref, location)
					plain.forgetStaticValueRefInField(ref, location)
				case 2:
					op = "collect"
					layered.collectStaticFieldValueRefsInField(value, location)
					plain.collectStaticFieldValueRefsInField(value, location)
				case 3:
					op = "collect-additional"
					layered.collectAdditionalStaticFieldValueRefsInField(value, location)
					plain.collectAdditionalStaticFieldValueRefsInField(value, location)
				default:
					op = "forget-location"
					layered.forgetStaticValueRefsInField(location)
					plain.forgetStaticValueRefsInField(location)
				}
				assertStaticRefStatesEqual(t, fmt.Sprintf("step %d %s", step, op), layered, plain)
				for probe := uint64(1); probe <= refCount+10; probe++ {
					a, b := layered.staticRefIndex(), plain.staticRefIndex()
					if a.known(probe) != b.known(probe) {
						t.Fatalf("step %d %s: known(%d) differs", step, op, probe)
					}
					if !reflect.DeepEqual(normalizedStaticRefLocations(map[uint64]staticFieldRefSet{probe: a.locations(probe)})[probe], normalizedStaticRefLocations(map[uint64]staticFieldRefSet{probe: b.locations(probe)})[probe]) {
						t.Fatalf("step %d %s: locations(%d) differ", step, op, probe)
					}
				}
			}
			// Neither the shared base nor a sibling saw any of it.
			gotRefs, gotFields := maps.Clone(baseRefs), normalizedStaticRefLocations(baseFields)
			if !reflect.DeepEqual(gotRefs, frozenRefs) || !reflect.DeepEqual(gotFields, normalizedStaticRefLocations(frozenFields)) {
				t.Fatal("writes through one runtime changed the shared base index")
			}
			assertStaticRefStatesEqual(t, "sibling", sibling, plainStaticRefVM(frozenRefs, frozenFields))
		})
	}
}

func TestStaticRefOverlayWriteLeavesSetsSharedWithBaseIntact(t *testing.T) {
	refs, fields, locations := staticRefIndexBase(30, 6)
	// Ref 3 holds three locations: an inline entry plus an overflow map.
	if fields[3].len() != 3 || len(fields[3].many) != 2 {
		t.Fatalf("fixture ref 3 has %d locations, %d in many", fields[3].len(), len(fields[3].many))
	}
	before := normalizedStaticRefLocations(map[uint64]staticFieldRefSet{3: fields[3]})
	machine := sharedStaticRefVM(refs, fields)
	extra := staticFieldRef{ClassName: "Extra", FieldName: "F"}
	machine.collectAdditionalStaticFieldValueRefsInField(func() Value { v := Object("Account"); v.Ref = 3; return v }(), extra)
	index := machine.staticRefIndex()
	if !index.locations(3).contains(extra) {
		t.Fatal("writer lost its own addition")
	}
	if fields[3].contains(extra) || len(fields[3].many) != 2 {
		t.Fatal("adding a location to a shared set changed the base set")
	}
	machine.forgetStaticValueRefInField(3, locations[3])
	if !reflect.DeepEqual(before, normalizedStaticRefLocations(map[uint64]staticFieldRefSet{3: fields[3]})) {
		t.Fatal("removing a location changed the base set")
	}
}

func TestStaticRefOverlayTombstoneThenReAdd(t *testing.T) {
	refs, fields, locations := staticRefIndexBase(10, 4)
	machine := sharedStaticRefVM(refs, fields)
	// Ref 1 lives only at locations[1].
	machine.forgetStaticValueRefInField(1, locations[1])
	index := machine.staticRefIndex()
	if index.known(1) || !index.locations(1).empty() {
		t.Fatal("ref 1 should be gone for the writer")
	}
	if !refs[1] || fields[1].empty() {
		t.Fatal("ref 1 vanished from the base")
	}
	seen := 0
	index.each(func(ref uint64) {
		if ref == 1 {
			t.Fatal("each visited a deleted ref")
		}
		seen++
	})
	if seen != 9 {
		t.Fatalf("each visited %d refs, want 9", seen)
	}
	readded := Object("Account")
	readded.Ref = 1
	machine.collectStaticFieldValueRefsInField(readded, locations[2])
	index = machine.staticRefIndex()
	if !index.known(1) || !index.locations(1).contains(locations[2]) || index.locations(1).contains(locations[1]) {
		t.Fatal("re-added ref has the wrong locations")
	}
}

func TestStaticRefOverlaySiblingsAreIsolatedUnderRace(t *testing.T) {
	refs, fields, locations := staticRefIndexBase(200, 8)
	frozenRefs, frozenFields := cloneStaticRefIndex(refs, fields)
	const workers = 8
	machines := make([]*VM, workers)
	models := make([]*VM, workers)
	for i := range machines {
		machines[i] = sharedStaticRefVM(refs, fields)
		models[i] = plainStaticRefVM(frozenRefs, frozenFields)
	}
	var wg sync.WaitGroup
	for i := range machines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(i) + 100))
			for step := 0; step < 300; step++ {
				location := locations[rng.Intn(len(locations))]
				ref := uint64(1 + rng.Intn(220))
				value := Object("Account")
				value.Ref = ref
				switch rng.Intn(3) {
				case 0:
					machines[i].forgetStaticValueRefInField(ref, location)
					models[i].forgetStaticValueRefInField(ref, location)
				case 1:
					machines[i].collectStaticFieldValueRefsInField(value, location)
					models[i].collectStaticFieldValueRefsInField(value, location)
				default:
					machines[i].collectAdditionalStaticFieldValueRefsInField(value, location)
					models[i].collectAdditionalStaticFieldValueRefsInField(value, location)
				}
			}
		}(i)
	}
	wg.Wait()
	for i := range machines {
		assertStaticRefStatesEqual(t, fmt.Sprintf("worker %d", i), machines[i], models[i])
	}
	if !reflect.DeepEqual(refs, frozenRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(fields), normalizedStaticRefLocations(frozenFields)) {
		t.Fatal("concurrent writers changed the shared base index")
	}
}

func TestStaticRefCloneOfLayeredIndexSeesWriterState(t *testing.T) {
	template, _ := staticRefsCloneFixture(t)
	template.FreezeClassLookup()
	first := template.CloneRuntimeFrozenShared(nil)
	class := classValue(first, "pkg.ListRegistry")
	replacement := List(Object("Replacement"))
	first.writeStaticFieldValue("pkg.ListRegistry", "Items", class, class.StaticFields["Items"], replacement)
	if first.staticRefBaseRefs == nil {
		t.Fatal("the write should have layered the index over the shared base")
	}
	second := first.CloneRuntimeFrozenShared(nil)
	assertStaticRefsMatchFresh(t, second)
	assertStaticRefsMatchFresh(t, first)
	assertStaticRefsMatchFresh(t, template)
	index := second.staticRefIndex()
	if !index.known(replacement.Ref) {
		t.Fatal("clone of a layered VM lost the writer's refs")
	}
}
