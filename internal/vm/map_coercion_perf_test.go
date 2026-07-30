package vm

import (
	"reflect"
	"testing"
)

func TestMapCoercionPreservesBackingWhenOnlyDeclaredTypeChanges(t *testing.T) {
	machine := New(nil)
	value := Map()
	value.Type = "Map<Object,Object>"
	key := mapKey(String("name"))
	value.Map[key] = String("value")
	value.MapKeys[key] = String("name")
	value.MapOrder = []string{key}

	coerced, err := machine.coerceAssignable("Map<String,String>", value)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "Map<String,String>" {
		t.Fatalf("coerced type = %q, want Map<String,String>", coerced.Type)
	}
	if !sameMapBacking(coerced.Map, value.Map) ||
		!sameMapBacking(coerced.MapKeys, value.MapKeys) ||
		!sameSliceBacking(coerced.MapOrder, value.MapOrder) {
		t.Fatalf("header-only coercion replaced map backing: before=%#v after=%#v", value, coerced)
	}
}

func TestMapCoercionPreservesExactConcreteSObjectRepresentation(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	value := Map()
	value.Type = "Map<String,Account>"
	key := mapKey(String("account"))
	account := Object("Account")
	account.Static = "SObject"
	account.Runtime = "Account"
	value.Map[key] = account
	value.MapKeys[key] = String("account")
	value.MapOrder = []string{key}

	coerced, err := machine.coerceAssignable("Map<String,SObject>", value)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != value.Type {
		t.Fatalf("concrete sObject map type = %q, want %q", coerced.Type, value.Type)
	}
	if !sameMapBacking(coerced.Map, value.Map) ||
		!sameMapBacking(coerced.MapKeys, value.MapKeys) ||
		!sameSliceBacking(coerced.MapOrder, value.MapOrder) {
		t.Fatalf("exact coercion replaced map backing: before=%#v after=%#v", value, coerced)
	}
}

func TestMapCoercionFailureLeavesTopLevelRepresentationUnchanged(t *testing.T) {
	machine := New(nil)
	value := Map()
	value.Type = "Map<String,String>"
	validKey := mapKey(String("001000000000001"))
	invalidKey := mapKey(String("not-an-id"))
	value.Map[validKey] = String("first")
	value.Map[invalidKey] = String("second")
	value.MapKeys[validKey] = String("001000000000001")
	value.MapKeys[invalidKey] = String("not-an-id")
	value.MapOrder = []string{validKey, invalidKey}
	before := cloneValuePreserveRefs(value)

	if _, err := machine.coerceAssignable("Map<Id,String>", value); err == nil {
		t.Fatal("map coercion accepted an invalid Id key")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("failed map coercion changed top-level representation:\nbefore=%#v\nafter=%#v", before, value)
	}
}

func TestMapCoercionPublishesEntryChangesThroughOriginalRepresentation(t *testing.T) {
	machine := New(nil)
	value := Map()
	value.Type = "Map<String,Object>"
	firstKey := mapKey(String("first"))
	secondKey := mapKey(String("second"))
	value.Map[firstKey] = Decimal(1)
	value.Map[secondKey] = Decimal(2)
	value.MapKeys[firstKey] = String("first")
	value.MapKeys[secondKey] = String("second")
	value.MapOrder = []string{firstKey, secondKey}
	beforeKeys := value.MapKeys
	beforeOrder := value.MapOrder

	coerced, err := machine.coerceAssignable("Map<String,Integer>", value)
	if err != nil {
		t.Fatal(err)
	}
	if got := coerced.Map[firstKey]; got.Kind != ValueInt || got.Int != 1 {
		t.Fatalf("first coerced value = %#v, want Integer 1", got)
	}
	if got := coerced.Map[secondKey]; got.Kind != ValueInt || got.Int != 2 {
		t.Fatalf("second coerced value = %#v, want Integer 2", got)
	}
	if reflect.DeepEqual(coerced.MapKeys, nil) ||
		!sameMapBacking(coerced.MapKeys, beforeKeys) ||
		!sameSliceBacking(coerced.MapOrder, beforeOrder) {
		t.Fatalf("changed coercion split original key/order representation: %#v", coerced)
	}
}

func TestMapCoercionRepairsNonCanonicalStoredKeyRepresentation(t *testing.T) {
	machine := New(nil)
	value := Map()
	value.Type = "Map<Object,Object>"
	rawKey := mapKey(String("actual"))
	value.Map[rawKey] = String("value")
	value.MapKeys = map[string]Value{mapKey(String("stale")): String("stale")}
	value.MapOrder = []string{rawKey}

	coerced, err := machine.coerceAssignable("Map<String,String>", value)
	if err != nil {
		t.Fatal(err)
	}
	if len(coerced.MapKeys) != 1 {
		t.Fatalf("stored keys = %#v, want one canonical key", coerced.MapKeys)
	}
	if got, ok := coerced.MapKeys[rawKey]; !ok || got.Kind != ValueString || got.Text != "actual" {
		t.Fatalf("canonical stored key = (%#v, %v), want actual", got, ok)
	}
	if coerced.Ref == value.Ref {
		t.Fatal("noncanonical map repair retained an alias ref whose metadata could not be updated coherently")
	}
}

func TestMapCoercionNestedFailureLeavesEarlierCollectionUnchanged(t *testing.T) {
	machine := New(nil)
	first := List(Decimal(1))
	first.Type = "List<Decimal>"
	failing := List(Decimal(1.5))
	failing.Type = "List<Decimal>"
	value := Map()
	value.Type = "Map<String,List<Decimal>>"
	firstKey := mapKey(String("first"))
	failingKey := mapKey(String("failing"))
	value.Map[firstKey] = first
	value.Map[failingKey] = failing
	value.MapKeys[firstKey] = String("first")
	value.MapKeys[failingKey] = String("failing")
	value.MapOrder = []string{firstKey, failingKey}
	before := cloneValuePreserveRefs(value)

	if _, err := machine.coerceAssignable("Map<String,List<Integer>>", value); err == nil {
		t.Fatal("nested map coercion accepted a non-integral Decimal")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("failed nested map coercion changed an earlier collection:\nbefore=%#v\nafter=%#v", before, value)
	}
}

func TestMapCoercionNestedSuccessPreservesAliasIdentity(t *testing.T) {
	machine := New(nil)
	shared := List(Decimal(1))
	shared.Type = "List<Decimal>"
	value := Map()
	value.Type = "Map<String,List<Decimal>>"
	key := mapKey(String("shared"))
	value.Map[key] = shared
	value.MapKeys[key] = String("shared")
	value.MapOrder = []string{key}

	coerced, err := machine.coerceAssignable("Map<String,List<Integer>>", value)
	if err != nil {
		t.Fatal(err)
	}
	got := coerced.Map[key]
	if got.Ref != shared.Ref || !sameSliceBacking(got.List, shared.List) {
		t.Fatalf("successful nested coercion changed alias identity: shared=%#v coerced=%#v", shared, got)
	}
	if shared.List[0].Kind != ValueInt || shared.List[0].Int != 1 {
		t.Fatalf("successful nested coercion did not update shared backing: %#v", shared)
	}
}

func TestMapCoercionNestedSuccessKeepsSiblingMapAliasCoherent(t *testing.T) {
	machine := New(nil)
	nested := Map()
	nested.Type = "Map<Object,Object>"
	oldKeyValue := Decimal(1)
	oldKey := mapKey(oldKeyValue)
	nested.Map[oldKey] = Decimal(2)
	nested.MapKeys[oldKey] = oldKeyValue
	nested.MapOrder = []string{oldKey}
	sibling := nested

	value := Map()
	value.Type = "Map<String,Map<Object,Object>>"
	outerKey := mapKey(String("nested"))
	value.Map[outerKey] = nested
	value.MapKeys[outerKey] = String("nested")
	value.MapOrder = []string{outerKey}

	coerced, err := machine.coerceAssignable("Map<String,Map<Integer,Integer>>", value)
	if err != nil {
		t.Fatal(err)
	}
	got := coerced.Map[outerKey]
	newKey := mapKey(Int(1))
	if got.Ref != sibling.Ref {
		t.Fatalf("nested map ref = %d, want sibling ref %d", got.Ref, sibling.Ref)
	}
	if !sameMapBacking(got.Map, sibling.Map) ||
		!sameMapBacking(got.MapKeys, sibling.MapKeys) ||
		!sameSliceBacking(got.MapOrder, sibling.MapOrder) {
		t.Fatalf("successful coercion split sibling representation:\ngot=%#v\nsibling=%#v", got, sibling)
	}
	if len(sibling.MapOrder) != 1 || sibling.MapOrder[0] != newKey {
		t.Fatalf("sibling order = %#v, want [%q]", sibling.MapOrder, newKey)
	}
	if key := sibling.MapKeys[newKey]; key.Kind != ValueInt || key.Int != 1 {
		t.Fatalf("sibling stored key = %#v, want Integer 1", key)
	}
	if item := sibling.Map[newKey]; item.Kind != ValueInt || item.Int != 2 {
		t.Fatalf("sibling lookup = %#v, want Integer 2", item)
	}
	if lookupKey := machine.mapLookupKey(sibling, Int(1)); lookupKey != newKey {
		t.Fatalf("sibling runtime lookup key = %q, want %q", lookupKey, newKey)
	}
	if _, ok := sibling.Map[oldKey]; ok {
		t.Fatalf("sibling retained old key %q: %#v", oldKey, sibling)
	}
}

func TestMapCoercionDetachesFromSiblingWithNilStoredKeys(t *testing.T) {
	machine := New(nil)
	nested := Map()
	nested.Type = "Map<String,Object>"
	key := mapKey(String("value"))
	nested.Map[key] = Decimal(2)
	nested.MapKeys = nil
	nested.MapOrder = nil
	sibling := nested

	coerced, err := machine.coerceAssignable("Map<String,Integer>", nested)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Ref == sibling.Ref {
		t.Fatal("coercion retained sibling ref with nil alias metadata")
	}
	if sibling.MapKeys != nil || sibling.MapOrder != nil {
		t.Fatalf("sibling metadata changed: %#v", sibling)
	}
	if item := sibling.Map[key]; item.Kind != ValueDecimal || item.Decimal != 2 {
		t.Fatalf("sibling value changed: %#v", item)
	}
	if stored := coerced.MapKeys[key]; stored.Kind != ValueString || stored.Text != "value" {
		t.Fatalf("coerced stored key = %#v, want String value", stored)
	}
	if item := coerced.Map[key]; item.Kind != ValueInt || item.Int != 2 {
		t.Fatalf("coerced value = %#v, want Integer 2", item)
	}
}

func TestMapCoercionDetachesFromSiblingWithIncompleteOrder(t *testing.T) {
	machine := New(nil)
	nested := Map()
	nested.Type = "Map<String,Object>"
	key := mapKey(String("value"))
	nested.Map[key] = Decimal(2)
	nested.MapKeys[key] = String("value")
	nested.MapOrder = []string{}
	sibling := nested

	coerced, err := machine.coerceAssignable("Map<String,Integer>", nested)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Ref == sibling.Ref {
		t.Fatal("coercion retained sibling ref with incomplete alias order")
	}
	if len(sibling.MapOrder) != 0 || sibling.Map[key].Kind != ValueDecimal {
		t.Fatalf("sibling representation changed: %#v", sibling)
	}
	if len(coerced.MapOrder) != 1 || coerced.MapOrder[0] != key {
		t.Fatalf("coerced order = %#v, want [%q]", coerced.MapOrder, key)
	}
	if item := coerced.Map[key]; item.Kind != ValueInt || item.Int != 2 {
		t.Fatalf("coerced value = %#v, want Integer 2", item)
	}
}

func TestMapCoercionKeepsCanonicalEmptyMapAlias(t *testing.T) {
	machine := New(nil)
	value := typedMap("Map<String,String>")
	sibling := value

	coerced, err := machine.coerceAssignable("Map<String,Object>", value)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Ref != sibling.Ref {
		t.Fatalf("empty canonical map ref = %d, want sibling ref %d", coerced.Ref, sibling.Ref)
	}
	if !sameMapBacking(coerced.Map, sibling.Map) || !sameMapBacking(coerced.MapKeys, sibling.MapKeys) {
		t.Fatalf("empty canonical map detached backing:\ncoerced=%#v\nsibling=%#v", coerced, sibling)
	}
}

func TestMapCoercionNestedMapFailureRestoresEarlierBacking(t *testing.T) {
	machine := New(nil)
	first := Map()
	first.Type = "Map<String,Object>"
	firstValueKey := mapKey(String("value"))
	first.Map[firstValueKey] = Decimal(1)
	first.MapKeys[firstValueKey] = String("value")
	first.MapOrder = []string{firstValueKey}
	failing := Map()
	failing.Type = "Map<String,Object>"
	failingValueKey := mapKey(String("value"))
	failing.Map[failingValueKey] = Decimal(1.5)
	failing.MapKeys[failingValueKey] = String("value")
	failing.MapOrder = []string{failingValueKey}

	value := Map()
	value.Type = "Map<String,Map<String,Object>>"
	firstKey := mapKey(String("first"))
	failingKey := mapKey(String("failing"))
	value.Map[firstKey] = first
	value.Map[failingKey] = failing
	value.MapKeys[firstKey] = String("first")
	value.MapKeys[failingKey] = String("failing")
	value.MapOrder = []string{firstKey, failingKey}
	before := cloneValuePreserveRefs(value)

	if _, err := machine.coerceAssignable("Map<String,Map<String,Integer>>", value); err == nil {
		t.Fatal("nested map coercion accepted a non-integral Decimal")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("failed nested map coercion changed an earlier map:\nbefore=%#v\nafter=%#v", before, value)
	}
	if !sameMapBacking(value.Map[firstKey].Map, first.Map) {
		t.Fatal("rollback replaced the earlier nested map backing")
	}
}

func TestMapCoercionNestedFailureRestoresEarlierMapOrder(t *testing.T) {
	machine := New(nil)
	first := Map()
	first.Type = "Map<Object,Object>"
	staleKey := mapKey(String("stale"))
	canonicalKey := mapKey(String("canonical"))
	first.Map[staleKey] = Decimal(1)
	first.MapKeys[staleKey] = String("canonical")
	first.MapOrder = []string{staleKey}
	failing := Map()
	failing.Type = "Map<Object,Object>"
	failingKey := mapKey(String("value"))
	failing.Map[failingKey] = Decimal(1.5)
	failing.MapKeys[failingKey] = String("value")
	failing.MapOrder = []string{failingKey}

	value := Map()
	value.Type = "Map<String,Map<Object,Object>>"
	firstKey := mapKey(String("first"))
	secondKey := mapKey(String("failing"))
	value.Map[firstKey] = first
	value.Map[secondKey] = failing
	value.MapKeys[firstKey] = String("first")
	value.MapKeys[secondKey] = String("failing")
	value.MapOrder = []string{firstKey, secondKey}
	before := cloneValuePreserveRefs(value)

	if _, err := machine.coerceAssignable("Map<String,Map<String,Integer>>", value); err == nil {
		t.Fatal("nested map coercion accepted a non-integral Decimal")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("failed nested map coercion changed map ordering metadata:\nbefore=%#v\nafter=%#v", before, value)
	}
	if got := value.Map[firstKey].MapOrder; len(got) != 1 || got[0] != staleKey {
		t.Fatalf("rollback order = %#v, want [%q]; converted key was %q", got, staleKey, canonicalKey)
	}
}

func TestCoercionGraphRollbackRestoresNestedObjectBranches(t *testing.T) {
	values := List(String("before"))
	nested := Map()
	nested.Type = "Map<String,String>"
	nestedKey := mapKey(String("state"))
	nested.Map[nestedKey] = String("before")
	nested.MapKeys[nestedKey] = String("state")
	nested.MapOrder = []string{nestedKey}
	envelope := Object("Envelope")
	envelope.Fields["values"] = values
	envelope.Fields["nested"] = nested
	root := Map()
	root.Type = "Map<String,Object>"
	rootKey := mapKey(String("root"))
	root.Map[rootKey] = envelope
	root.MapKeys[rootKey] = String("root")
	root.MapOrder = []string{rootKey}
	before := cloneValuePreserveRefs(root)

	rollback := captureCoercionMapBranches(root, root.MapOrder)
	values.List[0] = String("after")
	envelope.Fields["added"] = String("after")
	nested.Map[nestedKey] = String("after")
	rollback.restore()

	if !reflect.DeepEqual(root, before) {
		t.Fatalf("rollback did not restore nested object graph:\nbefore=%#v\nafter=%#v", before, root)
	}
	if !sameSliceBacking(root.Map[rootKey].Fields["values"].List, values.List) ||
		!sameMapBacking(root.Map[rootKey].Fields["nested"].Map, nested.Map) {
		t.Fatal("rollback replaced nested collection identity")
	}
}
