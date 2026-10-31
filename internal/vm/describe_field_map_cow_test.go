package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func fieldMapWrapper(t *testing.T, machine *VM, object string) Value {
	t.Helper()
	name, definition, ok := machine.describeObjectDefinition(object)
	if !ok {
		t.Fatalf("missing object %s", object)
	}
	return machine.describeSObjectValue(name, definition).Fields["fields"]
}

func fieldMapGet(t *testing.T, machine *VM, wrapper Value) Value {
	t.Helper()
	value, _, mutated, handled, err := machine.callPlatformObjectMember(wrapper, "getMap", nil, &Result{})
	if err != nil || !handled || mutated || value.Kind != ValueMap {
		t.Fatalf("getMap: value=%s handled=%v mutated=%v error=%v", value.Kind, handled, mutated, err)
	}
	return value
}

// Native read observations: testdata/describe_sharing/org.tsv:2,4,5,7.
// Writes below cover holder isolation and the captured read-only mutations.
func TestFieldMapReadHoldersPreserveOwnership(t *testing.T) {
	for _, primed := range []bool{false, true} {
		for _, namespaced := range []bool{false, true} {
			t.Run(fmt.Sprintf("primed=%v/namespace=%v", primed, namespaced), func(t *testing.T) {
				machine := describeShareUnprimedFor(namespaced)
				if primed {
					base, template := describeSharePrimedBaseFor(t, namespaced)
					machine = describeShareClone(base, template)
				}
				wrapper := fieldMapWrapper(t, machine, "Account")
				first := fieldMapGet(t, machine, wrapper)
				second := fieldMapGet(t, machine, wrapper)
				source := wrapper.Fields["map"]
				if first.Ref == second.Ref || first.Ref == source.Ref || second.Ref == source.Ref {
					t.Fatal("independent calls reused a map identity")
				}
				// Holders share the wrapper's frozen storage copy-on-write; every
				// participant is marked and owns a descriptor for its own Ref.
				if !sameMapBacking(first.Map, source.Map) || !sameMapBacking(second.MapKeys, source.MapKeys) ||
					&first.MapOrder[0] != &source.MapOrder[0] {
					t.Fatal("independent holders did not share the frozen storage")
				}
				for _, participant := range []Value{first, second, source} {
					descriptor := fieldMapHolderDescriptor(participant)
					if !participant.mapBackingShared || descriptor == nil || descriptor.value.Ref != participant.Ref {
						t.Fatal("a participant lacks its shared marker or own descriptor before publication")
					}
				}
				if fieldMapHolderDescriptor(first) == fieldMapHolderDescriptor(second) || fieldMapHolderDescriptor(first) == fieldMapHolderDescriptor(source) {
					t.Fatal("independent holders share one ownership descriptor")
				}
				private := privateDescribeCollection(first)
				if sameMapBacking(private.Map, first.Map) || sameMapBacking(private.MapKeys, first.MapKeys) ||
					&private.MapOrder[0] == &first.MapOrder[0] {
					t.Fatal("direct collection helper lost its eager-copy contract")
				}
				// Field tokens retain their intentional within-VM shallow aliases.
				key := mapKey(String("Name"))
				first.Map[key].Fields["label"] = String("private label")
				if second.Map[key].Fields["label"].Text != "private label" || source.Map[key].Fields["label"].Text != "private label" {
					t.Fatal("independent map holders deep-copied field tokens")
				}
			})
		}
	}
}

func TestFieldMapReadCoercionLeavesSourceUntouched(t *testing.T) {
	for _, target := range []string{"Map<String,Schema.SObjectField>", "Map<String,SObjectField>", "Map<String,Object>", "Map<String,Schema.DescribeFieldResult>"} {
		t.Run(target, func(t *testing.T) {
			machine := describeShareUnprimed()
			wrapper := fieldMapWrapper(t, machine, "Account")
			first := fieldMapGet(t, machine, wrapper)
			second := fieldMapGet(t, machine, wrapper)
			before := privateDescribeCollection(wrapper.Fields["map"])
			converted, err := machine.coerceAssignable(target, first)
			if err != nil {
				t.Fatal(err)
			}
			if len(converted.Map) != len(first.Map) || converted.Type != target {
				t.Fatalf("conversion changed shape: %s, %d entries", converted.Type, len(converted.Map))
			}
			for _, untouched := range []Value{second, wrapper.Fields["map"]} {
				if !reflect.DeepEqual(before.Map, untouched.Map) || !reflect.DeepEqual(before.MapKeys, untouched.MapKeys) ||
					!reflect.DeepEqual(before.MapOrder, untouched.MapOrder) {
					t.Fatal("coercion rewrote a sibling or the wrapper's frozen container")
				}
			}
		})
	}
}

// A coercion that changes entry representations historically updates existing
// aliases of that holder. Detaching shared storage must preserve that writeback
// while leaving independently returned maps and the source wrapper alone.
func TestFieldMapReadCoercionPreservesStoredAliases(t *testing.T) {
	for _, target := range []string{"Map<String,Object>", "Map<String,Schema.DescribeFieldResult>"} {
		t.Run(target, func(t *testing.T) {
			machine := describeShareUnprimed()
			wrapper := fieldMapWrapper(t, machine, "Account")
			first := fieldMapGet(t, machine, wrapper)
			second := fieldMapGet(t, machine, wrapper)
			owner := Object("FieldMapHolder")
			owner.Fields["fields"] = first
			machine.Globals["first"] = first
			machine.Globals["alias"] = first
			machine.Globals["nested"] = List(owner)
			converted, err := machine.coerceAssignable(target, first)
			if err != nil {
				t.Fatal(err)
			}
			key := mapKey(String("Name"))
			// The original Go-local value models an argument retained while
			// later arguments are coerced, before the callee frame is installed.
			// Publishing only to VM scope roots cannot update this alias; it
			// observes the detached storage at the collection read boundary.
			for _, alias := range []Value{first, machine.Globals["first"], machine.Globals["alias"], machine.Globals["nested"].List[0].Fields["fields"]} {
				alias = alias.currentMapKeySet()
				if alias.Ref != first.Ref || alias.Type != first.Type || !reflect.DeepEqual(alias.Map[key], converted.Map[key]) {
					t.Fatalf("coercion lost an existing holder alias: type=%s ref preserved=%v entry=%+v converted=%+v", alias.Type, alias.Ref == first.Ref, alias.Map[key], converted.Map[key])
				}
			}
			for _, independent := range []Value{second, wrapper.Fields["map"]} {
				if independent.Map[key].Type != "Schema.SObjectField" || independent.Map[key].Static != "" {
					t.Fatalf("coercion reached independent storage: %+v", independent.Map[key])
				}
			}
		})
	}
}

func TestFieldMapReadUnknownBackingStaysEager(t *testing.T) {
	machine := New(nil)
	ordinary := Map()
	ordinary.Map[mapKey(String("entry"))] = Int(1)
	for _, input := range []Value{ordinary, List(Int(1)), Null} {
		wrapper := Object("Schema.SObjectFieldMap")
		wrapper.Fields["map"] = input
		got, _, _, _, err := machine.callPlatformObjectMember(wrapper, "getMap", nil, &Result{})
		if err != nil || got.Kind != input.Kind {
			t.Fatalf("fallback changed: %v %v", got.Kind, err)
		}
		if input.Kind == ValueMap && (sameMapBacking(input.Map, got.Map) || got.Ref == input.Ref) {
			t.Fatal("unproven wrapper storage escaped without an eager copy")
		}
	}
}

func TestFieldMapReadMutationAndTokenRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespaced := range []bool{false, true} {
			for _, mutation := range []string{
				"keys.remove('name');", "keys.clear();", "keys.removeAll(new Set<String>{'name'});", "keys.retainAll(new Set<String>{'id'});",
			} {
				t.Run(api+fmt.Sprint(namespaced)+mutation, func(t *testing.T) {
					base, template := describeSharePrimedBaseFor(t, namespaced)
					source := `Map<String,Schema.SObjectField> first = Account.SObjectType.fields.getMap();
Map<String,Schema.SObjectField> second = Account.SObjectType.fields.getMap();
Map<String,Schema.SObjectField> alias = first;
Map<String,Schema.SObjectField> copy = first.clone();
Integer beforeSize = first.size();
Set<String> keys = first.keySet();
` + mutation + `
System.assertEquals(beforeSize, first.size());
System.assertEquals(beforeSize, second.size());
System.assertEquals(beforeSize, alias.size());
System.assertEquals(beforeSize, copy.size());
Schema.SObjectField token = copy.get('Name');
token.label = 'private label';
System.assertEquals('private label', first.get('Name').label);
System.assertEquals('private label', second.get('Name').label);
System.assertEquals('private label', Account.SObjectType.fields.Name.label);`
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for range 2 {
						machine := describeShareClone(base, template)
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						describeShareRunApex(t, describeShareClone(base, template), `System.assertEquals('Account Name', Account.SObjectType.fields.getMap().get('Name').label);`)
					}
				})
			}
			for _, mutation := range []string{"put('private', Account.Name)", "putAll(new Map<String,Schema.SObjectField>{'private'=>Account.Name})", "remove('Name')"} {
				t.Run(api+fmt.Sprint(namespaced)+mutation, func(t *testing.T) {
					machine := describeShareUnprimedFor(namespaced)
					program, err := CompileAnonymousWithOptions(`Map<String,Schema.SObjectField> fields = Account.SObjectType.fields.getMap(); fields.`+mutation+`;`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "System.FinalException: Collection is read-only") {
						t.Fatalf("read-only mutation changed: %v", err)
					}
				})
			}
		}
	}
}
