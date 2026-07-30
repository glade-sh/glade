package vm

import "testing"

func TestAliasContainmentCacheInvalidatesAncestorNegativeAfterNestedTypedMutation(t *testing.T) {
	machine := New(nil)
	target := Map()
	target.Type = "Map<String,String>"
	beforeKey := mapKey(String("before"))
	target.Map[beforeKey] = String("before")
	target.MapKeys[beforeKey] = String("before")
	target.MapOrder = []string{beforeKey}
	child := testTypedList("List<Object>", String("before"))
	root := testTypedList("List<Object>", child)
	machine.Globals["root"] = root

	if machine.valueContainsAliasRefCached(root, snapshotAlias(target), make(map[uint64]bool)) {
		t.Fatal("root unexpectedly contained target before nested mutation")
	}

	updatedChild := child
	updatedChild.List = append(append([]Value(nil), child.List...), target)
	machine.propagateCollectionMutation(child, updatedChild)
	if !machine.valueContainsAliasRefCached(machine.Globals["root"], snapshotAlias(target), make(map[uint64]bool)) {
		t.Fatal("root did not contain target after nested typed mutation")
	}

	updatedTarget := target
	updatedTarget.Map = cloneValueMapForContainmentTest(target.Map)
	updatedTarget.MapKeys = cloneValueMapForContainmentTest(target.MapKeys)
	afterKey := mapKey(String("after"))
	updatedTarget.Map[afterKey] = String("after")
	updatedTarget.MapKeys[afterKey] = String("after")
	updatedTarget.MapOrder = append(append([]string(nil), target.MapOrder...), afterKey)
	machine.propagateAliasSnapshotToScope(machine.Globals, snapshotAlias(target), updatedTarget)

	gotRoot := machine.Globals["root"]
	if len(gotRoot.List) != 1 || len(gotRoot.List[0].List) != 2 {
		t.Fatalf("root after propagation = %#v", gotRoot)
	}
	got := gotRoot.List[0].List[1]
	if got.Map[afterKey].Text != "after" {
		t.Fatalf("nested target = %#v, want propagated update after cache invalidation", got)
	}
}

func TestAliasContainmentCacheRetainsOneEntryPerStablePairAcrossMutations(t *testing.T) {
	machine := New(nil)
	target := Map()
	target.Type = "Map<String,String>"
	root := testTypedList("List<Object>", String("absent"))
	previous := snapshotAlias(target)
	seen := make(map[uint64]bool)

	for range 20_000 {
		machine.collectionMutationSeq++
		machine.recordCollectionMutation(root.Ref)
		clear(seen)
		if machine.valueContainsAliasRefCached(root, previous, seen) {
			t.Fatal("root unexpectedly contained target")
		}
	}

	if got := len(machine.aliasContainmentCache); got != 1 {
		t.Fatalf("cache entries = %d, want one stable root/target pair", got)
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootsAfterObjectFieldMutation(t *testing.T) {
	machine := New(nil)
	target := testTypedList("List<String>", String("before"))
	holder := Object("Holder")
	listRoot := testTypedList("List<Object>", holder)
	mapRoot := Map()
	mapRoot.Type = "Map<String,Object>"
	rootKey := mapKey(String("holder"))
	mapRoot.Map[rootKey] = holder
	mapRoot.MapKeys[rootKey] = String("holder")
	mapRoot.MapOrder = []string{rootKey}
	machine.Globals["listRoot"] = listRoot
	machine.Globals["mapRoot"] = mapRoot

	previous := snapshotAlias(target)
	for name, root := range machine.Globals {
		if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
			t.Fatalf("%s unexpectedly contained target before object mutation", name)
		}
	}

	machine.setObjectFieldValue(&holder, "Nested", target)
	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("after"))
	machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

	gotList := machine.Globals["listRoot"].List[0].Fields["Nested"]
	if len(gotList.List) != 2 || gotList.List[1].Text != "after" {
		t.Fatalf("List<Object> nested alias = %#v, want propagated update", gotList)
	}
	gotMap := machine.Globals["mapRoot"].Map[rootKey].Fields["Nested"]
	if len(gotMap.List) != 2 || gotMap.List[1].Text != "after" {
		t.Fatalf("Map<String,Object> nested alias = %#v, want propagated update", gotMap)
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootAfterSObjectFieldMutation(t *testing.T) {
	machine := New(nil)
	record := Object("Account")
	target := testTypedList("List<String>", String("before"))
	root := testTypedList("List<SObject>", record)
	machine.Globals["root"] = root
	previous := snapshotAlias(target)
	if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
		t.Fatal("root unexpectedly contained target before SObject mutation")
	}

	machine.setExplicitSObjectFieldValue(&record, "Contact__r", target)
	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("after"))
	machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

	got := machine.Globals["root"].List[0].Fields["Contact__r"]
	if len(got.List) != 2 || got.List[1].Text != "after" {
		t.Fatalf("SObject nested alias = %#v, want propagated update", got)
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootAfterGeneratedPlatformFieldMutation(t *testing.T) {
	machine := New(nil)
	holder := Object("Metadata.CustomMetadataValue")
	target := testTypedList("List<String>", String("before"))
	root := Map()
	root.Type = "Map<String,Object>"
	rootKey := mapKey(String("holder"))
	root.Map[rootKey] = holder
	root.MapKeys[rootKey] = String("holder")
	root.MapOrder = []string{rootKey}
	machine.Globals["root"] = root
	previous := snapshotAlias(target)
	if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
		t.Fatal("root unexpectedly contained target before generated-platform mutation")
	}

	if err := machine.assignPath(holder, []string{"value"}, target); err != nil {
		t.Fatal(err)
	}
	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("after"))
	machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

	got := machine.Globals["root"].Map[rootKey].Fields["value"]
	if len(got.List) != 2 || got.List[1].Text != "after" {
		t.Fatalf("generated-platform nested alias = %#v, want propagated update", got)
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootAfterGeneratedPlatformSetter(t *testing.T) {
	machine := New(nil)
	holder := Object("Metadata.CustomMetadataValue")
	target := testTypedList("List<String>", String("before"))
	root := testTypedList("List<Object>", holder)
	machine.Globals["root"] = root
	previous := snapshotAlias(target)
	if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
		t.Fatal("root unexpectedly contained target before generated setter")
	}

	method := Method{
		Name:       "Metadata.CustomMetadataValue.setValue",
		ClassName:  "Metadata.CustomMetadataValue",
		ReturnType: "void",
		Params:     []Param{{Name: "value", Type: "Object"}},
	}
	machine.generatedPlatformMethodDefaultReturn(method, holder, []Value{target})

	updated := target
	updated.List = append(append([]Value(nil), target.List...), String("after"))
	machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

	got := machine.Globals["root"].List[0].Fields["value"]
	if len(got.List) != 2 || got.List[1].Text != "after" {
		t.Fatalf("generated setter nested alias = %#v, want propagated update", got)
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootAfterPassivePlatformMutation(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		nestedField string
	}{
		{name: "setter", method: "setChild", nestedField: "child"},
		{name: "adder", method: "addChild", nestedField: "childs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machine := New(nil)
			holder := Object("Metadata.CustomMetadataValue")
			target := testTypedList("List<String>", String("before"))
			root := testTypedList("List<Object>", holder)
			machine.Globals["root"] = root
			previous := snapshotAlias(target)
			if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
				t.Fatal("root unexpectedly contained target before passive platform mutation")
			}

			_, _, mutated, handled, err := machine.callPlatformObjectMember(holder, test.method, []Value{target}, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if !handled || !mutated {
				t.Fatalf("%s handled = %t, mutated = %t; want both true", test.method, handled, mutated)
			}

			updated := target
			updated.List = append(append([]Value(nil), target.List...), String("after"))
			machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

			got := machine.Globals["root"].List[0].Fields[test.nestedField]
			if test.method == "addChild" {
				if len(got.List) != 1 {
					t.Fatalf("passive adder field = %#v, want one nested item", got)
				}
				got = got.List[0]
			}
			if len(got.List) != 2 || got.List[1].Text != "after" {
				t.Fatalf("passive %s nested alias = %#v, want propagated update", test.name, got)
			}
		})
	}
}

func TestAliasContainmentCacheInvalidatesAfterSObjectFormulaRefresh(t *testing.T) {
	machine := New(nil)
	record := Object("Account")
	target := testTypedList("List<String>", String("absent"))
	root := testTypedList("List<SObject>", record)
	previous := snapshotAlias(target)
	if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
		t.Fatal("root unexpectedly contained target before formula refresh")
	}
	before := machine.aliasContainmentMutationSeq

	if _, handled, err := machine.callSObjectMember(record, "recalculateFormulas", nil); err != nil {
		t.Fatal(err)
	} else if !handled {
		t.Fatal("SObject.recalculateFormulas was not handled")
	}
	if machine.aliasContainmentMutationSeq == before {
		t.Fatal("SObject.recalculateFormulas did not invalidate alias containment")
	}
}

func TestAliasContainmentCacheInvalidatesTypedRootsAfterUnitOfWorkRegistration(t *testing.T) {
	machine := New(nil)
	unitOfWork, err := machine.constructFrameworkSObjectUnitOfWork(
		[]Value{List(sObjectTypeToken("Account"))},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	record := Object("Account")
	listRoot := testTypedList("List<Object>", unitOfWork)
	mapRoot := typedMap("Map<String,Object>")
	rootKey := mapKey(String("unitOfWork"))
	mapRoot.Map[rootKey] = unitOfWork
	mapRoot.MapKeys[rootKey] = String("unitOfWork")
	mapRoot.MapOrder = []string{rootKey}
	machine.Globals["listRoot"] = listRoot
	machine.Globals["mapRoot"] = mapRoot

	previous := snapshotAlias(record)
	for name, root := range machine.Globals {
		if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
			t.Fatalf("%s unexpectedly contained record before registration", name)
		}
	}
	if _, handled, err := machine.callFrameworkSObjectUnitOfWorkMember(unitOfWork, "registerNew", []Value{record}, &Result{}); err != nil {
		t.Fatal(err)
	} else if !handled {
		t.Fatal("framework_SObjectUnitOfWork.registerNew was not handled")
	}

	updated := record
	updated.Fields = cloneValueMapForContainmentTest(record.Fields)
	updated.Fields["Name"] = String("updated")
	machine.propagateAliasSnapshotToScope(machine.Globals, previous, updated)

	gotList := registeredUnitOfWorkRecord(machine.Globals["listRoot"].List[0], "m_newListByType", "Account")
	if gotList.Fields["Name"].Text != "updated" {
		t.Fatalf("List<Object> registered record = %#v, want propagated update", gotList)
	}
	gotMap := registeredUnitOfWorkRecord(machine.Globals["mapRoot"].Map[rootKey], "m_newListByType", "Account")
	if gotMap.Fields["Name"].Text != "updated" {
		t.Fatalf("Map<String,Object> registered record = %#v, want propagated update", gotMap)
	}
}

func TestAliasContainmentCacheBoundsDistinctPairsWithoutMutation(t *testing.T) {
	const maxEntries = 16_384
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	machine := New(nil)
	root := testTypedList("List<String>", String("absent"))
	seen := make(map[uint64]bool)
	for range 20_000 {
		target := testTypedList("List<String>")
		clear(seen)
		if machine.valueContainsAliasRefCached(root, snapshotAlias(target), seen) {
			t.Fatal("root unexpectedly contained target")
		}
	}

	if got := len(machine.aliasContainmentCache); got > maxEntries {
		t.Fatalf("cache entries = %d, want at most %d", got, maxEntries)
	}
	if got := SnapshotPerfCounters().ScopeAlias.ContainmentEntriesEvicted; got == 0 {
		t.Fatal("cache did not report insertion-time eviction")
	}
}

func BenchmarkAliasContainmentNegativeWithTenPercentMutation(b *testing.B) {
	target := Map()
	target.Type = "Map<String,String>"
	root := testTypedList("List<Object>", String("leaf"))
	for range 64 {
		root = testTypedList("List<Object>", root)
	}
	previous := snapshotAlias(target)

	b.Run("versioned-cache", func(b *testing.B) {
		machine := New(nil)
		seen := make(map[uint64]bool)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%10 == 0 {
				machine.collectionMutationSeq++
				machine.recordCollectionMutation(root.Ref)
			}
			clear(seen)
			if machine.valueContainsAliasRefCached(root, previous, seen) {
				b.Fatal("root unexpectedly contained target")
			}
		}
	})

	b.Run("uncached-reference", func(b *testing.B) {
		seen := make(map[uint64]bool)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			clear(seen)
			if valueContainsAliasRef(root, previous.ref, previous.kind, seen) {
				b.Fatal("root unexpectedly contained target")
			}
		}
	})
}

func registeredUnitOfWorkRecord(unitOfWork Value, fieldName, objectName string) Value {
	bucket := unitOfWork.Fields[fieldName]
	records := bucket.Map[mapKey(String(objectName))]
	if len(records.List) == 0 {
		return Null
	}
	return records.List[0]
}

func testTypedList(typeName string, values ...Value) Value {
	value := List(values...)
	value.Type = typeName
	return value
}

func cloneValueMapForContainmentTest(values map[string]Value) map[string]Value {
	cloned := make(map[string]Value, len(values)+1)
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
