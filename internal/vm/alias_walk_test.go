package vm

import (
	"strconv"
	"testing"
)

func TestAliasWalkDeclaredObjectTypes(t *testing.T) {
	cases := []struct {
		typeName string
		want     bool
	}{
		{"List<String>", true},
		{"Map<String,Boolean>", true},
		{"Set<Id>", true},
		{"Map<String,List<Integer>>", true},
		{"Map<Set<Id>,List<Map<String,Decimal>>>", true},
		{"System.List<System.String>", true},
		{" list < sTrInG > ", true},
		{"List<Long>", true},
		{"List<Double>", true},
		{"List<Date>", true},
		{"List<Datetime>", true},
		{"List<Time>", true},
		{"List<Blob>", true},
		{"Object", false},
		{"SObject", false},
		{"Account", false},
		{"MyClass", false},
		{"Map<String,Object>", false},
		{"List<SObject>", false},
		{"List<AliasWalkInterface>", false},
		{"Map<Id,Account>", false},
		{"Map<MyClass,String>", false},
		{"List<T>", false},
		{"List<Type>", false},
		{"List<Schema.SObjectType>", false},
		{"List<Bool>", false},
		{"List<Int>", false},
		{"List<Currency>", false},
		{"List<Url>", false},
		{"List<Uuid>", false},
		{"List", false},
		{"", false},
		{" ", false},
		{"Map<String", false},
		{"Map<String,>", false},
		{"Map<String,Boolean,Integer>", false},
		{"List<>", false},
		{"List<String,Boolean>", false},
		{"List<String>>", false},
		{"Custom<String>", false},
		{"String", false},
	}
	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			for _, kind := range []ValueKind{ValueList, ValueSet, ValueMap} {
				value := Value{Kind: kind, Type: tc.typeName}
				// Repeat the verdict to exercise the memoized path as well.
				for range 2 {
					if got := valueDeclaredTypesCannotContainAliasKind(value, ValueObject); got != tc.want {
						t.Fatalf("kind %v, type %q: pruned = %v, want %v", kind, tc.typeName, got, tc.want)
					}
				}
			}
		})
	}
}

func TestAliasWalkDeclaredTypeHints(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  bool
	}{
		{"static fallback", Value{Kind: ValueList, Static: "List<String>"}, true},
		{"runtime fallback", Value{Kind: ValueList, Type: " ", Runtime: "List<String>"}, true},
		{"all scalar", Value{Kind: ValueMap, Type: "Map<String,Boolean>", Static: "Map<Id,Date>", Runtime: "Map<String,List<Integer>>"}, true},
		{"unknown type", Value{Kind: ValueList, Type: "Object", Static: "List<String>"}, false},
		{"unknown static", Value{Kind: ValueList, Type: "List<String>", Static: "List<Object>"}, false},
		{"unknown runtime", Value{Kind: ValueList, Type: "List<String>", Runtime: "List<Account>"}, false},
		{"wider runtime", Value{Kind: ValueList, Type: "List<String>", Static: "List<String>", Runtime: "List<Object>"}, false},
		{"sobject runtime", Value{Kind: ValueList, Type: "List<String>", Static: "List<String>", Runtime: "List<SObject>"}, false},
		{"interface runtime", Value{Kind: ValueList, Type: "List<String>", Static: "List<String>", Runtime: "List<AliasWalkInterface>"}, false},
		{"malformed static", Value{Kind: ValueList, Type: "List<String>", Static: "List<"}, false},
		{"no hints", Value{Kind: ValueList}, false},
		{"object is not collection", Value{Kind: ValueObject, Type: "List<String>"}, false},
		{"scalar is not collection", Value{Kind: ValueString, Type: "List<String>"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := valueDeclaredTypesCannotContainAliasKind(tc.value, ValueObject); got != tc.want {
				t.Fatalf("pruned = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAliasWalkDeclaredCollectionTargets(t *testing.T) {
	cases := []struct {
		typeName string
		kind     ValueKind
		want     bool
	}{
		{"List<String>", ValueList, false},
		{"List<String>", ValueMap, true},
		{"Set<Id>", ValueSet, false},
		{"Set<Id>", ValueList, true},
		{"Map<String,Boolean>", ValueMap, false},
		{"Map<String,Boolean>", ValueList, true},
		{"Map<String,List<Integer>>", ValueList, false},
		{"Map<String,List<Integer>>", ValueSet, true},
		{"List<Object>", ValueMap, false},
		{"List<SObject>", ValueMap, false},
		{"List<MyClass>", ValueMap, false},
		{"Map<Type,List<BaseDomain>>", ValueSet, false},
		{"List<Type>", ValueMap, true},
		{"Schema.GlobalDescribeMap", ValueMap, true},
		{"Schema.FieldSet", ValueList, false},
		{"List", ValueList, false},
		{"Map<String", ValueMap, false},
		{"", ValueList, false},
	}
	for _, tc := range cases {
		t.Run(tc.typeName+"/"+string(tc.kind), func(t *testing.T) {
			value := Value{Kind: ValueMap, Type: tc.typeName}
			for range 2 {
				if got := valueDeclaredTypesCannotContainAliasKind(value, tc.kind); got != tc.want {
					t.Fatalf("pruned = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestAliasWalkMapKeyMemoRetainsObjectType(t *testing.T) {
	for _, mapType := range []string{"Map<Type,String>", " Map<Type,String> "} {
		for _, target := range []struct {
			typeName string
			want     bool
		}{{"Account", true}, {"Type", false}, {"Account", true}, {"Type", false}} {
			if got := mapKeyTypeCannotContainAlias(mapType, aliasSnapshot{kind: ValueObject, typeName: target.typeName}); got != target.want {
				t.Fatalf("map %q, target %q: pruned = %v, want %v", mapType, target.typeName, got, target.want)
			}
		}
	}
}

func TestAliasWalkContainmentCacheBoundary(t *testing.T) {
	for _, kind := range []ValueKind{ValueList, ValueSet, ValueMap} {
		for _, size := range []int{8, 9} {
			for _, targetInKey := range []bool{false, true} {
				if targetInKey && kind != ValueMap {
					continue
				}
				name := string(kind) + "/" + strconv.Itoa(size) + "/key=" + strconv.FormatBool(targetInKey)
				t.Run(name, func(t *testing.T) {
					machine := New(nil)
					target := Object("Account")
					previous := snapshotAlias(target)
					root := Value{Kind: kind, Ref: newValueRef()}
					switch kind {
					case ValueList:
						root.Type, root.List = "List<Object>", make([]Value, size)
					case ValueSet:
						root.Type, root.Set = "Set<Object>", make([]Value, size)
					case ValueMap:
						root.Type = "Map<Object,Object>"
						root.Map, root.MapKeys = make(map[string]Value), make(map[string]Value)
						for i := range size / 2 {
							root.Map[strconv.Itoa(i)] = Null
						}
						for i := range size - size/2 {
							root.MapKeys[strconv.Itoa(i)] = Null
						}
					}
					probe := scopeAliasProbe{}
					if machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &probe) {
						t.Fatal("empty root contained target")
					}
					wantEntries := 0
					if size == 9 {
						wantEntries = 1
					}
					if len(machine.aliasContainmentCache) != wantEntries || probe.containmentCacheMisses != uint64(wantEntries) {
						t.Fatalf("entries = %d, cache misses = %d, want %d each", len(machine.aliasContainmentCache), probe.containmentCacheMisses, wantEntries)
					}
					// Seed a miss even for the small root, which must ignore it.
					key := aliasContainmentCacheKey{ValueRef: root.Ref, ValueKind: root.Kind, ValueType: root.Type, PreviousRef: target.Ref, PreviousKind: target.Kind}
					machine.rememberAliasContainmentMiss(key)
					switch kind {
					case ValueList:
						root.List[0] = target
					case ValueSet:
						root.Set[0] = target
					case ValueMap:
						if targetInKey {
							root.MapKeys["0"] = target
						} else {
							root.Map["0"] = target
						}
					}
					updated := target
					updated.Fields = map[string]Value{"Name": String("x")}
					if size == 9 {
						probe = scopeAliasProbe{}
						if machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &probe) || probe.containmentCacheHits != 1 || probe.recursiveVisits != 1 {
							t.Fatalf("large root did not reuse miss: %#v", probe)
						}
						if machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
							t.Fatal("large non-probe walk did not reuse miss")
						}
						if _, changed := replaceValueAliasRefWithCache(machine, root, previous, updated, make(map[uint64]bool)); changed {
							t.Fatal("large replacement walk did not reuse miss")
						}
						machine.advanceAliasContainmentMutation()
					}
					probe = scopeAliasProbe{}
					if !machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &probe) || probe.containmentCacheHits != 0 {
						t.Fatalf("target was hidden by miss: %#v", probe)
					}
					if !machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
						t.Fatal("non-probe walk missed target")
					}
					replaced, changed := replaceValueAliasRefWithCache(machine, root, previous, updated, make(map[uint64]bool))
					if !changed {
						t.Fatal("replacement walk missed target")
					}
					var got Value
					switch kind {
					case ValueList:
						got = replaced.List[0]
					case ValueSet:
						got = replaced.Set[0]
					case ValueMap:
						got = replaced.Map["0"]
						if targetInKey {
							got = replaced.MapKeys["0"]
						}
					}
					if got.Ref != target.Ref || got.Fields["Name"].Text != "x" {
						t.Fatalf("replaced target = %#v", got)
					}
				})
			}
		}
	}
}

func TestAliasWalkObjectHoldersPreserveAliases(t *testing.T) {
	const setup = `
Account original = new Account(Name = 'before');
List<Account> typed = new List<Account>{original};
List<SObject> generic = new List<SObject>{original};
List<Object> objects = new List<Object>{original};
Map<String,Object> values = new Map<String,Object>{'row' => original};

AliasWalkHolder holder = new AliasWalkHolder();
holder.o = original;
AliasWalkEnvelope envelope = new AliasWalkEnvelope();
envelope.o = original;
envelope.f = holder;
List<AliasWalkInterface> interfaces = new List<AliasWalkInterface>{holder};

Map<String,List<Object>> nested = new Map<String,List<Object>>{
    'rows' => new List<Object>{original}
};
AliasWalkHolder setHolder = new AliasWalkHolder();
setHolder.o = original;
Set<Object> members = new Set<Object>{setHolder};
String expected;
`
	const checks = `
System.assertEquals(expected, original.Name, 'original');
System.assertEquals(expected, typed[0].Name, 'List<Account>');
System.assertEquals(expected, generic[0].get('Name'), 'List<SObject>');
System.assertEquals(expected, ((Account)objects[0]).Name, 'List<Object>');
System.assertEquals(expected, ((Account)values.get('row')).Name, 'Map<String,Object>');
System.assertEquals(expected, ((Account)envelope.o).Name, 'Object field');
System.assertEquals(expected, ((Account)((AliasWalkHolder)envelope.f).o).Name, 'interface field');
System.assertEquals(expected, ((Account)((AliasWalkHolder)interfaces[0]).o).Name, 'interface list');
System.assertEquals(expected, ((Account)nested.get('rows')[0]).Name, 'nested map/list');
System.assertEquals(1, members.size());
for (Object member : members) {
    System.assertEquals(expected, ((Account)((AliasWalkHolder)member).o).Name, 'Set<Object> holder');
}
`
	for _, snapshots := range []bool{false, true} {
		t.Run("snapshots="+strconv.FormatBool(snapshots), func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, setup,
				Class{Name: "AliasWalkInterface", IsInterface: true},
				Class{
					Name: "AliasWalkHolder", Interfaces: []string{"AliasWalkInterface"},
					Fields: map[string]Field{"o": {Name: "o", Type: "Object"}},
				},
				Class{
					Name: "AliasWalkEnvelope",
					Fields: map[string]Field{
						"o": {Name: "o", Type: "Object"},
						"f": {Name: "f", Type: "AliasWalkInterface"},
					},
				},
			)
			for _, mutation := range []string{
				"expected = 'assigned'; original.Name = expected;",
				"expected = 'put'; original.put('Name', expected);",
			} {
				if snapshots {
					// Retain Apex identities while separating backing storage, so
					// shared Go maps cannot hide a missed alias replacement.
					for name, value := range machine.Globals {
						if name != "original" {
							machine.Globals[name] = cloneValuePreserveRefs(value)
						}
					}
				}
				program, err := CompileAnonymous(mutation)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("%s: %v", mutation, err)
				}
				// Inspect storage before Apex reads can refresh an alias.
				globals := machine.Globals
				for name, alias := range map[string]Value{
					"typed":     globals["typed"].List[0],
					"generic":   globals["generic"].List[0],
					"objects":   globals["objects"].List[0],
					"values":    globals["values"].Map[mapKey(String("row"))],
					"object":    globals["envelope"].Fields["o"],
					"interface": globals["envelope"].Fields["f"].Fields["o"],
					"list":      globals["interfaces"].List[0].Fields["o"],
					"nested":    globals["nested"].Map[mapKey(String("rows"))].List[0],
					"set":       globals["members"].Set[0].Fields["o"],
				} {
					if alias.Ref != globals["original"].Ref || !alias.Fields["Name"].Equal(globals["expected"]) {
						t.Errorf("%s: %s alias Ref=%d Name=%v, want Ref=%d Name=%v", mutation, name,
							alias.Ref, alias.Fields["Name"], globals["original"].Ref, globals["expected"])
					}
				}
				program, err = CompileAnonymous(checks)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("%s: %v", mutation, err)
				}
			}
		})
	}
}

func TestAliasWalkWiderTypeHintsPreserveAliases(t *testing.T) {
	for _, kind := range []ValueKind{ValueList, ValueMap} {
		for _, hint := range []string{"Type", "Static", "Runtime"} {
			t.Run(string(kind)+"/"+hint, func(t *testing.T) {
				machine := New(nil)
				target := Object("Account")
				var root Value
				narrow, wide := "List<String>", "List<Object>"
				if kind == ValueList {
					root = List(target)
				} else {
					root = Map()
					key := mapKey(String("row"))
					root.Map[key], root.MapKeys[key] = target, String("row")
					narrow, wide = "Map<String,Boolean>", "Map<String,Object>"
				}
				root.Type, root.Static, root.Runtime = narrow, narrow, narrow
				switch hint {
				case "Type":
					root.Type = wide
				case "Static":
					root.Static = wide
				case "Runtime":
					root.Runtime = wide
				}
				previous := snapshotAlias(target)
				if valueCannotContainAliasRef(root, target.Ref, target.Kind) {
					t.Fatal("wider hint did not veto pruning")
				}
				if !machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
					t.Fatal("containment dropped the alias")
				}
				if !machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &scopeAliasProbe{}) {
					t.Fatal("instrumented containment dropped the alias")
				}
				// Put the collection under an object so both walkers must honor
				// all its type hints at the field-level pruning point as well.
				holder := Object("Holder")
				holder.Fields["items"] = root
				scope := map[string]Value{"holder": holder}
				updated := target
				updated.Fields = map[string]Value{"Name": String("after")}
				machine.propagateAliasSnapshotToScope(scope, previous, updated)
				root = scope["holder"].Fields["items"]
				var got Value
				if kind == ValueList {
					got = root.List[0]
				} else {
					got = root.Map[mapKey(String("row"))]
				}
				if got.Ref != target.Ref || !got.Fields["Name"].Equal(String("after")) {
					t.Fatalf("replacement dropped the alias: %#v", got)
				}
			})
		}
	}
}

func TestAliasWalkEnhancedForScalarMarkers(t *testing.T) {
	const size = 600
	machine, stats := runDynamicObjectAliasProgram(t, `
List<Account> rows = new List<Account>();
for (Integer i = 0; i < 600; i++) {
    rows.add(new Account());
}
for (Account e : rows) {
    e.Name = 'x';
}
`)
	rows := machine.Globals["rows"].List
	if len(rows) != size {
		t.Fatalf("record count = %d, want %d", len(rows), size)
	}
	for i, row := range rows {
		if name := row.Fields["Name"]; !name.Equal(String("x")) {
			t.Fatalf("rows[%d].Name = %#v, want x", i, name)
		}
	}
	// Allow 20% above two triangular element walks; scalar marker maps
	// must not multiply the number of recursive visits.
	const maxVisits = 2 * size * (size + 1) / 2 * 12 / 10
	t.Logf("n=%d: RecursiveVisits=%d, limit=%d", size, stats.RecursiveVisits, maxVisits)
	if stats.RecursiveVisits > maxVisits {
		t.Fatalf("RecursiveVisits = %d, want at most %d", stats.RecursiveVisits, maxVisits)
	}
}
