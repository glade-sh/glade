package vm

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestAliasSObjectPruneEscapedCollections(t *testing.T) {
	for _, kind := range []ValueKind{ValueMap, ValueSet} {
		for _, shape := range []string{"static", "instance", "argument"} {
			t.Run(string(kind)+"/"+shape, func(t *testing.T) {
				collectionType := "Map<Integer,String>"
				if kind == ValueSet {
					collectionType = "Set<String>"
				}
				target := "AliasSObjectPruneState.Shared"
				if shape == "instance" {
					target = "holder.Items"
				}
				if shape == "argument" {
					target = "new " + collectionType + "()"
				}
				machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
List<Account> rows = new List<Account>();
for (Integer i = 0; i < 6; i++) {
    rows.add(new Account(Name = 'row-' + i, NumberOfEmployees = i));
}
AliasSObjectPruneState holder = new AliasSObjectPruneState();
%[1]s target = %[2]s;
AliasSObjectPruneState.Shared = target;
holder.Items = target;
List<Object> objects = new List<Object>{target};
Map<String,Object> values = new Map<String,Object>{'target' => target};
`, collectionType, target), aliasSObjectPruneClass(t, kind))
				if machine.localOnlyCollectionRefs[machine.Globals["target"].Ref] {
					t.Fatal("published collection is still local-only")
				}
				for i := range 6 {
					aliasSObjectPruneDetach(machine)
					receiver := "AliasSObjectPruneState.Shared"
					if shape == "instance" {
						receiver = "holder.Items"
					}
					mutation := fmt.Sprintf("%s.put(%d, rows[%d].Name);", receiver, i, i)
					if kind == ValueSet {
						mutation = fmt.Sprintf("%s.add(rows[%d].Name);", receiver, i)
					}
					if shape == "argument" {
						mutation = fmt.Sprintf("AliasSObjectPruneState.putArg(target, %d, rows[%d].Name);", i, i)
					}
					aliasSObjectPruneExecute(t, machine, mutation)
					aliasSObjectPruneCheckCollections(t, machine, kind, i+1, false)
				}
				aliasSObjectPruneDetach(machine)
				remove := "AliasSObjectPruneState.Shared.remove(1);"
				if kind == ValueSet {
					remove = "AliasSObjectPruneState.Shared.remove('row-1');"
				}
				if shape == "instance" {
					remove = "holder.Items.remove(1);"
					if kind == ValueSet {
						remove = "holder.Items.remove('row-1');"
					}
				}
				if shape == "argument" {
					remove = "AliasSObjectPruneState.removeArg(target);"
				}
				aliasSObjectPruneExecute(t, machine, remove)
				aliasSObjectPruneCheckCollections(t, machine, kind, 6, true)
				aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
System.assertEquals(5, target.size(), 'local');
System.assertEquals(5, AliasSObjectPruneState.Shared.size(), 'static');
System.assertEquals(5, holder.Items.size(), 'field');
System.assertEquals(5, ((%[1]s)objects[0]).size(), 'List<Object>');
System.assertEquals(5, ((%[1]s)values.get('target')).size(), 'Map<String,Object>');
`, collectionType))
			})
		}
	}
}

func TestAliasSObjectPrunePreservesChildListTargets(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, `
List<Contact> children = new List<Contact>{new Contact(LastName = 'before')};
Account account = new Account(Name = 'parent');
`)
	// Child relationships are read-only through put; inject a query-result graph.
	aliasSObjectPruneInjectField(machine, "account", "Contacts", machine.Globals["children"])
	aliasSObjectPruneExecute(t, machine, `
List<Account> records = new List<Account>{account};
List<Object> objects = new List<Object>{children};
`)
	for _, stage := range []struct {
		source string
		names  []string
	}{
		{"children.add(new Contact(LastName = 'after'));", []string{"before", "after"}},
		{"children.remove(0);", []string{"after"}},
	} {
		for name, value := range machine.Globals {
			if name != "children" {
				machine.Globals[name] = cloneValuePreserveRefs(value)
			}
		}
		aliasSObjectPruneExecute(t, machine, stage.source)
		for name, alias := range map[string]Value{
			"local":         machine.Globals["children"],
			"record":        machine.Globals["account"].Fields["Contacts"],
			"List<Account>": machine.Globals["records"].List[0].Fields["Contacts"],
			"List<Object>":  machine.Globals["objects"].List[0],
		} {
			if alias.Ref != machine.Globals["children"].Ref || len(alias.List) != len(stage.names) {
				t.Errorf("%s: %s Ref=%d size=%d, want Ref=%d size=%d", stage.source, name, alias.Ref, len(alias.List), machine.Globals["children"].Ref, len(stage.names))
				continue
			}
			for i, want := range stage.names {
				if !alias.List[i].Fields["LastName"].Equal(String(want)) {
					t.Errorf("%s: %s[%d].LastName=%v, want %s", stage.source, name, i, alias.List[i].Fields["LastName"], want)
				}
			}
		}
		if !machine.Globals["account"].Fields["Name"].Equal(String("parent")) {
			t.Fatal("parent scalar field changed")
		}
	}
}

func TestAliasSObjectPruneRetainsInvalidCollectionFields(t *testing.T) {
	// Restored/internal graphs can contain values that public writers reject
	// (S001/S002). Pruning must still preserve their aliases.
	for _, kind := range []ValueKind{ValueMap, ValueSet} {
		t.Run(string(kind), func(t *testing.T) {
			collectionType, mutation := "Map<String,String>", "target.put('key', 'after');"
			if kind == ValueSet {
				collectionType, mutation = "Set<String>", "target.add('after');"
			}
			machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
%[1]s target = new %[1]s();
Account record = new Account();
`, collectionType))
			aliasSObjectPruneInjectField(machine, "record", "Name", machine.Globals["target"])
			aliasSObjectPruneExecute(t, machine, `
List<Account> records = new List<Account>{record};
Map<Id,Account> byId = new Map<Id,Account>{'001000000000001AAA' => record};
`)
			for name, value := range machine.Globals {
				if name != "target" {
					machine.Globals[name] = cloneValuePreserveRefs(value)
				}
			}
			aliasSObjectPruneExecute(t, machine, mutation)
			aliases := map[string]Value{
				"record":        machine.Globals["record"].Fields["Name"],
				"List<Account>": machine.Globals["records"].List[0].Fields["Name"],
			}
			for _, record := range machine.Globals["byId"].Map {
				aliases["Map<Id,Account>"] = record.Fields["Name"]
			}
			if len(aliases) != 3 {
				t.Fatal("typed map fixture lost its record")
			}
			for name, alias := range aliases {
				if alias.Kind != kind || alias.Ref != machine.Globals["target"].Ref {
					t.Errorf("%s lost collection identity: kind=%s Ref=%d", name, alias.Kind, alias.Ref)
				}
				if kind == ValueMap {
					if len(alias.Map) != 1 || !alias.Map[mapKey(String("key"))].Equal(String("after")) {
						t.Errorf("%s lost map mutation: %#v", name, alias.Map)
					}
				} else if len(alias.Set) != 1 || !alias.Set[0].Equal(String("after")) {
					t.Errorf("%s lost set mutation: %#v", name, alias.Set)
				}
			}
		})
	}
}

func TestAliasSObjectPruneAssignmentRejectsNonFieldValues(t *testing.T) {
	// Object-typed runtime assignment follows the S001-S005 put oracle.
	for _, fixture := range []struct{ expression, valueType string }{
		{"new Map<String,String>()", "Map<String,String>"},
		{"new Set<String>()", "Set<String>"},
		{"new List<String>()", "List<String>"},
		{"new Account(Name = 'nested')", "Account"},
		{"new AliasSObjectPruneValue()", "AliasSObjectPruneValue"},
	} {
		t.Run(fixture.valueType, func(t *testing.T) {
			runDynamicObjectAliasProgram(t, fmt.Sprintf(`
Account record = new Account(Name = 'before');
Object value = %s;
String exceptionType;
String message;
try {
    record.Name = value;
} catch (SObjectException e) {
    exceptionType = e.getTypeName();
    message = e.getMessage();
}
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Illegal assignment from %s to String', message);
System.assertEquals('before', record.Name);
`, fixture.expression, fixture.valueType), Class{Name: "AliasSObjectPruneValue"})
		})
	}
}

func TestAliasSObjectPruneInternallyPublishedObjects(t *testing.T) {
	for _, nestedClass := range []bool{false, true} {
		t.Run(fmt.Sprintf("class=%t", nestedClass), func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, `
Map<String,String> target = new Map<String,String>();
AliasSObjectPruneValue box = new AliasSObjectPruneValue();
box.value = target;
Account record = new Account();
`, Class{Name: "AliasSObjectPruneValue", Fields: map[string]Field{"value": {Name: "value", Type: "Object"}}})
			value := machine.Globals["target"]
			if nestedClass {
				value = machine.Globals["box"]
			}
			// S001/S005 prohibit publication through scalar-field writers.
			aliasSObjectPruneInjectField(machine, "record", "Name", value)
			aliasSObjectPruneExecute(t, machine, "List<Account> records = new List<Account>{record};")
			if !machine.sObjectCollectionAliasRefs[machine.Globals["record"].Ref] {
				t.Fatal("internal injection did not register the exceptional record")
			}
			for name, value := range machine.Globals {
				if name != "target" {
					machine.Globals[name] = cloneValuePreserveRefs(value)
				}
			}
			aliasSObjectPruneExecute(t, machine, "target.put('key', 'after');")
			for name, record := range map[string]Value{
				"record": machine.Globals["record"], "typed list": machine.Globals["records"].List[0],
			} {
				alias := record.Fields["Name"]
				if nestedClass {
					alias = alias.Fields["value"]
				}
				aliasSObjectPruneCheckExceptionalAlias(t, name, alias, machine.Globals["target"])
			}
		})
	}
}

func TestAliasSObjectPruneUnsafeAncestors(t *testing.T) {
	for _, path := range []string{"parent", "list"} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			t.Run(path+"/"+string(kind), func(t *testing.T) {
				collectionType, mutation := "Map<String,String>", "target.put('key', 'after');"
				if kind == ValueSet {
					collectionType, mutation = "Set<String>", "target.add('after');"
				}
				machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
%[1]s target = new %[1]s();
Account record = new Account();
Account child = new Account();
List<Object> children = new List<Object>{target};
`, collectionType))
				if path == "parent" {
					aliasSObjectPruneInjectField(machine, "child", "Name", machine.Globals["target"])
					aliasSObjectPruneExecute(t, machine, "record.putSObject('Parent', child);")
				} else {
					aliasSObjectPruneInjectField(machine, "record", "Name", machine.Globals["children"])
				}
				aliasSObjectPruneExecute(t, machine, "List<Account> records = new List<Account>{record};")
				for name, value := range machine.Globals {
					if name != "target" {
						machine.Globals[name] = cloneValuePreserveRefs(value)
					}
				}
				aliasSObjectPruneExecute(t, machine, mutation)
				for name, record := range map[string]Value{
					"record": machine.Globals["record"], "typed list": machine.Globals["records"].List[0],
				} {
					alias := record.Fields["Parent"].Fields["Name"]
					if path == "list" {
						alias = record.Fields["Name"].List[0]
					}
					aliasSObjectPruneCheckExceptionalAlias(t, name, alias, machine.Globals["target"])
				}
			})
		}
	}
}

func TestAliasSObjectPruneUnsafeClones(t *testing.T) {
	for _, deep := range []bool{false, true} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			t.Run(fmt.Sprintf("deep=%t/%s", deep, kind), func(t *testing.T) {
				collectionType, mutation := "Map<String,String>", "target.put('key', 'after');"
				if kind == ValueSet {
					collectionType, mutation = "Set<String>", "target.add('after');"
				}
				machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
%[1]s original = new %[1]s();
Account source = new Account();
`, collectionType))
				aliasSObjectPruneInjectField(machine, "source", "Name", machine.Globals["original"])
				aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
Account copied = source.clone(false, %[2]t, false, false);
%[1]s target = (%[1]s)copied.get('Name');
List<Account> records = new List<Account>{copied};
`, collectionType, deep))
				source, copied := machine.Globals["source"], machine.Globals["copied"]
				if source.Ref == copied.Ref || !machine.sObjectCollectionAliasRefs[source.Ref] || !machine.sObjectCollectionAliasRefs[copied.Ref] {
					t.Fatal("clone must have a distinct, registered exceptional record Ref")
				}
				if (machine.Globals["original"].Ref == machine.Globals["target"].Ref) == deep {
					t.Fatal("clone collection identity does not match clone depth")
				}
				for name, value := range machine.Globals {
					if name != "target" {
						machine.Globals[name] = cloneValuePreserveRefs(value)
					}
				}
				aliasSObjectPruneExecute(t, machine, mutation)
				aliasSObjectPruneCheckExceptionalAlias(t, "clone", machine.Globals["copied"].Fields["Name"], machine.Globals["target"])
				aliasSObjectPruneCheckExceptionalAlias(t, "typed list clone", machine.Globals["records"].List[0].Fields["Name"], machine.Globals["target"])
			})
		}
	}
}

func TestAliasSObjectPruneDottedJSONParent(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	storage.EnsureStandardObject(machine.Org, "Contact")
	aliasSObjectPruneExecute(t, machine, `
Contact contact = (Contact)JSON.deserialize('{"Account.Name":"safe"}', Contact.class);
Account account = contact.Account;
Map<String,Object> target = new Map<String,Object>();
`)
	aliasSObjectPruneInjectField(machine, "account", "Unknown", machine.Globals["target"])
	aliasSObjectPruneInjectField(machine, "contact", "Account", machine.Globals["account"])
	aliasSObjectPruneExecute(t, machine, "List<Contact> records = new List<Contact>{contact};")
	for name, value := range machine.Globals {
		if name != "target" {
			machine.Globals[name] = cloneValuePreserveRefs(value)
		}
	}
	aliasSObjectPruneExecute(t, machine, "target.put('key', 'after');")
	for name, alias := range map[string]Value{
		"parent":            machine.Globals["account"].Fields["Unknown"],
		"child parent":      machine.Globals["contact"].Fields["Account"].Fields["Unknown"],
		"typed list parent": machine.Globals["records"].List[0].Fields["Account"].Fields["Unknown"],
	} {
		aliasSObjectPruneCheckExceptionalAlias(t, name, alias, machine.Globals["target"])
	}
}

// These tests preserve Glade's current mutable snapshot behavior. Native
// getPopulatedFieldsAsMap mutation behavior still needs a native capture.
func TestAliasSObjectPrunePopulatedFieldsMap(t *testing.T) {
	for _, shape := range []string{"static", "instance", "argument"} {
		t.Run(shape, func(t *testing.T) {
			class := Class{
				Name:         "AliasSObjectPopulatedState",
				Fields:       map[string]Field{"Items": {Name: "Items", Type: "Map<String,Object>"}},
				StaticFields: map[string]Field{"Shared": {Name: "Shared", Type: "Map<String,Object>", Static: true}},
				Methods:      make(map[string]Method),
			}
			for _, method := range []struct{ name, source string }{
				{"putArg", "target.put('Name', 'map-only'); target.put('NumberOfEmployees', 999);"},
				{"removeArg", "target.remove('Name');"},
			} {
				program, err := CompileAnonymous(method.source)
				if err != nil {
					t.Fatal(err)
				}
				class.Methods[method.name] = Method{
					Name: class.Name + "." + method.name, ClassName: class.Name,
					ReturnType: "void", IsStatic: true, Program: program,
					Params: []Param{{Name: "target", Type: "Map<String,Object>"}},
				}
			}
			machine, _ := runDynamicObjectAliasProgram(t, `
Account record = new Account(Name = 'before');
Account recordAlias = record;
List<Account> records = new List<Account>{record};
Map<String,Account> recordValues = new Map<String,Account>{'record' => record};
Map<String,Object> target = record.getPopulatedFieldsAsMap();
AliasSObjectPopulatedState holder = new AliasSObjectPopulatedState();
AliasSObjectPopulatedState.Shared = target;
holder.Items = target;
List<Object> objects = new List<Object>{target};
Map<String,Object> values = new Map<String,Object>{'target' => target};
`, class)
			detach := func() {
				for name, value := range machine.Globals {
					if name != "target" {
						machine.Globals[name] = cloneValuePreserveRefs(value)
					}
				}
				state := machine.Classes[class.Name]
				field := state.StaticFields["Shared"]
				field.Value = cloneValuePreserveRefs(field.Value)
				state.StaticFields["Shared"] = field
			}
			check := func(want map[string]Value) {
				t.Helper()
				// Inspect detached storage before any Apex read can repair aliases.
				for name, alias := range map[string]Value{
					"local":     machine.Globals["target"],
					"static":    machine.Classes[class.Name].StaticFields["Shared"].Value,
					"field":     machine.Globals["holder"].Fields["Items"],
					"list":      machine.Globals["objects"].List[0],
					"map value": machine.Globals["values"].Map[mapKey(String("target"))],
				} {
					if alias.Kind != ValueMap || alias.Ref != machine.Globals["target"].Ref || len(alias.Map) != len(want) {
						t.Errorf("%s: populated map identity/size changed: %#v", name, alias)
					}
					for key, value := range want {
						if got, ok := alias.Map[mapKey(String(key))]; !ok || !got.Equal(value) {
							t.Errorf("%s[%s]=%v, present=%v, want %v", name, key, got, ok, value)
						}
					}
				}
				for name, record := range map[string]Value{
					"record":       machine.Globals["record"],
					"record alias": machine.Globals["recordAlias"],
					"record list":  machine.Globals["records"].List[0],
					"record map":   machine.Globals["recordValues"].Map[mapKey(String("record"))],
				} {
					if !record.Fields["Name"].Equal(String("later")) || !record.Fields["NumberOfEmployees"].Equal(Int(23)) {
						t.Errorf("%s changed with populated map: %#v", name, record.Fields)
					}
				}
			}
			detach()
			aliasSObjectPruneExecute(t, machine, "record.Name = 'later'; record.NumberOfEmployees = 23;")
			check(map[string]Value{"Name": String("before")})
			// Current Glade accepts both mutations; the returned map is independent
			// of the source record, but its own aliases retain reference semantics.
			receiver := class.Name + ".Shared"
			if shape == "instance" {
				receiver = "holder.Items"
			}
			put := receiver + ".put('Name', 'map-only'); " + receiver + ".put('NumberOfEmployees', 999);"
			remove := receiver + ".remove('Name');"
			if shape == "argument" {
				put = class.Name + ".putArg(target);"
				remove = class.Name + ".removeArg(target);"
			}
			detach()
			aliasSObjectPruneExecute(t, machine, put)
			check(map[string]Value{"Name": String("map-only"), "NumberOfEmployees": Int(999)})
			detach()
			aliasSObjectPruneExecute(t, machine, remove)
			check(map[string]Value{"NumberOfEmployees": Int(999)})
		})
	}
}

func TestAliasSObjectPruneRuntimeCloneRegistry(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(fmt.Sprintf("frozen=%t", frozen), func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, `
Map<String,String> target = new Map<String,String>();
Account record = new Account();
`)
			aliasSObjectPruneInjectField(machine, "record", "Name", machine.Globals["target"])
			aliasSObjectPruneExecute(t, machine, "List<Account> records = new List<Account>{record};")
			clone := machine.CloneRuntime(nil)
			if frozen {
				machine.FreezeClassLookup()
				clone = machine.CloneRuntimeFrozenShared(nil)
			}
			clone.SetOrg(machine.Org)
			for name, value := range machine.Globals {
				clone.Globals[name] = cloneValuePreserveRefs(value)
			}
			if !clone.sObjectCollectionAliasRefs[machine.Globals["record"].Ref] {
				t.Fatal("runtime clone lost exceptional record registry")
			}
			aliasSObjectPruneExecute(t, clone, "target.put('key', 'after');")
			aliasSObjectPruneCheckExceptionalAlias(t, "cloned runtime record", clone.Globals["record"].Fields["Name"], clone.Globals["target"])
			aliasSObjectPruneCheckExceptionalAlias(t, "cloned runtime typed list", clone.Globals["records"].List[0].Fields["Name"], clone.Globals["target"])
			aliasSObjectPruneExecute(t, clone, `
Account cloneOnly = new Account();
Set<String> cloneTarget = new Set<String>{'clone'};
`)
			aliasSObjectPruneInjectField(clone, "cloneOnly", "Name", clone.Globals["cloneTarget"])
			ref := clone.Globals["cloneOnly"].Ref
			if !clone.sObjectCollectionAliasRefs[ref] || machine.sObjectCollectionAliasRefs[ref] {
				t.Fatal("runtime clone shared its writable exceptional registry with source")
			}
		})
	}
}

func TestAliasSObjectPruneStaticMapVisits(t *testing.T) {
	visits := make(map[int]uint64)
	for _, size := range []int{200, 800} {
		machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
List<Account> rows = new List<Account>();
for (Integer i = 0; i < %d; i++) {
    rows.add(new Account(Name = 'row-' + i, NumberOfEmployees = i));
}
Integer initializedSize = AliasSObjectPruneState.Shared.size();
List<Object> aliasHolder = new List<Object>{AliasSObjectPruneState.Shared};
`, size), aliasSObjectPruneClass(t, ValueMap))
		if machine.localOnlyCollectionRefs[machine.Classes["AliasSObjectPruneState"].StaticFields["Shared"].Value.Ref] {
			t.Fatal("static initializer did not publish the map")
		}
		recorder := NewPerfRecorder()
		machine.SetPerfRecorder(recorder)
		aliasSObjectPruneExecute(t, machine, `
for (Integer i = 0; i < rows.size(); i++) {
    AliasSObjectPruneState.Shared.put(i, rows[i].Name);
}
`)
		stats := recorder.Snapshot().ScopeAlias
		visits[size] = stats.RecursiveVisits
		if stats.Calls == 0 || stats.Roots == 0 {
			t.Fatalf("n=%d: escaped-map fill did not run scope propagation: %#v", size, stats)
		}
		if got := len(machine.Classes["AliasSObjectPruneState"].StaticFields["Shared"].Value.Map); got != size {
			t.Fatalf("n=%d: map size=%d", size, got)
		}
		// Allow 32 constant-size scope/root visits per put and 64 for setup.
		// A single record-list traversal per put costs n*n and exceeds this
		// bound at both sizes. Record construction is outside the recorder.
		limit := uint64(32*size + 64)
		t.Logf("n=%d: RecursiveVisits=%d, limit=%d", size, visits[size], limit)
		if visits[size] == 0 || visits[size] > limit {
			t.Errorf("n=%d: RecursiveVisits=%d, want 1..%d", size, visits[size], limit)
		}
	}
	if visits[800] >= 5*visits[200] {
		t.Errorf("RecursiveVisits grew from %d to %d; want 800 records below 5x of 200", visits[200], visits[800])
	}
}

func TestAliasSObjectPruneTypeVerdicts(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Opportunity")
	machine.SetOrg(&org)
	account := Object("Account")
	account.Fields["Name"] = String("safe")
	generic := Object("SObject")
	generic.Fields["Name"] = String("generic")
	unknown := Object("MissingRecordType")
	unknown.Fields["Name"] = String("unknown")
	for _, test := range []struct {
		name string
		root Value
		want bool
	}{
		{"known record", account, true},
		{"generic record", generic, true},
		{"unknown record", unknown, false},
		{"known list", testTypedList("List<Account>", account), true},
		{"generic list", testTypedList("List<SObject>", generic), true},
		{"array", testTypedList("Account[]", account), true},
		{"qualified list", testTypedList("System.List<Account>", account), true},
		{"unknown list", testTypedList("List<MissingRecordType>", unknown), false},
		{"object list", testTypedList("List<Object>", account), false},
	} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			t.Run(test.name+"/"+string(kind), func(t *testing.T) {
				for range 2 {
					if got := machine.valueCannotContainAliasRef(test.root, newValueRef(), kind); got != test.want {
						t.Fatalf("pruned=%v, want %v", got, test.want)
					}
				}
				if _, ok := machine.sObjectAliasTypes[test.root.Type]; !ok {
					t.Fatalf("exact type %q was not memoized", test.root.Type)
				}
			})
		}
	}
	for _, typeName := range []string{
		"Map<Id,Account>", "Map<Account,String>", "Map<Account,SObject>",
		"Map<String,Boolean>", "Set<Opportunity>", "List<List<SObject>>",
	} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			if !machine.sObjectCollectionContentsCannotContain(Value{Type: typeName}, kind) {
				t.Errorf("%s contents were not pruned for %s", typeName, kind)
			}
		}
	}
	for _, typeName := range []string{"Map<String,Object>", "Map<Object,Account>", "List<Object>", "List<MissingRecordType>", "Map<String,>"} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			if machine.sObjectCollectionContentsCannotContain(Value{Type: typeName}, kind) {
				t.Errorf("%s contents were pruned for %s", typeName, kind)
			}
		}
	}
	for _, root := range []Value{account, testTypedList("List<Account>", account)} {
		for _, kind := range []ValueKind{ValueMap, ValueSet} {
			if valueCannotContainAliasRef(root, newValueRef(), kind) || (*VM)(nil).valueCannotContainAliasRef(root, newValueRef(), kind) {
				t.Errorf("package/no-VM path unexpectedly used SObject knowledge for %s", root.Type)
			}
		}
		if machine.valueCannotContainAliasRef(root, newValueRef(), ValueList) {
			t.Errorf("List target was pruned for %s", root.Type)
		}
	}
	for _, kind := range []ValueKind{ValueMap, ValueSet} {
		root := Map()
		root.Type = "Map<Id,Account>"
		if kind == ValueSet {
			root = Set(account)
			root.Type = "Set<Account>"
		}
		previous := snapshotAlias(root)
		if machine.valueCannotContainAliasRef(root, root.Ref, kind) {
			t.Fatalf("direct %s identity was pruned", kind)
		}
		if !machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) ||
			!machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &scopeAliasProbe{}) ||
			!machine.valueContainsAliasRef(root, root.Ref, kind, make(map[uint64]bool)) ||
			!valueContainsAliasRef(root, root.Ref, kind, make(map[uint64]bool)) {
			t.Errorf("a containment walk missed the direct %s identity", kind)
		}
		if _, changed := replaceValueAliasRefWithCache(machine, root, previous, cloneValuePreserveRefs(root), make(map[uint64]bool)); !changed {
			t.Errorf("replacement walk missed the direct %s identity", kind)
		}
	}
}

func TestAliasSObjectPruneExceptionalRegistryIsConservative(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	scalar := Object("Account")
	scalar.Fields["Name"] = String("safe")
	parent := Object("Account")
	parent.Fields["Parent"] = scalar
	parentList := Object("Account")
	parentList.Fields["Children"] = testTypedList("List<Account>", scalar)
	exceptional := Object("Account")
	machine.registerSObjectAliasField(exceptional, "Name", Map())
	if machine.sObjectCollectionAliasRefs[parent.Ref] || machine.sObjectCollectionAliasRefs[parentList.Ref] {
		t.Fatal("fixture unexpectedly registered unrelated ancestors")
	}
	for _, kind := range []ValueKind{ValueMap, ValueSet} {
		if !machine.valueCannotContainAliasRef(scalar, newValueRef(), kind) {
			t.Errorf("unrelated scalar-only record was not pruned for %s", kind)
		}
		for _, root := range []Value{parent, parentList, testTypedList("List<Account>", scalar)} {
			if machine.valueCannotContainAliasRef(root, newValueRef(), kind) {
				t.Errorf("relationship-bearing record or typed collection was pruned with nonempty registry: %s", root.Type)
			}
		}
	}
}

func TestAliasSObjectPruneScalarRegistration(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	record := Object("Account")
	for _, scalar := range []Value{Null, String("name"), Int(1), Bool(true)} {
		machine.registerSObjectAliasField(record, "Name", scalar)
		machine.setExplicitSObjectFieldValue(&record, "Name", scalar)
	}
	if machine.sObjectCollectionAliasRefs != nil || machine.sObjectAliasTypes != nil {
		t.Fatalf("scalar writes touched registry/type memo: registry=%v memo=%v", machine.sObjectCollectionAliasRefs, machine.sObjectAliasTypes)
	}
}

func BenchmarkAliasSObjectPruneScalarRegistration(b *testing.B) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	record, scalar := Object("Account"), String("name")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine.registerSObjectAliasField(record, "Name", scalar)
	}
	b.StopTimer()
	if machine.sObjectCollectionAliasRefs != nil || machine.sObjectAliasTypes != nil {
		b.Fatal("scalar registration touched registry/type memo")
	}
}

func TestAliasSObjectPruneSetOrgInvalidatesTypeVerdicts(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	record := Object("AliasRecord__c")
	record.Fields["Name"] = String("row")
	if machine.valueCannotContainAliasRef(record, newValueRef(), ValueMap) {
		t.Fatal("unresolved record type was pruned")
	}
	org.Objects[record.Type] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: record.Type}}
	machine.SetOrg(&org)
	if !machine.valueCannotContainAliasRef(record, newValueRef(), ValueMap) {
		t.Fatal("SetOrg retained the prior unresolved verdict")
	}
	delete(org.Objects, record.Type)
	machine.SetOrg(&org)
	if machine.valueCannotContainAliasRef(record, newValueRef(), ValueMap) {
		t.Fatal("SetOrg retained the prior resolved verdict")
	}
}

func TestAliasSObjectPruneRestoredInstanceProperty(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	target := Map()
	target.Type = "Map<String,String>"
	record := Object("Account")
	record.Fields["Name"] = target
	controller := Object("RestoredController")
	controller.Fields["rows"] = testTypedList("List<Account>", record)
	rows, ok, err := machine.ReadInstanceProperty(controller, "rows")
	if err != nil || !ok || len(rows.List) != 1 {
		t.Fatalf("restored rows: ok=%v err=%v value=%#v", ok, err, rows)
	}
	if !machine.sObjectCollectionAliasRefs[record.Ref] {
		t.Fatal("restored property graph was not registered")
	}
	machine.Globals["rows"] = cloneValuePreserveRefs(rows)
	machine.Globals["target"] = target
	aliasSObjectPruneExecute(t, machine, "target.put('key', 'after');")
	aliasSObjectPruneCheckExceptionalAlias(t, "restored graph", machine.Globals["rows"].List[0].Fields["Name"], machine.Globals["target"])
}

func aliasSObjectPruneClass(t *testing.T, kind ValueKind) Class {
	t.Helper()
	collectionType, put, remove := "Map<Integer,String>", "target.put(key, item);", "target.remove(1);"
	if kind == ValueSet {
		collectionType, put, remove = "Set<String>", "target.add(item);", "target.remove('row-1');"
	}
	class := Class{
		Name:         "AliasSObjectPruneState",
		Fields:       map[string]Field{"Items": {Name: "Items", Type: collectionType}},
		StaticFields: map[string]Field{"Shared": {Name: "Shared", Type: collectionType, Static: true}},
		Methods:      make(map[string]Method),
	}
	for _, fixture := range []struct{ name, source string }{
		{"<static_init>", "AliasSObjectPruneState.Shared = new " + collectionType + "();"},
		{"<init>", "this.Items = new " + collectionType + "();"},
		{"putArg", put},
		{"removeArg", remove},
	} {
		program, err := CompileAnonymous(fixture.source)
		if err != nil {
			t.Fatal(err)
		}
		method := Method{Name: class.Name + "." + fixture.name, ClassName: class.Name, ReturnType: "void", IsStatic: true, Program: program}
		switch fixture.name {
		case "<static_init>":
			class.StaticInitializers = []Method{method}
		case "<init>":
			method.IsStatic, method.IsConstructor = false, true
			class.Constructors = []Method{method}
		default:
			method.Params = []Param{{Name: "target", Type: collectionType}}
			if fixture.name == "putArg" {
				method.Params = append(method.Params, Param{Name: "key", Type: "Integer"}, Param{Name: "item", Type: "String"})
			}
			class.Methods[fixture.name] = method
		}
	}
	return class
}

func aliasSObjectPruneDetach(machine *VM) {
	for name, value := range machine.Globals {
		if name != "target" {
			machine.Globals[name] = cloneValuePreserveRefs(value)
		}
	}
	class := machine.Classes["AliasSObjectPruneState"]
	field := class.StaticFields["Shared"]
	field.Value = cloneValuePreserveRefs(field.Value)
	class.StaticFields["Shared"] = field
}

func aliasSObjectPruneCheckCollections(t *testing.T, machine *VM, kind ValueKind, filled int, removed bool) {
	t.Helper()
	for name, alias := range map[string]Value{
		"local":     machine.Globals["target"],
		"static":    machine.Classes["AliasSObjectPruneState"].StaticFields["Shared"].Value,
		"field":     machine.Globals["holder"].Fields["Items"],
		"list":      machine.Globals["objects"].List[0],
		"map value": machine.Globals["values"].Map[mapKey(String("target"))],
	} {
		if alias.Kind != kind || alias.Ref != machine.Globals["target"].Ref {
			t.Errorf("%s lost collection identity: kind=%s Ref=%d", name, alias.Kind, alias.Ref)
		}
		wantSize := filled
		if removed {
			wantSize--
		}
		if (kind == ValueMap && len(alias.Map) != wantSize) || (kind == ValueSet && len(alias.Set) != wantSize) {
			t.Errorf("%s: map size=%d set size=%d, want %s size=%d", name, len(alias.Map), len(alias.Set), kind, wantSize)
		}
		for i := range 6 {
			want := String("row-" + strconv.Itoa(i))
			present := false
			if kind == ValueMap {
				got, found := alias.Map[mapKey(Int(int64(i)))]
				present = found
				if found && !got.Equal(want) {
					t.Errorf("%s[%d]=%v, want %v", name, i, got, want)
				}
			} else {
				for _, got := range alias.Set {
					present = present || got.Equal(want)
				}
			}
			if wantPresent := i < filled && (!removed || i != 1); present != wantPresent {
				t.Errorf("%s contains %d=%v, want %v", name, i, present, wantPresent)
			}
		}
	}
	for i, record := range machine.Globals["rows"].List {
		if !record.Fields["Name"].Equal(String("row-"+strconv.Itoa(i))) || !record.Fields["NumberOfEmployees"].Equal(Int(int64(i))) {
			t.Errorf("record %d changed: %#v", i, record.Fields)
		}
	}
}

func aliasSObjectPruneCheckExceptionalAlias(t *testing.T, name string, alias, target Value) {
	t.Helper()
	if alias.Kind != target.Kind || alias.Ref != target.Ref {
		t.Errorf("%s lost collection identity: kind=%s Ref=%d, want kind=%s Ref=%d", name, alias.Kind, alias.Ref, target.Kind, target.Ref)
	}
	if target.Kind == ValueMap {
		if len(alias.Map) != 1 || !alias.Map[mapKey(String("key"))].Equal(String("after")) {
			t.Errorf("%s lost map mutation: %#v", name, alias.Map)
		}
	} else if len(alias.Set) != 1 || !alias.Set[0].Equal(String("after")) {
		t.Errorf("%s lost set mutation: %#v", name, alias.Set)
	}
}

func aliasSObjectPruneExecute(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("%s: %v", source, err)
	}
}

// Deliberately bypass public writer validation to model restored/internal graphs.
func aliasSObjectPruneInjectField(machine *VM, global, field string, value Value) {
	record := machine.Globals[global]
	machine.setExplicitSObjectFieldValue(&record, field, value)
	machine.Globals[global] = record
}
