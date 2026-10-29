package vm

import (
	"fmt"
	"reflect"
	"testing"
)

// These local mutation controls preserve the existing private-copy contract.
// Native read controls: testdata/describe_read_workload/org.tsv:7-10,16-19.
// The mutation observations themselves are pinned by describeShareRows.
func TestGlobalDescribeViewsIsolateMutations(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, mutation := range []string{
				"holder.put('private', Account.SObjectType);",
				"holder.putAll(new Map<String,Schema.SObjectType>{'private' => Account.SObjectType});",
				"holder.remove('account');",
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
	if _, _, err := machine.callValueMember("holder", view, "put", []Value{String("private"), sObjectTypeToken("Account")}, &Result{}); err != nil {
		t.Fatal(err)
	}
	updated := machine.Globals["holder"]
	if sameMapBacking(updated.Map, raw.Map) || updated.Ref != view.Ref {
		t.Fatal("write did not detach the view while preserving its identity")
	}
	if _, ok := raw.Map[mapKey(String("private"))]; ok || !reflect.DeepEqual(order, orderedValueMapKeys(raw)) {
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
			if left.Ref == right.Ref || sameMapBacking(left.Map, right.Map) || !sameMapBacking(left.MapKeys, right.MapKeys) {
				t.Fatal("clone rebuilt immutable global describe keys or shares writable entries")
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
		raw := machine.globalDescribeMapValue()
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
		if _, _, err := machine.callValueMember("", machine.globalDescribeMapValue(), mutation.method, mutation.args, &Result{}); err != nil {
			t.Fatal(err)
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
