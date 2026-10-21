package vm

import (
	"fmt"
	"strings"
	"testing"
)

func TestMapPutParentContainmentReusesUnrelatedScopeProof(t *testing.T) {
	const puts = 12
	for _, width := range []int{128, 512} {
		t.Run(fmt.Sprintf("scope_%d", width), func(t *testing.T) {
			machine, _ := mapPutStaticIndexFixture(t, 8, false, false)
			unrelated := List()
			unrelated.Type = "List<Schema.SObjectField>"
			for i := range width {
				field := Object("Schema.SObjectField")
				field.Fields["name"] = String(fmt.Sprintf("Field_%d__c", i))
				owner := Object("Schema.SObjectType")
				owner.Fields["name"] = String("Catalog__c")
				field.Fields["owner"] = owner
				unrelated.List = append(unrelated.List, field)
			}
			machine.Globals["unrelated"] = unrelated
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			for i := range puts {
				mapPutStaticIndexPut(t, machine, String(fmt.Sprintf("entry_%d", i)), mapPutStaticIndexNode("value"))
			}
			visits := recorder.Snapshot().ScopeAlias.RecursiveVisits
			// Direct record writes with unchanged headers skip the unrelated scope
			// walk. Remaining visits are bounded by the map-parent checks.
			limit := uint64(puts * 16)
			if visits == 0 || visits > limit {
				t.Fatalf("scope recursive visits = %d, want 1..%d for %d puts", visits, limit, puts)
			}
			if got := machine.Globals["unrelated"]; got.Ref != unrelated.Ref || len(got.List) != width {
				t.Fatal("unrelated scope graph changed during map writes")
			}
			assertStaticRefsMatchFresh(t, machine)
		})
	}
}

func TestMapPutParentContainmentPreservesCyclicReachability(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	parent := machine.cloneValuePreserveRefs(factory)
	a, b := List(), List()
	a.Type, b.Type = "List<Object>", "List<Object>"
	a.List, b.List = make([]Value, 12), make([]Value, 12)
	for i := range a.List {
		a.List[i], b.List[i] = Null, Null
	}
	a.List[0], a.List[1], b.List[0] = b, parent, a
	// DFS first reaches B -> A while A is active, then finds A -> parent.
	// A partial miss below B is not a proof that B cannot reach the parent.
	if !machine.valueContainsAliasRefCached(a, snapshotAlias(factory), make(map[uint64]bool)) {
		t.Fatal("cycle fixture must reach the parent through A's later edge")
	}
	machine.Globals["cycleA"], machine.Globals["cycleB"] = a, b
	for _, key := range []string{"first", "second", "third"} {
		item := mapPutStaticIndexNode(key)
		mapPutStaticIndexPut(t, machine, String(key), item)
		fromA := machine.Globals["cycleA"].List[1]
		fromB := machine.Globals["cycleB"].List[0].List[1]
		mapPutParentContainmentAssertEntry(t, "cycle A", fromA, key, item.Ref)
		mapPutParentContainmentAssertEntry(t, "cycle B", fromB, key, item.Ref)
	}
}

func TestMapPutParentContainmentPreservesDivergentBackingReachability(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	empty := Object("IndexedMapNode")
	present := Object("IndexedMapNode")
	present.Ref = empty.Ref
	for i := range 16 {
		name := fmt.Sprintf("Padding_%d", i)
		empty.Fields[name], present.Fields[name] = String("unrelated"), String("unrelated")
	}
	parent := machine.cloneValuePreserveRefs(factory)
	// A detached parent snapshot can retain an older child-map identity. Only
	// parent propagation can refresh it; updating the current map cannot.
	parent.Fields["Items"] = cloneValue(factory.Fields["Items"])
	present.Fields["Child"] = parent
	// Each root gets an independent legacy containment walk. Their shared Ref
	// must not make a negative object-backing proof apply to the other root.
	machine.Globals["emptyBacking"] = empty
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	machine.Globals["presentBacking"] = present
	for _, key := range []string{"first", "second", "third"} {
		item := mapPutStaticIndexNode(key)
		mapPutStaticIndexPut(t, machine, String(key), item)
		parent := machine.Globals["presentBacking"].Fields["Child"]
		mapPutParentContainmentAssertEntry(t, "same Ref with populated backing", parent, key, item.Ref)
	}
}

func TestMapPutParentContainmentInvalidatesAfterOrdinaryWrite(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	holder := Object("IndexedMapNode")
	holder.Fields["Child"] = mapPutParentContainmentLargeMap()
	machine.Globals["holder"] = holder
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	if err := machine.assignPath(holder, []string{"Child"}, machine.cloneValuePreserveRefs(factory)); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"after", "again"} {
		item := mapPutStaticIndexNode(key)
		mapPutStaticIndexPut(t, machine, String(key), item)
		parent := machine.Globals["holder"].Fields["Child"]
		mapPutParentContainmentAssertEntry(t, "ordinary-write parent alias", parent, key, item.Ref)
	}
}

func TestMapPutParentContainmentFullMapReplacementIntroducesParent(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	oldMap := machine.cloneValuePreserveRefs(factory.Fields["Items"])
	// The current map already reaches the parent, while this old detached
	// map snapshot does not. The next inserted value alone is parent-free.
	mapPutStaticIndexPut(t, machine, String("parent"), machine.cloneValuePreserveRefs(factory))
	graph := mapPutParentContainmentLargeMap()
	key := mapKey(String("target"))
	graph.Map[key], graph.MapKeys[key] = oldMap, String("target")
	graph.MapOrder = append(graph.MapOrder, key)
	machine.Globals["lateMapAlias"] = graph
	mapPutStaticIndexPut(t, machine, String("second"), mapPutStaticIndexNode("second"))
	item := mapPutStaticIndexNode("third")
	mapPutStaticIndexPut(t, machine, String("third"), item)
	current := machine.Globals["lateMapAlias"].Map[key]
	parent := current.Map[mapKey(String("parent"))]
	if parent.Ref != factory.Ref {
		t.Fatalf("replacement map parent Ref = %d, want %d", parent.Ref, factory.Ref)
	}
	mapPutParentContainmentAssertEntry(t, "parent introduced by full map replacement", parent, "third", item.Ref)
	mapPutStaticIndexAssertMapEntry(t, "replaced map alias", current, "third", item.Ref)
}

func TestMapPutParentContainmentPreservesAliasesAfterThrowingPropertyAssignment(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	getter, err := CompileAnonymous("return Shared;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous("throw new IllegalArgumentException('setter blocked');")
	if err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["IndexedMapFactory"]
	class.Fields["Shared"] = Field{Name: "Shared", Type: "Map<String,Object>", Access: "public"}
	alias := Field{
		Name: "Alias", Type: "Map<String,Object>", Access: "public", Property: true,
		Getter: &Method{Name: "IndexedMapFactory.Alias.get", ClassName: "IndexedMapFactory", ReturnType: "Map<String,Object>", Program: getter},
		Setter: &Method{Name: "IndexedMapFactory.Alias.set", ClassName: "IndexedMapFactory", Params: []Param{{Name: "value", Type: "Map<String,Object>"}}, Program: setter},
	}
	class.Fields["Alias"] = alias
	machine.Classes["IndexedMapFactory"] = class
	factory.Fields["Shared"] = factory.Fields["Items"]
	// Initialize the property route before the safe put establishes its proof.
	if _, err := machine.callGetter("IndexedMapFactory", alias, factory); err != nil {
		t.Fatal(err)
	}
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	oldMap := machine.cloneValuePreserveRefs(factory.Fields["Items"])

	// A method frame can retain a detached parent Value with the same identity.
	// Its alternate property still returns the original shared map backing.
	detachedThis := machine.cloneValuePreserveRefs(factory)
	detachedThis.Fields["Items"] = factory.Fields["Items"]
	detachedThis.Fields["Shared"] = factory.Fields["Items"]
	mapPutStaticIndexBind(machine, detachedThis)
	receiver, err := machine.callGetter("IndexedMapFactory", alias, detachedThis)
	if err != nil {
		t.Fatal(err)
	}
	if receiver.Ref != factory.Fields["Items"].Ref {
		t.Fatal("alternate getter must return the same map identity")
	}
	_, handled, err := machine.callMapValueMember("this.Alias", receiver, "put", []Value{String("parent"), detachedThis}, resultForLookup())
	if !handled || err != nil {
		t.Fatalf("getter collection mutation: handled=%v err=%v", handled, err)
	}
	// Native getter mutations bypass the setter; an explicit assignment invokes it.
	if err := machine.assignValue("this.Alias", receiver, false); err == nil || !strings.Contains(err.Error(), "setter blocked") {
		t.Fatalf("explicit property assignment = %v, want setter exception", err)
	}
	if got := factory.Fields["Items"].Map[mapKey(String("parent"))]; got.Ref != factory.Ref {
		t.Fatal("failed setter fixture must retain the put's earlier shared-backing mutation")
	}
	mapPutStaticIndexBind(machine, factory)
	graph := mapPutParentContainmentLargeMap()
	key := mapKey(String("target"))
	graph.Map[key], graph.MapKeys[key] = oldMap, String("target")
	graph.MapOrder = append(graph.MapOrder, key)
	machine.Globals["afterFailure"] = graph
	mapPutStaticIndexPut(t, machine, String("after"), mapPutStaticIndexNode("after"))
	last := mapPutStaticIndexNode("last")
	mapPutStaticIndexPut(t, machine, String("last"), last)
	parent := machine.Globals["afterFailure"].Map[key].Map[mapKey(String("parent"))]
	items := parent.Fields["Items"]
	if got := items.Map[mapKey(String("last"))]; got.Ref != last.Ref {
		t.Fatalf("parent after failed setter retained last Ref %d, want %d", got.Ref, last.Ref)
	}
	ordered := false
	for _, encoded := range items.MapOrder {
		ordered = ordered || encoded == mapKey(String("last"))
	}
	if !ordered {
		t.Fatal("parent after failed setter retained a stale map-order header")
	}
}

func TestMapPutParentContainmentDoesNotRetainZeroRefWrapperProof(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	wrapper := Object("IndexedMapNode")
	wrapper.Ref = 0
	wrapper.Fields["Child"] = mapPutParentContainmentLargeMap()
	machine.Globals["wrapper"] = wrapper
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	// A wrapper with no reference identity cannot retain a negative identity
	// certificate. Change its owned graph while retaining that zero Ref.
	wrapper.Fields["Child"] = machine.cloneValuePreserveRefs(factory)
	item := mapPutStaticIndexNode("visible")
	mapPutStaticIndexPut(t, machine, String("visible"), item)
	mapPutParentContainmentAssertEntry(t, "zero Ref wrapper", machine.Globals["wrapper"].Fields["Child"], "visible", item.Ref)
}

func TestMapPutParentContainmentPreservesNestedZeroRefWrappers(t *testing.T) {
	t.Run("scope", func(t *testing.T) {
		machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
		wrapper := Object("IndexedMapNode")
		wrapper.Ref = 0
		wrapper.Fields["Child"] = Null
		graph := List(wrapper)
		graph.Type = "List<Object>"
		for range 16 {
			graph.List = append(graph.List, String("unrelated"))
		}
		machine.Globals["nestedWrapper"] = graph
		mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
		parent := machine.cloneValuePreserveRefs(factory)
		parent.Fields["Items"] = cloneValue(factory.Fields["Items"])
		// The mutable zero-identity backing also disqualifies its enclosing
		// nonzero-identity graph from retaining a negative certificate.
		wrapper.Fields["Child"] = parent
		item := mapPutStaticIndexNode("visible")
		mapPutStaticIndexPut(t, machine, String("visible"), item)
		retained := machine.Globals["nestedWrapper"].List[0].Fields["Child"]
		mapPutParentContainmentAssertEntry(t, "nested zero Ref scope wrapper", retained, "visible", item.Ref)
	})
	t.Run("map", func(t *testing.T) {
		machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
		mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
		oldMap := machine.cloneValuePreserveRefs(factory.Fields["Items"])
		wrapper := Object("IndexedMapNode")
		wrapper.Ref = 0
		wrapper.Fields["Child"] = Null
		mapPutStaticIndexPut(t, machine, String("wrapper"), wrapper)
		parent := machine.cloneValuePreserveRefs(factory)
		parent.Fields["Items"] = cloneValue(factory.Fields["Items"])
		wrapper.Fields["Child"] = parent
		graph := mapPutParentContainmentLargeMap()
		key := mapKey(String("target"))
		graph.Map[key], graph.MapKeys[key] = oldMap, String("target")
		graph.MapOrder = append(graph.MapOrder, key)
		machine.Globals["nestedMapWrapper"] = graph
		// The first replacement exposes the map's newly parent-containing
		// wrapper through the old snapshot. The next put must see that parent.
		mapPutStaticIndexPut(t, machine, String("after"), mapPutStaticIndexNode("after"))
		item := mapPutStaticIndexNode("last")
		mapPutStaticIndexPut(t, machine, String("last"), item)
		current := machine.Globals["nestedMapWrapper"].Map[key]
		retained := current.Map[mapKey(String("wrapper"))].Fields["Child"]
		mapPutParentContainmentAssertEntry(t, "nested zero Ref map wrapper", retained, "last", item.Ref)
		mapPutStaticIndexAssertMapEntry(t, "map with nested zero Ref wrapper", current, "last", item.Ref)
	})
}

func TestMapPutParentContainmentPreservesDetachedMapOrder(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, true)
	holder := Object("IndexedMapNode")
	holder.Fields["Child"] = machine.cloneValuePreserveRefs(factory)
	machine.Globals["detached"] = holder
	for _, key := range []string{"one", "two", "three", "four"} {
		item := mapPutStaticIndexNode(key)
		mapPutStaticIndexPut(t, machine, String(key), item)
		mapPutStaticIndexCheckAliases(t, machine, key, item.Ref)
		mapPutParentContainmentAssertEntry(t, "detached scope factory", machine.Globals["detached"].Fields["Child"], key, item.Ref)
	}
}

func TestMapPutParentContainmentKeepsRuntimeClonesIsolated(t *testing.T) {
	template, _ := mapPutStaticIndexFixture(t, 8, false, false)
	template.Globals["unrelated"] = mapPutParentContainmentLargeMap()
	mapPutStaticIndexPut(t, template, String("seed"), mapPutStaticIndexNode("seed"))
	template.FreezeClassLookup()
	clone, sibling := template.CloneRuntimeFrozenShared(nil), template.CloneRuntimeFrozenShared(nil)
	class, ok := clone.ensureMutableClass("IndexedMapRegistry")
	if !ok {
		t.Fatal("cloned registry missing")
	}
	factory := class.StaticFields["Factory"].Value
	mapPutStaticIndexBind(clone, factory)
	holder := Object("IndexedMapNode")
	holder.Fields["Child"] = clone.cloneValuePreserveRefs(factory)
	clone.Globals["holder"] = holder
	item := mapPutStaticIndexNode("clone only")
	mapPutStaticIndexPut(t, clone, String("clone"), item)
	mapPutParentContainmentAssertEntry(t, "cloned holder", clone.Globals["holder"].Fields["Child"], "clone", item.Ref)
	for label, machine := range map[string]*VM{"template": template, "sibling": sibling} {
		items := machine.Classes["IndexedMapRegistry"].StaticFields["Factory"].Value.Fields["Items"]
		if _, exists := items.Map[mapKey(String("clone"))]; exists || len(items.MapOrder) != 1 {
			t.Fatalf("%s received cloned map insertion", label)
		}
	}
}

func mapPutParentContainmentLargeMap() Value {
	value := Map()
	value.Type = "Map<String,Object>"
	for i := range 16 {
		key := String(fmt.Sprintf("padding_%d", i))
		encoded := mapKey(key)
		value.Map[encoded], value.MapKeys[encoded] = String("unrelated"), key
		value.MapOrder = append(value.MapOrder, encoded)
	}
	return value
}

func mapPutParentContainmentAssertEntry(t *testing.T, label string, factory Value, key string, ref uint64) {
	t.Helper()
	mapPutStaticIndexAssertMapEntry(t, label, factory.Fields["Items"], key, ref)
}

func TestMapPutParentContainmentCompleteCycleProof(t *testing.T) {
	parent := Object("IndexedMapFactory")
	a, b := List(), List()
	a.Type, b.Type = "List<Object>", "List<Object>"
	a.List, b.List = make([]Value, 12), make([]Value, 12)
	a.List[0], a.List[1], b.List[0] = b, parent, a
	cache := &mapPutParentContainmentCache{parentRef: parent.Ref, misses: make(map[mapPutProofBacking]Value)}
	if !cache.contains(a, nil, false) {
		t.Fatal("A must reach the parent after traversing its B cycle")
	}
	// Searching A first must not retain B's partial DFS miss: B reaches the
	// parent through A even though that ancestor was already active in DFS.
	if !cache.contains(b, nil, false) {
		t.Fatal("B must still reach the parent through A after the earlier search")
	}
}
