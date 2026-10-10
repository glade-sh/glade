package vm

import (
	"fmt"
	"strings"
	"testing"
)

func TestAliasEscapeExpressionSObjectSetters(t *testing.T) {
	for _, factory := range []string{"new Account()", "(Account)Schema.getGlobalDescribe().get('Account').newSObject(null, false)"} {
		for _, receiver := range []string{"holders[0]", "holders.get(0)"} {
			for _, setter := range []string{"put", "putSObject"} {
				t.Run(factory+"/"+receiver+"/"+setter, func(t *testing.T) {
					machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
List<Account> holders = new List<Account>{new Account()};
Account original = %s;
String expected;
`, factory))
					if !machine.localOnlyObjectRefs[machine.Globals["original"].Ref] {
						t.Fatal("fixture record must be local-only before publication")
					}
					if setter == "put" {
						// S006/S007 reject relationship names for put; S012 publishes
						// the parent through putSObject before the alias checks.
						executeAliasEscapeProgram(t, machine, fmt.Sprintf(`
String exceptionType;
String message;
try { %s.put('Parent', original); }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Invalid field Parent for Account', message);
`, receiver))
					}
					executeAliasEscapeProgram(t, machine, receiver+".putSObject('Parent', original);")
					// No Apex getter or assertion may receive the record before this check.
					requireAliasEscaped(t, machine, machine.Globals["original"])
					checkAliasEscapeMutations(t, machine, func() map[string]Value {
						return map[string]Value{"Parent": machine.Globals["holders"].List[0].Fields["Parent"]}
					}, `System.assertEquals(expected, holders[0].Parent.Name);`)
				})
			}
		}
	}
}

// PC001-PC006 / PC101-PC106: cache values remain shared within a transaction.
// getPartition receiver equivalence and returned-map mutation use store analogy.
func TestAliasEscapePlatformCacheSharedReferences(t *testing.T) {
	for _, scope := range []string{"Org", "Session"} {
		for _, receiver := range []string{"static", "partition"} {
			cache := "Cache." + scope
			if receiver == "partition" {
				cache += ".getPartition('local.default')"
			}
			for _, tc := range []struct{ name, setup, mutation, check string }{
				{"PC001_PC006/list-original", "List<Integer> original=new List<Integer>{1};", "original.add(2);", "System.assertEquals('[1,2]', JSON.serialize(returned));"},
				{"PC002_PC006/list-returned", "List<Integer> original=new List<Integer>{1};", "((List<Integer>)" + cache + ".get('row')).add(2);", "System.assertEquals('[1,2]', JSON.serialize(returned));"},
				{"PC003/map-original", "Map<String,Integer> original=new Map<String,Integer>{'a'=>1};", "original.put('b',2);", "System.assertEquals(2, ((Map<String,Integer>)returned).get('b'));"},
				{"map-returned-store-analogy", "Map<String,Integer> original=new Map<String,Integer>{'a'=>1};", "((Map<String,Integer>)" + cache + ".get('row')).put('b',2);", "System.assertEquals(2, original.get('b')); System.assertEquals(2, ((Map<String,Integer>)returned).get('b'));"},
				{"PC004/sobject-original", "Account original=new Account(Name='x');", "original.Name='y';", "System.assertEquals('y', ((Account)returned).Name);"},
				{"PC005/sobject-returned", "Account original=(Account)Schema.getGlobalDescribe().get('Account').newSObject(); original.Name='x';", "((Account)" + cache + ".get('row')).Name='y';", "System.assertEquals('y', original.Name); System.assertEquals('y', ((Account)returned).Name);"},
			} {
				t.Run(scope+"/"+receiver+"/"+tc.name, func(t *testing.T) {
					machine, _ := runDynamicObjectAliasProgram(t, tc.setup+cache+".put('row', original);")
					original := machine.Globals["original"]
					stored := machine.platformCache["cache."+strings.ToLower(scope)+"partition:local.default"]["row"].Value
					if stored.Ref == 0 || stored.Ref != original.Ref {
						t.Fatal("put must retain the original Ref")
					}
					if original.Kind == ValueObject {
						requireAliasEscaped(t, machine, original)
					}
					// Warm the index before mutation: putting after an earlier miss must work.
					machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
					executeAliasEscapeProgram(t, machine, tc.mutation)
					stored = machine.platformCache["cache."+strings.ToLower(scope)+"partition:local.default"]["row"].Value
					if original.Kind == ValueList && len(stored.List) != 2 {
						t.Fatal("cache root kept a stale slice header")
					}
					executeAliasEscapeProgram(t, machine, "Object returned="+cache+".get('row'); System.assert(original === returned); "+tc.check)
				})
			}
		}
	}
}

// PC008/PC108 and PC009/PC109 preserve both the keySet-derived key and the
// original custom-object key. getPartition uses the same store by analogy.
func TestAliasEscapePlatformCacheSharedMapKeys(t *testing.T) {
	for _, scope := range []string{"Org", "Session"} {
		for _, receiver := range []string{"static", "partition"} {
			cache := "Cache." + scope
			if receiver == "partition" {
				cache += ".getPartition('local.default')"
			}
			for _, key := range []string{"originalKey", "cachedKey"} {
				t.Run(scope+"/"+receiver+"/"+key, func(t *testing.T) {
					runDynamicObjectAliasProgram(t, fmt.Sprintf(`
SCWrap originalKey=new SCWrap();
Map<SCWrap,String> original=new Map<SCWrap,String>{originalKey=>'v'};
%[1]s.put('row',original);
Map<SCWrap,String> cached=(Map<SCWrap,String>)%[1]s.get('row');
SCWrap cachedKey=new List<SCWrap>(cached.keySet())[0];
System.assert(original === cached);
System.assert(originalKey === cachedKey);
System.assert(cached.containsKey(%[2]s));
cached.remove(%[2]s);
System.assertEquals(0, cached.size());
System.assertEquals(0, original.size());
cached.put(%[2]s,'w');
System.assertEquals(1,cached.size());
System.assertEquals('w', original.get(originalKey));
System.assertEquals('w', ((Map<SCWrap,String>)%[1]s.get('row')).get(originalKey));
`, cache, key), Class{Name: "SCWrap", Fields: map[string]Field{"n": {Name: "n", Type: "Integer"}}})
				})
			}
		}
	}
}

// Multi-key, builder and scan paths are not in the native capture. They share
// the store and therefore return the same references (store-analogy coverage).
func TestAliasEscapePlatformCacheMultiGetSharedReferences(t *testing.T) {
	for _, scope := range []string{"Org", "Session"} {
		for _, receiver := range []string{"static", "partition"} {
			cache := "Cache." + scope
			if receiver == "partition" {
				cache += ".getPartition('local.default')"
			}
			t.Run(scope+"/"+receiver, func(t *testing.T) {
				runDynamicObjectAliasProgram(t, fmt.Sprintf(`
List<Integer> original=new List<Integer>{1};
%[1]s.put('row',original);
Map<String,Object> multi=%[1]s.get(new Set<String>{'row'});
List<Integer> returned=(List<Integer>)multi.get('row');
System.assert(original === returned);
returned.add(2);
System.assertEquals('[1,2]',JSON.serialize(%[1]s.get('row')));
original.add(3);
System.assertEquals('[1,2,3]',JSON.serialize(((Map<String,Object>)%[1]s.get(new Set<String>{'row'})).get('row')));
`, cache))
			})
		}
		// Legacy static List overload: before the supported API floor.
		t.Run(scope+"/legacy-list", func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, "List<Integer> original=new List<Integer>{1}; Cache."+scope+".put('row',original);")
			values, err := machine.cacheStaticDefaultGet("Cache."+scope+".get", []Value{List(String("row"))})
			if err != nil {
				t.Fatal(err)
			}
			if values.List[0].Ref != machine.Globals["original"].Ref {
				t.Fatal("multi-get changed Ref")
			}
		})
	}
}

func TestAliasEscapePlatformCacheBuilderSharedReferences(t *testing.T) {
	load, err := CompileAnonymous(`return AliasCacheBuilder.Source;`)
	if err != nil {
		t.Fatal(err)
	}
	loader := Class{Name: "AliasCacheBuilder", Interfaces: []string{"Cache.CacheBuilder"},
		StaticFields: map[string]Field{"Source": {Name: "Source", Type: "List<Integer>", Static: true}},
		Methods:      map[string]Method{"doLoad": {Name: "AliasCacheBuilder.doLoad", ClassName: "AliasCacheBuilder", ReturnType: "Object", Params: []Param{{Name: "key", Type: "String"}}, Program: load}}}
	for _, scope := range []string{"Org", "Session"} {
		for _, receiver := range []string{"static", "partition"} {
			cache := "Cache." + scope
			if receiver == "partition" {
				cache += ".getPartition('local.default')"
			}
			t.Run(scope+"/"+receiver, func(t *testing.T) {
				runDynamicObjectAliasProgram(t, fmt.Sprintf(`
AliasCacheBuilder.Source=new List<Integer>{1};
List<Integer> first=(List<Integer>)%[1]s.get(AliasCacheBuilder.class,'row');
System.assert(first === AliasCacheBuilder.Source);
first.add(2);
List<Integer> second=(List<Integer>)%[1]s.get(AliasCacheBuilder.class,'row');
System.assert(first === second);
System.assertEquals('[1,2]',JSON.serialize(second));
AliasCacheBuilder.Source.add(3);
List<Integer> third=(List<Integer>)%[1]s.get(AliasCacheBuilder.class,'row');
System.assertEquals('[1,2,3]',JSON.serialize(third));
`, cache), loader)
			})
		}
	}
}

func TestAliasEscapePlatformCacheScanSharedReferences(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "List<Integer> original=new List<Integer>{1};")
	original := machine.Globals["original"]
	partition := cacheSecondaryKeyPartition("feature")
	machine.cachePutSecondary(partition, "row", original, "secondary")
	scan := machine.cacheScanResult(machine.cacheSecondaryScan(partition, "", ""), 1)
	returned := scan.Fields["result"].Map[mapKey(String("row"))]
	if returned.Ref != original.Ref {
		t.Fatal("scan changed Ref")
	}
	machine.Globals["returned"] = returned
	executeAliasEscapeProgram(t, machine, "returned.add(2);")
	stored, ok := machine.cacheGet(partition, "row")
	if !ok || len(stored.List) != 2 || stored.Ref != original.Ref {
		t.Fatal("scan mutation did not reach the secondary cache root")
	}
	executeAliasEscapeProgram(t, machine, "original.add(3);")
	scan = machine.cacheScanResult(machine.cacheSecondaryScan(partition, "", ""), 1)
	if got := scan.Fields["result"].Map[mapKey(String("row"))]; len(got.List) != 3 || got.Ref != original.Ref {
		t.Fatal("original mutation did not reach scan")
	}
}

// Store-analogy controls for nested roots, reverse-index lifecycle and frozen
// runtime copies. These internal graphs are not additional Salesforce rows.
func TestAliasEscapePlatformCacheRootLifecycle(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "List<Integer> child=new List<Integer>{1}; List<List<Integer>> original=new List<List<Integer>>{child};")
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	executeAliasEscapeProgram(t, machine, "Cache.Org.put('row',original); child.add(2);")
	value, ok := machine.cacheGet("cache.orgpartition:local.default", "row")
	if !ok || len(value.List[0].List) != 2 {
		t.Fatal("nested list mutation did not reach cache after warm empty index")
	}
	executeAliasEscapeProgram(t, machine, "Cache.Org.put('row',new List<Integer>{3}); child.add(4);")
	executeAliasEscapeProgram(t, machine, "System.assertEquals('[3]',JSON.serialize(Cache.Org.get('row')));")
	ref := machine.platformCache["cache.orgpartition:local.default"]["row"].Value.Ref
	machine.FreezeClassLookup()
	clone := machine.CloneRuntimeFrozenShared(nil)
	cloneEntry := clone.platformCache["cache.orgpartition:local.default"]["row"].Value
	if ref == cloneEntry.Ref {
		t.Fatal("runtime copies must retain their existing isolated identities")
	}
	clone.Globals["copied"] = cloneEntry
	executeAliasEscapeProgram(t, clone, "copied.add(5);")
	updated, _ := clone.cacheGet("cache.orgpartition:local.default", "row")
	if len(updated.List) != 2 {
		t.Fatal("clone reused source cache root index")
	}
	value, _ = machine.cacheGet("cache.orgpartition:local.default", "row")
	if len(value.List) != 1 {
		t.Fatal("clone mutated source cache")
	}
	executeAliasEscapeProgram(t, machine, "Cache.Org.remove('row');")
	if len(machine.platformCache) != 0 || machine.staticValueRefs[ref] {
		t.Fatal("removed cache entry retained a root")
	}
	// Nested collections inside a cached SObject stay reachable as aliases.
	record := Object("Account")
	child := typedMap("Map<String,Integer>")
	record.Fields["Retained"] = child
	machine.cachePut("cache.orgpartition:local.default", "record", record, 1)
	machine.Globals["mapChild"] = child
	executeAliasEscapeProgram(t, machine, "mapChild.put('a',1);")
	value, _ = machine.cacheGet("cache.orgpartition:local.default", "record")
	if len(value.Fields["Retained"].Map) != 1 {
		t.Fatal("registry hid cached nested map alias")
	}
	machine.fakeNow = machine.fakeNow.Add(2 * 1000000000)
	if _, ok := machine.cacheGet("cache.orgpartition:local.default", "record"); ok || len(machine.platformCache) != 0 {
		t.Fatal("expiry retained a cache root")
	}
}

func TestAliasEscapePlatformCacheNewNestedRefs(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, `
List<List<Integer>> middle=new List<List<Integer>>();
List<List<List<Integer>>> root=new List<List<List<Integer>>>{middle};
Cache.Org.put('row',root);
`)
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	executeAliasEscapeProgram(t, machine, `List<Integer> leaf=new List<Integer>{1}; middle.add(leaf);`)
	leaf := machine.Globals["leaf"]
	location := staticFieldRef{ClassName: "cache.orgpartition:local.default", FieldName: "row"}
	if !machine.staticValueRefFields[leaf.Ref].contains(location) {
		t.Fatal("inserted grandchild was not indexed at the cache root")
	}
	// Retain only the leaf in caller scope so refreshing root/middle locals
	// cannot hide a missing cache-root update.
	machine.Globals = map[string]Value{"leaf": leaf}
	executeAliasEscapeProgram(t, machine, `leaf.add(2);`)
	cached, _ := machine.cacheGet(location.ClassName, location.FieldName)
	if len(cached.List[0].List[0].List) != 2 {
		t.Fatal("new nested leaf mutation left a stale cached slice header")
	}
}

func TestAliasEscapeJSONConstructorChildren(t *testing.T) {
	constructor, err := CompileAnonymous(`this.Name = 'constructor';`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, typeName, json, bind, checks string
		child                              func(Value) Value
	}{
		{
			"list", "List<AliasEscapeChild>", `[{}]`, "AliasEscapeChild original = holder[0];",
			`System.assertEquals(1, holder.size()); System.assertEquals(expected, holder[0].Name);`,
			func(v Value) Value { return v.List[0] },
		},
		{
			"set", "Set<AliasEscapeChild>", `[{}]`, "AliasEscapeChild original; for (AliasEscapeChild item : holder) { original = item; }",
			`System.assertEquals(1, holder.size()); for (AliasEscapeChild item : holder) { System.assertEquals(expected, item.Name); }`,
			func(v Value) Value { return v.Set[0] },
		},
		{
			"map", "Map<String,AliasEscapeChild>", `{"row":{}}`, "AliasEscapeChild original = holder.get('row');",
			`System.assertEquals(1, holder.size()); System.assertEquals(expected, holder.get('row').Name);`,
			func(v Value) Value { return v.Map[mapKey(String("row"))] },
		},
		{
			"field", "AliasEscapeEnvelope", `{"Child":{}}`, "AliasEscapeChild original = holder.Child;",
			`System.assertEquals(expected, holder.Child.Name);`,
			func(v Value) Value { return v.Fields["Child"] },
		},
	}
	for _, childType := range []string{"AliasEscapeChild", "Account"} {
		// The user-class fixture exercises real constructor-created children.
		// A synthetic Account constructor also covers this VM branch with native
		// SObject.put; this shim is not a Salesforce custom Account constructor.
		childClass := Class{
			Name:   childType,
			Fields: map[string]Field{"Name": {Name: "Name", Type: "String"}},
			Constructors: []Method{{
				Name: childType + ".<init>", ClassName: childType, IsConstructor: true, Program: constructor,
			}},
		}
		envelopeClass := Class{
			Name:   "AliasEscapeEnvelope",
			Fields: map[string]Field{"Child": {Name: "Child", Type: childType}},
		}
		var mutations []string
		if childType == "AliasEscapeChild" {
			mutations = []string{"expected = 'assigned'; original.Name = expected;"}
		}
		for _, method := range []string{"deserialize", "deserializeStrict"} {
			for _, tc := range cases {
				t.Run(childType+"/"+method+"/"+tc.name, func(t *testing.T) {
					typeName := strings.ReplaceAll(tc.typeName, "AliasEscapeChild", childType)
					machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
%[4]s local = (%[4]s)JSON.%[1]s('{}', %[4]s.class);
%[2]s holder = (%[2]s)JSON.%[1]s('%[3]s', %[2]s.class);
String expected;
`, method, typeName, tc.json, childType), childClass, envelopeClass)
					local := machine.Globals["local"]
					if !machine.localOnlyObjectRefs[local.Ref] || local.Fields["Name"].Text != "constructor" {
						t.Fatal("standalone constructor-created root must remain local-only")
					}
					child := tc.child(machine.Globals["holder"])
					if child.Fields["Name"].Text != "constructor" || child.Ref == local.Ref {
						t.Fatal("nested child must have run its own constructor")
					}
					// Inspect the child immediately after JSON publishes it, before
					// binding an Apex variable or accessing any container member.
					requireAliasEscaped(t, machine, child)
					executeAliasEscapeProgram(t, machine, strings.ReplaceAll(tc.bind, "AliasEscapeChild", childType))
					checkAliasEscapeMutations(t, machine, func() map[string]Value {
						return map[string]Value{tc.name: tc.child(machine.Globals["holder"])}
					}, strings.ReplaceAll(tc.checks, "AliasEscapeChild", childType), mutations...)
				})
			}
		}
	}
}

func TestAliasEscapeHandoffRepros(t *testing.T) {
	constructor, err := CompileAnonymous(`this.ctorRan = true;`)
	if err != nil {
		t.Fatal(err)
	}
	childClass := Class{
		Name: "AliasEscapeChild",
		Fields: map[string]Field{
			"Name":    {Name: "Name", Type: "String"},
			"ctorRan": {Name: "ctorRan", Type: "Boolean"},
		},
		Constructors: []Method{{Name: "AliasEscapeChild.<init>", ClassName: "AliasEscapeChild", IsConstructor: true, Program: constructor}},
	}
	for _, tc := range []struct{ name, source, want string }{
		{"A", `
List<Account> holders = new List<Account>{new Account()};
Account parent = new Account();
holders[0].putSObject('Parent', parent);
parent.put('Name', 'after');
holders[0].put('Name', 'holder');
System.assertEquals('holder', holders[0].Name);
System.debug('A|retained=' + JSON.serialize(holders).contains('after'));
`, "A|retained=true"},
		{"B", `
Account original = new Account(Name = 'stored');
Cache.Org.put('aliascopy', original);
original.Name = 'source';
Account first = (Account)Cache.Org.get('aliascopy');
System.debug('B|first=' + first.Name);
first.Name = 'read';
Account second = (Account)Cache.Org.get('aliascopy');
System.debug('B|original=' + original.Name);
System.debug('B|second=' + second.Name);
`, "B|first=source\nB|original=read\nB|second=read"},
		// This ordinary Apex probe is a control, not a claimed observable
		// baseline failure: shared backing can mask C's wrong locality flag.
		{"C", `
List<AliasEscapeChild> rows = (List<AliasEscapeChild>)JSON.deserialize('[{"Name":"before"}]', List<AliasEscapeChild>.class);
AliasEscapeChild child = rows[0];
System.debug('C|ctor=' + String.valueOf(child.ctorRan));
child.Name = 'after';
System.debug('C|alias=' + rows[0].Name);
`, "C|ctor=true\nC|alias=after"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, "", childClass)
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(result.Debug, "\n")
			t.Logf("Glade System.debug output:\n%s", got)
			if got != tc.want {
				t.Fatalf("debug output = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("CDetached", func(t *testing.T) {
		machine, _ := runDynamicObjectAliasProgram(t, `
List<AliasEscapeChild> rows = (List<AliasEscapeChild>)JSON.deserialize('[{"Name":"before"}]', List<AliasEscapeChild>.class);
AliasEscapeChild child = rows[0];
`, childClass)
		// This is an explicit white-box snapshot boundary, not an Apex action.
		machine.Globals["rows"] = cloneValuePreserveRefs(machine.Globals["rows"])
		program, err := CompileAnonymous(`
child.Name = 'after';
System.debug('C|detached-alias=' + rows[0].Name);
`)
		if err != nil {
			t.Fatal(err)
		}
		result, err := machine.Execute(program)
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(result.Debug, "\n")
		t.Logf("Glade System.debug output with detached holder:\n%s", got)
		if got != "C|detached-alias=after" {
			t.Fatalf("debug output = %q, want C|detached-alias=after", got)
		}
	})
}

func executeAliasEscapeProgram(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("%s: %v", source, err)
	}
}

func requireAliasEscaped(t *testing.T, machine *VM, value Value) {
	t.Helper()
	if value.Kind != ValueObject || value.Ref == 0 {
		t.Fatalf("expected a record with a Ref, got kind=%s Ref=%d", value.Kind, value.Ref)
	}
	if machine.localOnlyObjectRefs[value.Ref] {
		t.Fatalf("published %s Ref=%d is still local-only", value.Type, value.Ref)
	}
}

func checkAliasEscapeMutations(t *testing.T, machine *VM, retained func() map[string]Value, checks string, mutations ...string) {
	t.Helper()
	if len(mutations) == 0 {
		mutations = []string{
			"expected = 'put'; original.put('Name', expected);",
			"expected = 'assigned'; original.Name = expected;",
		}
	}
	for _, mutation := range mutations {
		// Preserve Apex identities but detach holder storage before each write.
		for name, value := range machine.Globals {
			if name != "original" {
				machine.Globals[name] = cloneValuePreserveRefs(value)
			}
		}
		executeAliasEscapeProgram(t, machine, mutation)
		original, expected := machine.Globals["original"], machine.Globals["expected"]
		requireAliasEscaped(t, machine, original)
		// Check backing storage before Apex reads have a chance to refresh it.
		for name, alias := range retained() {
			if alias.Ref != original.Ref || !alias.Fields["Name"].Equal(expected) {
				t.Errorf("%s: %s Ref=%d Name=%v, want Ref=%d Name=%v", mutation, name,
					alias.Ref, alias.Fields["Name"], original.Ref, expected)
			}
		}
		executeAliasEscapeProgram(t, machine, "System.assertEquals(expected, original.Name);\n"+checks)
	}
}
