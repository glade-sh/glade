package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// These local mutation controls preserve the existing private-copy contract.
// Native read controls: testdata/describe_read_workload/org.tsv:7-10,16-19.
// The mutation observations themselves are pinned by describeShareRows.
func TestGlobalDescribeViewsIsolateMutations(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, mutation := range []string{
				// put, putAll and remove are read-only (review_round11 gw_put,
				// gw_putall, gk_direct); these are the writes that succeed.
				"holder.keySet().remove('account');",
				"holder.keySet().retainAll(new Set<String>{'user'});",
				"holder.clear();",
			} {
				t.Run(fmt.Sprintf("namespace=%v/api=%s/%s", namespaced, api, mutation), func(t *testing.T) {
					for _, rawWriter := range []bool{false, true} {
						machine := describeShareUnprimedFor(namespaced)
						writer := mutation
						if rawWriter {
							writer = "Schema.getGlobalDescribe()" + mutation[len("holder"):]
						}
						source := `Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Integer sizeBefore = sibling.size();
` + writer + `
System.assertEquals(sizeBefore, sibling.size());
System.assert(!sibling.containsKey('private'));
System.assertEquals(Account.SObjectType, sibling.get('account'));
`
						if !rawWriter {
							source += `System.assertEquals(sizeBefore, Schema.getGlobalDescribe().size());
System.assert(!Schema.getGlobalDescribe().containsKey('private'));`
						}
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatalf("raw writer=%v: %v", rawWriter, err)
						}
					}
				})
			}
		}
	}
}

func TestGlobalDescribeViewKeepsCanonicalStorageUntilWrite(t *testing.T) {
	machine := describeShareUnprimed()
	raw := machine.globalDescribeMapValue()
	view, err := machine.coerceAssignable("Map<String,Schema.SObjectType>", raw)
	if err != nil {
		t.Fatal(err)
	}
	if view.Ref == raw.Ref || !sameMapBacking(raw.Map, view.Map) {
		t.Fatal("read-only binding rebuilt the map or lost private identity")
	}
	order := append([]string(nil), orderedValueMapKeys(raw)...)
	machine.Globals["holder"] = view
	machine.VarTypes["holder"] = view.Type
	if _, _, err := machine.callValueMember("holder", view, "clear", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	updated := machine.Globals["holder"]
	if sameMapBacking(updated.Map, raw.Map) || updated.Ref != view.Ref {
		t.Fatal("write did not detach the view while preserving its identity")
	}
	if _, ok := raw.Map[mapKey(String("account"))]; !ok || !reflect.DeepEqual(order, orderedValueMapKeys(raw)) {
		t.Fatal("write changed canonical entries or order")
	}
}

func TestGlobalDescribeClonesShareBaseMap(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			first := describeShareClone(base, template)
			second := describeShareClone(base, template)
			left, right := first.schemaGlobalDescribe(), second.schemaGlobalDescribe()
			// Clones share the entries read-only (global_describe_map_share_test.go);
			// each keeps its own identity and detaches on its first write.
			if left.Ref == right.Ref || !sameMapBacking(left.Map, right.Map) || !sameMapBacking(left.MapKeys, right.MapKeys) {
				t.Fatal("clone rebuilt immutable global describe keys or entries, or shares a map identity")
			}
			projectedLeft, projectedRight := first.globalDescribeMapValue(), second.globalDescribeMapValue()
			if !sameMapBacking(projectedLeft.MapKeys, projectedRight.MapKeys) {
				t.Fatal("clone rebuilt the namespace projection's immutable stored keys")
			}
			// Token properties are writable in the existing local baseline.
			// Reading a token through any collection route must privatize it.
			for _, source := range []string{
				`Schema.SObjectType token = Schema.getGlobalDescribe().get('account'); token.object = 'Contact'; System.assertEquals('Contact', String.valueOf(Schema.getGlobalDescribe().get('account')));`,
				`for (Schema.SObjectType token : Schema.getGlobalDescribe().values()) { token.object = 'Contact'; }`,
				`Map<String,Schema.SObjectType> cloned = Schema.getGlobalDescribe().clone(); Schema.SObjectType token = cloned.get('account'); token.object = 'Contact';`,
			} {
				writer := describeShareClone(base, template)
				describeShareRunApex(t, writer, source)
				reader := describeShareClone(base, template)
				describeShareRunApex(t, reader, `System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));`)
			}
		})
	}
}

func TestGlobalDescribeWrittenEntriesStillValidateOnTypedBinding(t *testing.T) {
	for _, entry := range []struct{ key, value Value }{
		{String("invalid"), String("not a type token")},
		{Int(1), sObjectTypeToken("Account")},
		{Bool(true), sObjectTypeToken("Account")},
	} {
		machine := describeShareUnprimed()
		// The raw app/module cache stays writable; the global view is not.
		raw := machine.schemaGlobalDescribe()
		if _, _, err := machine.callValueMember("", raw, "put", []Value{entry.key, entry.value}, &Result{}); err != nil {
			t.Fatal(err)
		}
		if _, err := machine.coerceAssignable("Map<String,Schema.SObjectType>", machine.globalDescribeMapValue()); err == nil {
			t.Fatalf("typed binding skipped validation after raw write: key=%v value=%v", entry.key, entry.value)
		}
	}
}

func TestGlobalDescribeWritesPreserveRawAppAndModuleViews(t *testing.T) {
	for _, mutation := range []struct {
		method string
		args   []Value
	}{
		{"put", []Value{String("private"), sObjectTypeToken("Account")}},
		{"putAll", []Value{Map()}},
		{"remove", []Value{String("missing")}},
	} {
		machine := describeShareUnprimed()
		// NATIVE review_round11 gw_put, gw_putall, gw_remove_missing: the
		// global view refuses the write and leaves the raw cache alone.
		if _, _, err := machine.callValueMember("", machine.globalDescribeMapValue(), mutation.method, mutation.args, &Result{}); err == nil || err.Error() != readOnlyCollectionFault {
			t.Fatalf("%s: err=%v, want the read-only fault", mutation.method, err)
		}
		if isGlobalDescribeMap(machine.schemaGlobalDescribe()) {
			t.Fatalf("%s published the global-only marker into the raw cache", mutation.method)
		}
		describeShareRunApex(t, machine, `
System.assert(Schema.getAppDescribe('app').keySet().contains('Widget__c'));
System.assert(Schema.getModuleDescribe('module').keySet().contains('Widget__c'));
`)
	}
}

// keySet on the global describe map returns a fresh Set whose contents match
// the lower-cased unique keys; mutating a copy of it must not touch the map.
// NATIVE review_round10 g_gd_add: add on the key set itself raises the
// uncatchable FinalException, and g_gd_clone_add: a clone is writable. remove
// and clear on the key set itself write through to that map and not to a fresh
// getGlobalDescribe() (g_gd_remove N to N-1, g_gd_clear 0|0|N; APIs 62.0, 67.0).
func TestGlobalDescribeKeySetIsFreshAndComplete(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprintf("namespace=%v", namespaced), func(t *testing.T) {
			machine := describeShareUnprimedFor(namespaced)
			program, err := CompileAnonymousWithOptions(`Map<String,Schema.SObjectType> g = Schema.getGlobalDescribe();
Set<String> keys = g.keySet().clone();
System.assert(keys.contains('account'));
System.assert(!keys.contains('Account'));
Integer n = keys.size();
Integer mapSize = g.size();
keys.remove('account');
keys.add('private');
Set<String> again = g.keySet();
System.assertEquals(n, again.size());
System.assert(again.contains('account'));
System.assert(!again.contains('private'));
System.assertEquals(mapSize, g.size());
for (String k : again) {
    System.assertEquals(k, k.toLowerCase());
}
Set<String> live = g.keySet();
live.remove('account');
System.assertEquals(n - 1, live.size());
System.assert(!live.contains('account'));
System.assertEquals(mapSize - 1, g.size());
System.assert(!g.containsKey('account'));
System.assertEquals(null, g.get('account'));
System.assertEquals(mapSize, Schema.getGlobalDescribe().size());
System.assert(Schema.getGlobalDescribe().containsKey('account'));
live.clear();
System.assertEquals(0, live.size());
System.assertEquals(0, g.size());
System.assertEquals(mapSize, Schema.getGlobalDescribe().size());
`, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// NATIVE review_round11 dc_typed_self, dc_chained_self, dc_int, dc_object,
// dc_list_value, dc_nested_map, dc_nested_sobject_list_map and dc_fieldmap (62.0, 67.0): Map.deepClone is refused unless the map's values
// are SObjects; dc_sobject and dc_account run.
func TestMapDeepCloneNeedsSObjectValues(t *testing.T) {
	for _, row := range []struct{ source, err string }{
		{"Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Map<String,Schema.SObjectType> d = gd.deepClone();", "Operation only applies to SObject value map types: Map<String,Schema.SObjectType>"},
		{"Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();", "Operation only applies to SObject value map types: Map<String,Schema.SObjectType>"},
		{"Map<String,Integer> m = new Map<String,Integer>{'a' => 1}; Map<String,Integer> d = m.deepClone();", "Operation only applies to SObject value map types: Map<String,Integer>"},
		{"Map<String,Object> m = new Map<String,Object>{'a' => 1}; Map<String,Object> d = m.deepClone();", "Operation only applies to SObject value map types: Map<String,Object>"},
		{"Map<String,Map<String,Account>> m = new Map<String,Map<String,Account>>{'a' => new Map<String,Account>()}; Map<String,Map<String,Account>> d = m.deepClone();", "Operation only applies to SObject value map types: Map<String,Map<String,Account>>"},
		{"Map<String,Map<Id,SObject>> m = new Map<String,Map<Id,SObject>>(); Map<String,Map<Id,SObject>> d = m.deepClone();", "Operation only applies to SObject value map types: Map<String,Map<Id,SObject>>"},
		{"Map<String,Schema.SObjectField> m = Schema.SObjectType.Account.fields.getMap(); Map<String,Schema.SObjectField> d = m.deepClone();", "Operation only applies to SObject value map types: Map<String,Schema.SObjectField>"},
		{"Map<String,SObject> m = new Map<String,SObject>{'a' => new Account(Name = 'x')}; Map<String,SObject> d = m.deepClone(); System.assertEquals('x', d.get('a').get('Name'));", ""},
		{"Map<Id,Account> m = new Map<Id,Account>(); Map<Id,Account> d = m.deepClone(); System.assertEquals(0, d.size());", ""},
	} {
		for _, namespaced := range []bool{false, true} {
			machine := describeShareUnprimedFor(namespaced)
			program, err := CompileAnonymousWithOptions(row.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			switch {
			case row.err == "" && err != nil:
				t.Fatalf("%s: %v", row.source, err)
			case row.err != "" && (err == nil || !strings.Contains(err.Error(), row.err)):
				t.Fatalf("%s: err=%v, want %q", row.source, err, row.err)
			}
		}
	}
}
