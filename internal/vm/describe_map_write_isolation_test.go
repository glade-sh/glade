package vm

import (
	"fmt"
	"testing"
)

// These are local baseline controls, not new native mutation claims. Native
// reads: testdata/describe_read_workload/org.tsv:7-10,16-19. Its ORG-RUN.txt
// explicitly carries local global-map writes outside native conformance.
func TestDescribeMapKeySetWritesKeepPrivateHolders(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			for _, route := range []string{"getAppDescribe('app')", "getModuleDescribe('module')"} {
				for _, mutation := range []string{"remove('Account')", "clear()", "removeAll(new Set<String>{'Account'})", "retainAll(new Set<String>{'Contact'})"} {
					t.Run(fmt.Sprintf("%s/namespace=%v/%s/%s", api, namespaced, route, mutation), func(t *testing.T) {
						machine := describeShareUnprimedFor(namespaced)
						source := `Map<String,Schema.SObjectType> holder = Schema.` + route + `;
Map<String,Schema.SObjectType> sibling = Schema.` + route + `;
Integer sizeBefore = sibling.size();
Set<String> keys = holder.keySet(); keys.` + mutation + `;
System.assert(!holder.containsKey('Account'), 'writer did not change');
System.assertEquals(sizeBefore, sibling.size(), 'sibling size changed');
System.assert(sibling.containsKey('Account'), 'sibling key changed');
System.assert(Schema.` + route + `.containsKey('Account'), 'raw cache changed');`
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func TestSharedMapKeySetMutationDetachesEveryRemoval(t *testing.T) {
	for _, mutation := range []string{"remove('Account')", "clear()", "removeAll(new Set<String>{'Account'})", "retainAll(new Set<String>{'Contact'})"} {
		t.Run(mutation, func(t *testing.T) {
			machine := New(nil)
			original := Map()
			original.Map[mapKey(String("Account"))] = String("one")
			original.Map[mapKey(String("Contact"))] = String("two")
			fillDescribeMapStoredKeys(&original)
			holder := original
			holder.Ref = newValueRef()
			holder.Type = "Map<String,String>"
			holder.mapBackingShared = true
			machine.Globals["holder"] = holder
			machine.VarTypes["holder"] = holder.Type
			describeShareRunApex(t, machine, `Map<String,String> alias = holder;
Set<String> keys = holder.keySet(); Set<String> keysAlias = keys;
keys.`+mutation+`;
System.assert(!holder.containsKey('Account'));
System.assertEquals(holder.size(), alias.size());
System.assertEquals(holder.size(), keys.size());
System.assertEquals(holder.size(), keysAlias.size());`)
			if len(original.Map) != 2 {
				t.Fatal("key-set write changed shared backing")
			}
			if got := machine.Globals["holder"]; got.Ref != holder.Ref || sameMapBacking(got.Map, original.Map) {
				t.Fatal("key-set write lost holder identity or retained shared backing")
			}
		})
	}
}

func TestNamespacedDescribeProjectionWritesStayCallLocal(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mutation := range []string{"put('private', Account.SObjectType)", "putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType})", "remove('account')", "clear()"} {
			t.Run(api+"/"+mutation, func(t *testing.T) {
				machine := describeShareUnprimedFor(true)
				source := `Integer sizeBefore = Schema.getGlobalDescribe().size();
Schema.getGlobalDescribe().` + mutation + `;
System.assert(!Schema.getGlobalDescribe().containsKey('private'), 'write persisted');
System.assertEquals(sizeBefore, Schema.getGlobalDescribe().size(), 'cached projection changed');
System.assertEquals(Account.SObjectType, Schema.getGlobalDescribe().get('account'));`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestNamespacedDescribeRawHolderWritePreservesNextCall(t *testing.T) {
	machine := describeShareUnprimedFor(true)
	first := machine.globalDescribeMapValue()
	sizeBefore := len(first.Map)
	machine.Globals["first"] = first
	if _, _, err := machine.callValueMember("first", first, "clear", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	if len(machine.Globals["first"].Map) != 0 {
		t.Fatal("write did not reach its holder")
	}
	if len(machine.globalDescribeMapValue().Map) != sizeBefore {
		t.Fatal("raw holder write reached a following call")
	}
}

// Immutable preparation must not cache values derived from mutable maps or
// synthesized token payloads. All observations are baseline compatibility pins.
func TestNamespacedDescribeProjectionTracksMutableInputs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, source := range []string{
			`Schema.getGlobalDescribe();
Schema.getAppDescribe('app').put('private', Account.SObjectType);
System.assert(Schema.getGlobalDescribe().containsKey('private'));`,
			`Schema.getGlobalDescribe();
Schema.getModuleDescribe('module').putAll(new Map<String,Schema.SObjectType>{'private'=>Account.SObjectType});
System.assert(Schema.getGlobalDescribe().containsKey('private'));`,
			`Schema.SObjectType token = Schema.getGlobalDescribe().get('zzns__gadget__changeevent');
System.assert(token != null);
token.object = 'Contact';
System.assertEquals('zzns__Gadget__ChangeEvent', String.valueOf(Schema.getGlobalDescribe().get('zzns__gadget__changeevent')));`,
		} {
			t.Run(fmt.Sprintf("%s/%s", api, source), func(t *testing.T) {
				machine := describeShareUnprimedFor(true)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDescribeExpressionKeySetsKeepSnapshotLifetime(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			for _, route := range []string{"getAppDescribe('app')", "getModuleDescribe('module')"} {
				for _, mutation := range []string{"remove('Account')", "clear()", "removeAll(new Set<String>{'Account'})", "retainAll(new Set<String>{'Contact'})"} {
					t.Run(fmt.Sprintf("%s/%v/%s/%s", api, namespaced, route, mutation), func(t *testing.T) {
						machine := describeShareUnprimedFor(namespaced)
						source := `Schema.getGlobalDescribe(); Set<String> keys = Schema.` + route + `.keySet(); System.assert(keys.contains('Account')); keys.` + mutation + `; System.assert(!keys.contains('Account')); System.assert(Schema.` + route + `.containsKey('Account'), 'expression key-set snapshot changed the raw cache');`
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
