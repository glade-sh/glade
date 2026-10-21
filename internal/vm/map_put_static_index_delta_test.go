package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestMapPutStaticIndexDeltaAvoidsRescanningDescriptorGraph(t *testing.T) {
	const puts = 12
	for _, width := range []int{128, 512} {
		t.Run(fmt.Sprintf("descriptor_%d", width), func(t *testing.T) {
			machine, factory := mapPutStaticIndexFixture(t, width, false, false)
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			for i := range puts {
				item := mapPutStaticIndexNode("item")
				item.Fields["Child"] = List(mapPutStaticIndexNode("nested"))
				mapPutStaticIndexPut(t, machine, String(fmt.Sprintf("entry_%d", i)), item)
			}
			work := recorder.Snapshot().StaticAlias.RefScanVisits
			if work == 0 || work > puts*64 {
				t.Fatalf("index reference scans = %d, want positive work bounded by %d for %d small insertions (descriptor width %d)", work, puts*64, puts, width)
			}
			if got := len(factory.Fields["Items"].Map); got != puts {
				t.Fatalf("factory entries = %d, want %d", got, puts)
			}
			assertStaticRefsMatchFresh(t, machine)
		})
	}
}

func TestMapPutStaticIndexDeltaScansKnownItemSubtree(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, false, false)
	item := mapPutStaticIndexNode("known")
	mapPutStaticIndexPut(t, machine, String("first"), item)
	assertStaticRefsMatchFresh(t, machine)
	if !machine.staticValueRefs[item.Ref] {
		t.Fatal("fixture item must already be indexed before it gains a descendant")
	}
	// The same object identity can gain descendants before a subsequent put.
	// The inserted subtree must be scanned even when its root is already known.
	added := List(mapPutStaticIndexNode("new descendant"))
	item.Fields["Child"] = added
	mapPutStaticIndexPut(t, machine, String("second"), item)
	if !machine.staticValueRefs[added.Ref] || !machine.staticValueRefs[added.List[0].Ref] {
		t.Fatal("put of an already indexed item omitted its new descendants")
	}
	assertStaticRefsMatchFresh(t, machine)
	mapPutStaticIndexCheckAliases(t, machine, "second", item.Ref)
}

func TestMapPutStaticIndexDeltaScansInsertedMapBackReference(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, false, false)
	known := mapPutStaticIndexNode("known")
	mapPutStaticIndexPut(t, machine, String("first"), known)
	added := List(mapPutStaticIndexNode("new descendant"))
	known.Fields["Child"] = added
	item := mapPutStaticIndexNode("back reference")
	item.Fields["Child"] = machine.Globals["this"].Fields["Items"]
	mapPutStaticIndexPut(t, machine, String("back"), item)
	for _, ref := range []uint64{added.Ref, added.List[0].Ref} {
		if !machine.staticValueRefs[ref] {
			t.Fatalf("inserted map back-reference omitted newly reachable reference %d", ref)
		}
	}
	assertStaticRefsMatchFresh(t, machine)
	mapPutStaticIndexCheckAliases(t, machine, "back", item.Ref)
}

func TestMapPutStaticIndexDeltaColdIndexPreservesWrites(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, true, true)
	machine.staticValueRefs, machine.staticValueRefFields = nil, nil
	item := mapPutStaticIndexNode("cold")
	mapPutStaticIndexPut(t, machine, String("cold"), item)
	mapPutStaticIndexCheckAliases(t, machine, "cold", item.Ref)
	assertStaticRefsMatchFresh(t, machine)
}

func TestMapPutStaticIndexDeltaBypassesPropertySetter(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, false, false)
	program, err := CompileAnonymous("SetterCalls = SetterCalls + 1; Items = value;")
	if err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["IndexedMapFactory"]
	field := class.Fields["Items"]
	field.Setter = &Method{Name: "IndexedMapFactory.Items.set", ClassName: "IndexedMapFactory", Params: []Param{{Name: "value", Type: "Map<String,Object>"}}, Program: program}
	class.Fields["Items"] = field
	class.Fields["SetterCalls"] = Field{Name: "SetterCalls", Type: "Integer", Access: "public"}
	machine.Classes["IndexedMapFactory"] = class
	factory.Fields["SetterCalls"] = Int(0)
	item := mapPutStaticIndexNode("setter")
	mapPutStaticIndexPut(t, machine, String("setter"), item)
	if got := machine.Globals["this"].Fields["SetterCalls"].Int; got != 0 {
		t.Fatalf("getter collection mutation invoked setter %d times", got)
	}
	mapPutStaticIndexCheckAliases(t, machine, "setter", item.Ref)
	assertStaticRefsMatchFresh(t, machine)
}

func TestMapPutStaticIndexSmallRegistryReusesValidatedPath(t *testing.T) {
	const puts = 8
	machine, _ := mapPutStaticIndexFixture(t, 128, false, false)
	mapPutStaticIndexAddSecondFactory(machine, false)
	class := machine.Classes["IndexedMapRegistry"]
	delete(class.StaticFields, "Factory")
	machine.Classes["IndexedMapRegistry"] = class
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	for i := range puts {
		mapPutStaticIndexPut(t, machine, String(fmt.Sprintf("entry_%d", i)), mapPutStaticIndexNode("value"))
	}
	items := machine.Globals["this"].Fields["Items"]
	location := staticFieldRef{ClassName: "IndexedMapRegistry", FieldName: "Registry"}
	hint := machine.staticAliasChildHints[machine.staticAliasChildHintKey(snapshotAlias(items), location)]
	// The old one-hop hint recursively walks the factory again. This guard
	// requires the validated registry -> factory -> map path to be retained.
	if string(hint.Kind) != "mapValueMapField" {
		t.Fatalf("small registry retained %q hint, want validated map-field path", hint.Kind)
	}
	if hits := recorder.Snapshot().StaticAlias.ChildHintHits; hits < puts-1 {
		t.Fatalf("cached path hits = %d, want at least %d", hits, puts-1)
	}
	retained := machine.Classes["IndexedMapRegistry"].StaticFields["Registry"].Value.Map[mapKey(String("factory"))].Fields["Items"]
	if len(retained.MapOrder) != puts || len(retained.Map) != puts {
		t.Fatalf("small registry retained %d ordered keys and %d entries, want %d", len(retained.MapOrder), len(retained.Map), puts)
	}
	assertStaticRefsMatchFresh(t, machine)
}

func TestMapPutStaticIndexSmallRegistryUpdatesDuplicateMapAliases(t *testing.T) {
	for _, detached := range []bool{false, true} {
		t.Run(fmt.Sprintf("detached_%v", detached), func(t *testing.T) {
			machine, _ := mapPutStaticIndexFixture(t, 8, detached, false)
			mapPutStaticIndexAddSecondFactory(machine, true)
			for _, key := range []string{"first", "second", "third"} {
				item := mapPutStaticIndexNode(key)
				mapPutStaticIndexPut(t, machine, String(key), item)
				mapPutStaticIndexCheckAliases(t, machine, key, item.Ref)
				second := mapPutStaticIndexSecondFactory(machine)
				mapPutStaticIndexAssertMapEntry(t, "second factory", second.Fields["Items"], key, item.Ref)
				assertStaticRefsMatchFresh(t, machine)
			}
		})
	}
}

func TestMapPutStaticIndexSmallRegistryInvalidatesAfterOrdinaryWrite(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, true, false)
	second := mapPutStaticIndexAddSecondFactory(machine, false)
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	alias := machine.cloneValuePreserveRefs(machine.Globals["this"].Fields["Items"])
	if err := machine.assignPath(second, []string{"Published"}, alias); err != nil {
		t.Fatal(err)
	}
	item := mapPutStaticIndexNode("after ordinary write")
	mapPutStaticIndexPut(t, machine, String("after"), item)
	mapPutStaticIndexAssertMapEntry(t, "ordinary-write alias", mapPutStaticIndexSecondFactory(machine).Fields["Published"], "after", item.Ref)
	mapPutStaticIndexCheckAliases(t, machine, "after", item.Ref)
	assertStaticRefsMatchFresh(t, machine)
}

func TestMapPutStaticIndexSmallRegistryInsertedBackReferenceInvalidatesPath(t *testing.T) {
	machine, _ := mapPutStaticIndexFixture(t, 8, true, false)
	second := mapPutStaticIndexAddSecondFactory(machine, false)
	mapPutStaticIndexPut(t, machine, String("warm"), mapPutStaticIndexNode("warm"))
	// The inserted subtree reaches a parent that gained another alias to the
	// target map. Its complete graph invalidates the previously unique path.
	second.Fields["Published"] = machine.cloneValuePreserveRefs(machine.Globals["this"].Fields["Items"])
	item := mapPutStaticIndexNode("back reference")
	item.Fields["Child"] = second
	mapPutStaticIndexPut(t, machine, String("back"), item)
	mapPutStaticIndexAssertMapEntry(t, "inserted parent alias", mapPutStaticIndexSecondFactory(machine).Fields["Published"], "back", item.Ref)
	mapPutStaticIndexCheckAliases(t, machine, "back", item.Ref)
	assertStaticRefsMatchFresh(t, machine)
}

func TestMapPutStaticIndexSmallRegistrySameRefDifferentBackingSiblings(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, true, false)
	second := mapPutStaticIndexAddSecondFactory(machine, false)
	empty := Object("IndexedMapNode")
	divergent := Object("IndexedMapNode")
	divergent.Ref = empty.Ref
	divergent.Fields["Child"] = machine.cloneValuePreserveRefs(factory.Fields["Items"])
	// The uniqueness proof examines the selected parent's fields before the
	// other registry value. An empty backing with this Ref cannot prove that
	// the different backing in the other value lacks the target map.
	for label, parent := range mapPutStaticIndexFactories(machine) {
		if label != "registry static" {
			parent.Fields["Published"] = empty
		}
	}
	registry := machine.Classes["IndexedMapRegistry"].StaticFields["Registry"].Value
	selected := registry.Map[mapKey(String("factory"))]
	selected.Fields["Published"] = empty
	second.Fields["Published"] = divergent
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	for _, key := range []string{"first", "second"} {
		item := mapPutStaticIndexNode(key)
		mapPutStaticIndexPut(t, machine, String(key), item)
		hidden := mapPutStaticIndexSecondFactory(machine).Fields["Published"].Fields["Child"]
		mapPutStaticIndexAssertMapEntry(t, "divergent sibling backing", hidden, key, item.Ref)
		mapPutStaticIndexCheckAliases(t, machine, key, item.Ref)
		// The legacy full index walks by Ref and can omit one divergent
		// backing. The semantic aliases above are the control here.
	}
}

func TestMapPutStaticIndexDeltaRetainsCyclesAndDetachedAliases(t *testing.T) {
	for _, detached := range []bool{false, true} {
		t.Run(fmt.Sprintf("detached_%v", detached), func(t *testing.T) {
			machine, _ := mapPutStaticIndexFixture(t, 8, detached, true)
			item := mapPutStaticIndexNode("before")
			item.Fields["Child"] = item
			mapPutStaticIndexPut(t, machine, String("cycle"), item)
			assertStaticRefsMatchFresh(t, machine)
			mapPutStaticIndexCheckAliases(t, machine, "cycle", item.Ref)
			if err := machine.assignPath(item, []string{"Value"}, String("after")); err != nil {
				t.Fatal(err)
			}
			mapPutStaticIndexCheckAliases(t, machine, "cycle", item.Ref)
			for label, factory := range mapPutStaticIndexFactories(machine) {
				if got := factory.Fields["Items"].Map[mapKey(String("cycle"))].Fields["Value"].Text; got != "after" {
					t.Fatalf("%s retained item Value = %q, want after", label, got)
				}
			}
			assertStaticRefsMatchFresh(t, machine)
		})
	}
}

func TestMapPutStaticIndexDeltaObjectKeyFallbackIndexesSubtree(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, false, false)
	items := factory.Fields["Items"]
	items.Type = "Map<Object,Object>"
	factory.Fields["Items"] = items
	class := machine.Classes["IndexedMapFactory"]
	field := class.Fields["Items"]
	field.Type = items.Type
	class.Fields["Items"] = field
	machine.Classes["IndexedMapFactory"] = class
	key := mapPutStaticIndexNode("key")
	key.Fields["Child"] = List(mapPutStaticIndexNode("key descendant"))
	item := mapPutStaticIndexNode("value")
	item.Fields["Child"] = List(mapPutStaticIndexNode("value descendant"))
	mapPutStaticIndexPut(t, machine, key, item)
	assertStaticRefsMatchFresh(t, machine)
	for _, ref := range []uint64{key.Ref, key.Fields["Child"].Ref, item.Ref, item.Fields["Child"].Ref} {
		if !machine.staticValueRefs[ref] {
			t.Fatalf("fallback omitted key/item subtree reference %d", ref)
		}
	}
}

func TestMapPutStaticIndexDeltaKeepsFrozenClonesIsolated(t *testing.T) {
	template, _ := mapPutStaticIndexFixture(t, 8, false, false)
	template.FreezeClassLookup()
	first, sibling := template.CloneRuntimeFrozenShared(nil), template.CloneRuntimeFrozenShared(nil)
	class, ok := first.ensureMutableClass("IndexedMapRegistry")
	if !ok {
		t.Fatal("cloned registry missing")
	}
	factory := class.StaticFields["Factory"].Value
	mapPutStaticIndexBind(first, factory)
	before := normalizedStaticRefLocations(template.staticValueRefFields)
	item := mapPutStaticIndexNode("clone only")
	mapPutStaticIndexPut(t, first, String("clone"), item)
	assertStaticRefsMatchFresh(t, first)
	assertStaticRefsMatchFresh(t, template)
	assertStaticRefsMatchFresh(t, sibling)
	if !reflect.DeepEqual(before, normalizedStaticRefLocations(template.staticValueRefFields)) {
		t.Fatal("cloned insertion changed the template reference index")
	}
	for label, machine := range map[string]*VM{"template": template, "sibling": sibling} {
		if got := len(machine.Classes["IndexedMapRegistry"].StaticFields["Factory"].Value.Fields["Items"].Map); got != 0 {
			t.Fatalf("%s inherited %d clone entries", label, got)
		}
	}
}

func TestStaticIndexPublishesUnqualifiedFieldSubtree(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, false, true)
	child := List(mapPutStaticIndexNode("first"))
	// Unqualified Apex field assignment uses this shared setter without the
	// qualified assignPath propagation. Its new subtree still needs locations.
	machine.setObjectFieldValue(&factory, "Published", child)
	assertStaticRefsMatchFresh(t, machine)
	if !machine.staticValueRefs[child.Ref] || !machine.staticValueRefs[child.List[0].Ref] {
		t.Fatal("new field subtree was not published to the warmed static index")
	}
	machine.Globals["published"] = child
	if _, handled, err := machine.callListValueMember("published", child, "add", []Value{mapPutStaticIndexNode("second")}, resultForLookup()); err != nil || !handled {
		t.Fatalf("published list mutation: handled=%v err=%v", handled, err)
	}
	for label, retained := range mapPutStaticIndexFactories(machine) {
		if got := len(retained.Fields["Published"].List); got != 2 {
			t.Fatalf("%s retained published list length = %d, want 2", label, got)
		}
	}
	assertStaticRefsMatchFresh(t, machine)
}

func TestStaticIndexPublishesLazyDescribeChildList(t *testing.T) {
	machine, factory := mapPutStaticIndexFixture(t, 8, false, true)
	org := storage.NewOrgState()
	machine.Org = &org
	child := Object("Schema.ChildRelationship")
	child.Fields["relationshipName"] = String("RelatedRows")
	child.Fields["junctionIdListNames"] = List(String("LinkedIds"))
	machine.childRelCache = newChildRelationshipCache()
	machine.childRelCache.store("catalog__c", []Value{child})
	descriptor := factory.Fields["Descriptor"]
	if _, exists := descriptor.Fields["childRelationships"]; exists {
		t.Fatal("fixture must attach its child list lazily")
	}
	children, err := machine.describeChildRelationshipsForDescribe(&descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if len(children.List) != 1 || children.List[0].Ref != child.Ref {
		t.Fatalf("lazy child list = %#v, want the prepared cached relationship", children)
	}
	for _, ref := range []uint64{children.Ref, child.Ref, child.Fields["junctionIdListNames"].Ref} {
		if !machine.staticValueRefs[ref] {
			t.Fatalf("lazy Schema child attachment omitted reference %d", ref)
		}
	}
	assertStaticRefsMatchFresh(t, machine)
}

func mapPutStaticIndexFixture(t *testing.T, width int, detached, cached bool) (*VM, Value) {
	t.Helper()
	machine := New(nil)
	getter := Method{Name: "IndexedMapFactory.Items.get", ClassName: "IndexedMapFactory", ReturnType: "Map<String,Object>"}
	if err := machine.RegisterClass(Class{Name: "IndexedMapFactory", Fields: map[string]Field{
		"Items":      {Name: "Items", Type: "Map<String,Object>", Access: "public", Property: true, HasGetter: true, HasSetter: true, Getter: &getter},
		"Descriptor": {Name: "Descriptor", Type: "Schema.DescribeSObjectResult", Access: "public"},
		"Published":  {Name: "Published", Type: "Object", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "IndexedMapNode", Fields: map[string]Field{
		"Value": {Name: "Value", Type: "String", Access: "public"},
		"Child": {Name: "Child", Type: "Object", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "IndexedMapRegistry", StaticFields: map[string]Field{}}); err != nil {
		t.Fatal(err)
	}
	descriptor := Object("Schema.DescribeSObjectResult")
	descriptor.Fields["name"] = String("Catalog__c")
	fields := Map()
	fields.Type = "Map<String,Schema.SObjectField>"
	shared := Object("Schema.SObjectType")
	shared.Fields["name"] = String("Catalog__c")
	for i := range width {
		name := String(fmt.Sprintf("Entry_%d__c", i))
		field := Object("Schema.SObjectField")
		field.Fields["name"] = name
		field.Fields["owner"] = shared
		field.Fields["referenceTo"] = List(shared)
		encoded := mapKey(name)
		fields.Map[encoded], fields.MapKeys[encoded] = field, name
		fields.MapOrder = append(fields.MapOrder, encoded)
	}
	descriptor.Fields["fields"] = fields
	items := Map()
	items.Type = "Map<String,Object>"
	factory := Object("IndexedMapFactory")
	factory.Fields["Descriptor"], factory.Fields["Items"] = descriptor, items
	registry := Map()
	registry.Type = "Map<String,IndexedMapFactory>"
	registry.Map[mapKey(String("factory"))] = factory
	registry.MapKeys[mapKey(String("factory"))] = String("factory")
	registry.MapOrder = []string{mapKey(String("factory"))}
	copyAlias := func(value Value) Value {
		if detached {
			return machine.cloneValuePreserveRefs(value)
		}
		return value
	}
	class := machine.Classes["IndexedMapRegistry"]
	for name, value := range map[string]Value{"Factory": copyAlias(factory), "Registry": copyAlias(registry)} {
		class.StaticFields[name] = Field{Name: name, Type: value.Type, Static: true, Value: value, InitialValue: value}
	}
	machine.Classes["IndexedMapRegistry"] = class
	machine.Globals["factoryAlias"] = copyAlias(factory)
	if cached {
		machine.cachePut("cache.orgpartition:local.default", "factory", copyAlias(factory), 0)
	}
	mapPutStaticIndexBind(machine, factory)
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	assertStaticRefsMatchFresh(t, machine)
	return machine, factory
}

func mapPutStaticIndexBind(machine *VM, factory Value) {
	machine.Globals["this"] = factory
	machine.currentClass = "IndexedMapFactory"
	machine.currentMethod = *machine.Classes["IndexedMapFactory"].Fields["Items"].Getter
}

func mapPutStaticIndexNode(value string) Value {
	node := Object("IndexedMapNode")
	node.Fields["Value"] = String(value)
	return node
}

func mapPutStaticIndexAddSecondFactory(machine *VM, shareItems bool) Value {
	first := machine.Globals["this"]
	second := Object("IndexedMapFactory")
	second.Fields["Descriptor"] = cloneValue(first.Fields["Descriptor"])
	items := Map()
	items.Type = "Map<String,Object>"
	if shareItems {
		items = machine.cloneValuePreserveRefs(first.Fields["Items"])
	}
	second.Fields["Items"] = items
	registry := machine.Classes["IndexedMapRegistry"].StaticFields["Registry"].Value
	key := mapKey(String("second"))
	registry.Map[key], registry.MapKeys[key] = second, String("second")
	registry.MapOrder = append(registry.MapOrder, key)
	class := machine.Classes["IndexedMapRegistry"]
	field := class.StaticFields["Registry"]
	field.Value = registry
	class.StaticFields["Registry"] = field
	machine.Classes["IndexedMapRegistry"] = class
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	return second
}

func mapPutStaticIndexSecondFactory(machine *VM) Value {
	return machine.Classes["IndexedMapRegistry"].StaticFields["Registry"].Value.Map[mapKey(String("second"))]
}

func mapPutStaticIndexAssertMapEntry(t *testing.T, label string, items Value, key string, ref uint64) {
	t.Helper()
	if got := items.Map[mapKey(String(key))]; got.Ref != ref {
		t.Fatalf("%s item Ref = %d, want %d", label, got.Ref, ref)
	}
	if len(items.MapOrder) != len(items.Map) {
		t.Fatalf("%s map order has %d keys for %d entries", label, len(items.MapOrder), len(items.Map))
	}
}

func mapPutStaticIndexPut(t *testing.T, machine *VM, key, item Value) {
	t.Helper()
	receiver := machine.Globals["this"].Fields["Items"]
	if _, handled, err := machine.callMapValueMember("this.Items", receiver, "put", []Value{key, item}, resultForLookup()); err != nil || !handled {
		t.Fatalf("map put: handled=%v err=%v", handled, err)
	}
}

func mapPutStaticIndexFactories(machine *VM) map[string]Value {
	class := machine.Classes["IndexedMapRegistry"]
	factories := map[string]Value{
		"direct static":   class.StaticFields["Factory"].Value,
		"registry static": class.StaticFields["Registry"].Value.Map[mapKey(String("factory"))],
	}
	if value, ok := machine.Globals["factoryAlias"]; ok {
		factories["global alias"] = value
	}
	if entries := machine.platformCache["cache.orgpartition:local.default"]; entries != nil {
		factories["cache entry"] = entries["factory"].Value
	}
	return factories
}

func mapPutStaticIndexCheckAliases(t *testing.T, machine *VM, key string, ref uint64) {
	t.Helper()
	for label, factory := range mapPutStaticIndexFactories(machine) {
		items := factory.Fields["Items"]
		if got := items.Map[mapKey(String(key))]; got.Ref != ref {
			t.Fatalf("%s item Ref = %d, want %d", label, got.Ref, ref)
		}
		if len(items.MapOrder) != len(items.Map) {
			t.Fatalf("%s map order has %d keys for %d entries", label, len(items.MapOrder), len(items.Map))
		}
	}
}
