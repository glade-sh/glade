package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDynamicObjectLocalOnlyAlias(t *testing.T) {
	factories := []struct {
		name  string
		setup string
		expr  string
	}{
		{"newSObject", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject(null, false)"},
		{"newSObjectNoArguments", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject()"},
		{"newSObjectNullId", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject(null)"},
		{"newSObjectWithId", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject('001000000000001AAA')"},
		{"newSObjectWithDefaults", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject(null, true)"},
		{"newSObjectRecordTypeWithoutDefaults", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject('012000000000002AAA', false)"},
		{"newSObjectRecordTypeWithDefaults", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account');", "accountType.newSObject('012000000000002AAA', true)"},
		{"describeNewSObject", "Schema.SObjectType accountType = Schema.getGlobalDescribe().get('Account').getDescribe().getSObjectType();", "accountType.newSObject(null, false)"},
		{"clone", "SObject template = new Account();", "template.clone(false, false, false, false)"},
		{"cloneNoArguments", "SObject template = new Account();", "template.clone()"},
		{"cloneDeep", "SObject template = new Account();", "template.clone(false, true, false, false)"},
		{"clonePreserveId", "SObject template = new Account(Id = '001000000000001AAA', Name = 'template');", "template.clone(true)"},
		{"cloneDeepPreserveId", "SObject template = new Account(Id = '001000000000001AAA', Name = 'template');", "template.clone(true, true)"},
		{"newInstance", "Type accountType = Type.forName('Account');", "(SObject)accountType.newInstance()"},
		{"deserialize", "", "(SObject)JSON.deserialize('{}', Account.class)"},
		{"deserializeStrict", "", "(SObject)JSON.deserializeStrict('{}', Account.class)"},
		{"deserializeGenericSObject", "", `(SObject)JSON.deserialize('{"attributes":{"type":"Account"}}', SObject.class)`},
		{"deserializeStrictGenericSObject", "", `(SObject)JSON.deserializeStrict('{"attributes":{"type":"Account"}}', SObject.class)`},
	}
	loop := func(setup, expr string, size int) string {
		return fmt.Sprintf(`
%s
List<SObject> rows = new List<SObject>();
for (Integer i = 0; i < %d; i++) {
    SObject row = %s;
    row.put('Name', 'normal');
    row.put('NumberOfEmployees', i);
    rows.add(row);
}
`, setup, size, expr)
	}
	sizes := []int{200, 800}
	expected := make(map[int]Value, len(sizes))
	for _, size := range sizes {
		machine, _ := runDynamicObjectAliasProgram(t, loop("", "new Account()", size))
		expected[size] = machine.Globals["rows"]
	}
	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			visits := make(map[int]uint64, len(sizes))
			for _, size := range sizes {
				machine, stats := runDynamicObjectAliasProgram(t, loop(factory.setup, factory.expr, size))
				t.Logf("n=%d: Calls=%d Roots=%d RecursiveVisits=%d", size, stats.Calls, stats.Roots, stats.RecursiveVisits)
				visits[size] = stats.RecursiveVisits
				got, want := machine.Globals["rows"], expected[size]
				if got.Kind != ValueList || len(got.List) != size || len(want.List) != size {
					t.Fatalf("n=%d: dynamic/direct list sizes = %d/%d, dynamic kind = %s", size, len(got.List), len(want.List), got.Kind)
				}
				for i, row := range got.List {
					for _, field := range []string{"Name", "NumberOfEmployees"} {
						if !row.Fields[field].Equal(want.List[i].Fields[field]) {
							t.Fatalf("n=%d: row %d %s = %#v, direct construction = %#v", size, i, field, row.Fields[field], want.List[i].Fields[field])
						}
					}
					if row.Fields["Name"].Text != "normal" || row.Fields["NumberOfEmployees"].Int != int64(i) {
						t.Fatalf("n=%d: row %d lost populated fields: %#v", size, i, row.Fields)
					}
				}
				if stats.Calls == 0 || stats.Roots == 0 {
					t.Fatalf("n=%d: alias recorder did not observe the loop: %#v", size, stats)
				}
			}
			// A fully local loop performs no recursive visits at either size.
			// Otherwise enforce the requested growth bound without wall-clock timing.
			if visits[200] == 0 {
				if visits[800] != 0 {
					t.Fatalf("recursive visits grew from zero at n=200 to %d at n=800", visits[800])
				}
			} else if visits[800] >= 3*visits[200] {
				t.Fatalf("recursive visits at n=800 = %d, want below 3 * %d at n=200", visits[800], visits[200])
			}

			t.Run("referenceSemantics", func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
%[1]s
SObject a = %[2]s;
SObject b = a;
Account fieldTarget;
a.put('Name', 'a-put');
System.assertEquals('a-put', b.get('Name'));
b.put('Name', 'b-put');
System.assertEquals('b-put', a.get('Name'));
fieldTarget = (Account)a;
fieldTarget.Name = 'a-field';
System.assertEquals('a-field', b.get('Name'));
fieldTarget = (Account)b;
fieldTarget.Name = 'b-field';
System.assertEquals('b-field', a.get('Name'));

SObject listRow = %[2]s;
List<SObject> listAlias = new List<SObject>();
listAlias.add(listRow);
listRow.put('Name', 'list-original');
System.assertEquals('list-original', listAlias[0].get('Name'));
listAlias[0].put('Name', 'list-element');
System.assertEquals('list-element', listRow.get('Name'));
fieldTarget = (Account)listRow;
fieldTarget.Name = 'list-original-field';
System.assertEquals('list-original-field', listAlias[0].get('Name'));
fieldTarget = (Account)listAlias[0];
fieldTarget.Name = 'list-final';
System.assertEquals('list-final', listRow.get('Name'));
System.assertEquals(1, listAlias.size());

SObject mapRow = %[2]s;
Map<String, SObject> mapAlias = new Map<String, SObject>();
mapAlias.put('row', mapRow);
mapRow.put('Name', 'map-original');
System.assertEquals('map-original', mapAlias.get('row').get('Name'));
mapAlias.get('row').put('Name', 'map-value');
System.assertEquals('map-value', mapRow.get('Name'));
fieldTarget = (Account)mapRow;
fieldTarget.Name = 'map-original-field';
System.assertEquals('map-original-field', mapAlias.get('row').get('Name'));
SObject mapValue = mapAlias.get('row');
fieldTarget = (Account)mapValue;
fieldTarget.Name = 'map-final';
System.assertEquals('map-final', mapRow.get('Name'));
System.assertEquals('map-final', mapAlias.get('row').get('Name'));
System.assertEquals(1, mapAlias.size());

SObject keyRow = %[2]s;
Map<SObject, String> keyed = new Map<SObject, String>();
keyed.put(keyRow, 'payload');
keyRow.put('Name', 'key-original');
SObject keyAlias;
for (SObject item : keyed.keySet()) {
    System.assertEquals('key-original', item.get('Name'));
    keyAlias = item;
}
keyAlias.put('Name', 'key-element');
System.assertEquals('key-element', keyRow.get('Name'));
fieldTarget = (Account)keyRow;
fieldTarget.Name = 'key-original-field';
for (SObject item : keyed.keySet()) {
    System.assertEquals('key-original-field', item.get('Name'));
}
fieldTarget = (Account)keyAlias;
fieldTarget.Name = 'key-final';
System.assertEquals('key-final', keyRow.get('Name'));
for (SObject item : keyed.keySet()) {
    System.assertEquals('key-final', item.get('Name'));
}
System.assertEquals(1, keyed.size());

SObject setRow = %[2]s;
Set<SObject> members = new Set<SObject>();
members.add(setRow);
setRow.put('Name', 'set-original');
SObject setAlias;
for (SObject item : members) {
    System.assertEquals('set-original', item.get('Name'));
    setAlias = item;
}
setAlias.put('Name', 'set-element');
System.assertEquals('set-element', setRow.get('Name'));
fieldTarget = (Account)setRow;
fieldTarget.Name = 'set-original-field';
for (SObject item : members) {
    System.assertEquals('set-original-field', item.get('Name'));
}
fieldTarget = (Account)setAlias;
fieldTarget.Name = 'set-final';
System.assertEquals('set-final', setRow.get('Name'));
for (SObject item : members) {
    System.assertEquals('set-final', item.get('Name'));
}
System.assertEquals(1, members.size());

SObject fieldRow = %[2]s;
Account holder = new Account();
holder.putSObject('Parent', fieldRow);
fieldRow.put('Name', 'field-original');
System.assertEquals('field-original', holder.getSObject('Parent').get('Name'));
holder.getSObject('Parent').put('Name', 'field-child');
System.assertEquals('field-child', fieldRow.get('Name'));
fieldTarget = (Account)fieldRow;
fieldTarget.Name = 'field-original-field';
System.assertEquals('field-original-field', holder.getSObject('Parent').get('Name'));
SObject fieldAlias = holder.getSObject('Parent');
fieldTarget = (Account)fieldAlias;
fieldTarget.Name = 'field-final';
System.assertEquals('field-final', fieldRow.get('Name'));
System.assertEquals('field-final', holder.getSObject('Parent').get('Name'));

SObject duplicateRow = %[2]s;
List<SObject> duplicates = new List<SObject>();
duplicates.add(duplicateRow);
duplicates.add(duplicateRow);
duplicates[0].put('Name', 'duplicate-first');
System.assertEquals('duplicate-first', duplicateRow.get('Name'));
System.assertEquals('duplicate-first', duplicates[1].get('Name'));
duplicates[1].put('Name', 'duplicate-final');
System.assertEquals('duplicate-final', duplicateRow.get('Name'));
System.assertEquals('duplicate-final', duplicates[0].get('Name'));
System.assertEquals(2, duplicates.size());

SObject movingRow = %[2]s;
List<SObject> source = new List<SObject>();
List<SObject> destination = new List<SObject>();
source.add(movingRow);
SObject removed = source.remove(0);
destination.add(removed);
System.assertEquals(0, source.size());
System.assertEquals(1, destination.size());
movingRow.put('Name', 'moving-original');
System.assertEquals('moving-original', destination[0].get('Name'));
destination[0].put('Name', 'moving-final');
System.assertEquals('moving-final', movingRow.get('Name'));
System.assertEquals('moving-final', removed.get('Name'));
`, factory.setup, factory.expr))
				globals := machine.Globals
				if template, ok := globals["template"]; ok {
					if template.Ref == globals["a"].Ref {
						t.Fatal("clone retained its source root reference")
					}
					if name, present := template.Fields["Name"]; present && name.Kind != ValueNull && !name.Equal(String("template")) {
						t.Errorf("mutating a clone changed its source Name: %#v", name)
					}
					if id, present := template.Fields["Id"]; present && id.Kind != ValueNull && !id.Equal(globals["a"].Fields["Id"]) {
						t.Error("preserveId clone lost its source Id")
					}
				}
				if len(globals["listAlias"].List) != 1 || len(globals["mapAlias"].Map) != 1 ||
					len(globals["keyed"].MapKeys) != 1 || len(globals["members"].Set) != 1 ||
					len(globals["duplicates"].List) != 2 || len(globals["source"].List) != 0 || len(globals["destination"].List) != 1 {
					t.Fatal("resulting alias containers have unexpected sizes")
				}
				var storedKey Value
				for encoded, key := range globals["keyed"].MapKeys {
					storedKey = key
					if payload := globals["keyed"].Map[encoded]; !payload.Equal(String("payload")) {
						t.Fatalf("mutating the key changed its map value: %#v", payload)
					}
				}
				for _, check := range []struct {
					root      string
					name      string
					aliases   []Value
					localOnly bool
				}{
					{"a", "b-field", []Value{globals["b"]}, true},
					{"listRow", "list-final", globals["listAlias"].List, false},
					{"mapRow", "map-final", []Value{globals["mapAlias"].Map[mapKey(String("row"))], globals["mapValue"]}, false},
					{"keyRow", "key-final", []Value{storedKey, globals["keyAlias"]}, false},
					{"setRow", "set-final", append([]Value{globals["setAlias"]}, globals["members"].Set...), false},
					{"fieldRow", "field-final", []Value{globals["holder"].Fields["Parent"], globals["fieldAlias"]}, false},
					{"duplicateRow", "duplicate-final", globals["duplicates"].List, false},
					{"movingRow", "moving-final", []Value{globals["destination"].List[0], globals["removed"]}, false},
				} {
					root := globals[check.root]
					if root.Kind != ValueObject || root.Ref == 0 {
						t.Fatalf("%s is not an object with a reference: %#v", check.root, root)
					}
					for _, alias := range append([]Value{root}, check.aliases...) {
						if alias.Kind != ValueObject || alias.Ref != root.Ref || !alias.Fields["Name"].Equal(String(check.name)) {
							t.Errorf("%s alias = %#v, want Ref %d and Name %q", check.root, alias, root.Ref, check.name)
						}
					}
					if localOnly := machine.localOnlyObjectRefs[root.Ref]; localOnly != check.localOnly {
						t.Errorf("%s local-only = %t, want %t", check.root, localOnly, check.localOnly)
					}
				}
			})
		})
	}
}

func TestJSONLocalOnlyAliasRegistersOnlyObjectRoots(t *testing.T) {
	for _, method := range []string{"deserialize", "deserializeStrict"} {
		t.Run(method, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
Account root = (Account)JSON.%[1]s('{"Name":"root","Parent":{"attributes":{"type":"Account"},"Name":"parent"}}', Account.class);
SObject genericRoot = (SObject)JSON.%[1]s('{"attributes":{"type":"Account"},"Name":"generic"}', SObject.class);
List<Account> rows = (List<Account>)JSON.%[1]s('[{"Name":"list"}]', List<Account>.class);
Map<String, Account> indexed = (Map<String, Account>)JSON.%[1]s('{"row":{"Name":"map"}}', Map<String, Account>.class);
`, method))
			for _, name := range []string{"root", "genericRoot"} {
				root := machine.Globals[name]
				if root.Kind != ValueObject || root.Ref == 0 || !machine.localOnlyObjectRefs[root.Ref] {
					t.Errorf("%s was not registered as a fresh object root: %#v", name, root)
				}
			}
			children := map[string]Value{
				"relationship": machine.Globals["root"].Fields["Parent"],
				"list":         machine.Globals["rows"].List[0],
				"map":          machine.Globals["indexed"].Map[mapKey(String("row"))],
			}
			for name, child := range children {
				if child.Kind != ValueObject || child.Ref == 0 {
					t.Fatalf("%s child is not an object: %#v", name, child)
				}
				if machine.localOnlyObjectRefs[child.Ref] {
					t.Errorf("%s child was registered despite being contained in another value", name)
				}
			}
		})
	}
}

func TestDynamicObjectLocalOnlyAliasPreservesConstructorAndSetterEscapes(t *testing.T) {
	constructor, err := CompileAnonymous("AliasRoot.Published = this;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous("AliasRoot.Published = this; this.Name = value;")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		expr        string
		constructor bool
	}{
		{"newInstanceConstructor", "Type.forName('AliasRoot').newInstance()", true},
		{"deserializeConstructor", "JSON.deserialize('{}', AliasRoot.class)", true},
		{"deserializeStrictConstructor", "JSON.deserializeStrict('{}', AliasRoot.class)", true},
		{"deserializeSetter", `JSON.deserialize('{"Publish":"initial"}', AliasRoot.class)`, false},
		{"deserializeStrictSetter", `JSON.deserializeStrict('{"Publish":"initial"}', AliasRoot.class)`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			class := Class{
				Name: "AliasRoot",
				StaticFields: map[string]Field{
					"Published": {Name: "Published", Type: "AliasRoot", Static: true},
				},
				Fields: map[string]Field{
					"Name": {Name: "Name", Type: "String"},
					"Publish": {
						Name: "Publish", Type: "String",
						Setter: &Method{Name: "AliasRoot.Publish.set", ClassName: "AliasRoot", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
					},
				},
			}
			if test.constructor {
				class.Constructors = []Method{{Name: "AliasRoot.<init>", ClassName: "AliasRoot", IsConstructor: true, Program: constructor}}
			}
			machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
AliasRoot root = (AliasRoot)%s;
System.assertNotEquals(null, AliasRoot.Published);
root.Name = 'after';
System.assertEquals('after', AliasRoot.Published.Name);
`, test.expr), class)
			root := machine.Globals["root"]
			if root.Kind != ValueObject || root.Ref == 0 {
				t.Fatalf("factory did not return an object: %#v", root)
			}
			if machine.localOnlyObjectRefs[root.Ref] {
				t.Fatal("factory re-registered an object published to a static during construction or deserialization")
			}
		})
	}
}

func TestDynamicObjectLocalOnlyAliasAggregateClone(t *testing.T) {
	const source = `
List<AggregateResult> aggregates = [SELECT COUNT(Id) total FROM Account];
AggregateResult original = aggregates[0];
AggregateResult first = (AggregateResult)original.clone();
AggregateResult second = (AggregateResult)original.clone(false, true, false, false);
AggregateResult alias = first;
System.assertEquals(0, original.get('total'));
System.assertEquals(null, first.get('Id'));
Boolean readRejected = false;
try { first.get('total'); }
catch (SObjectException e) { readRejected = e.getMessage() == 'Invalid field total for AggregateResult'; }
System.assert(readRejected);
Boolean writeRejected = false;
try { alias.put('total', 7); }
catch (SObjectException e) { writeRejected = e.getMessage() == 'Invalid field total for AggregateResult'; }
System.assert(writeRejected);
System.assertEquals(0, original.get('total'));
`
	for _, escaped := range []bool{false, true} {
		t.Run(fmt.Sprintf("escaped=%t", escaped), func(t *testing.T) {
			body := source
			if escaped {
				body += `
List<AggregateResult> listAlias = new List<AggregateResult>{first};
Map<String, AggregateResult> mapAlias = new Map<String, AggregateResult>();
mapAlias.put('clone', first);
Set<AggregateResult> setAlias = new Set<AggregateResult>();
setAlias.add(first);
System.assertEquals(1, listAlias.size());
System.assertEquals(1, mapAlias.size());
System.assertEquals(1, setAlias.size());
System.assertEquals(0, original.get('total'));
`
			}
			machine, _ := runDynamicObjectAliasProgram(t, body)
			original, first, second := machine.Globals["original"], machine.Globals["first"], machine.Globals["second"]
			if first.Kind != ValueObject || second.Kind != ValueObject || first.Ref == 0 || second.Ref == 0 || first.Ref == original.Ref || second.Ref == original.Ref || first.Ref == second.Ref {
				t.Fatalf("clone refs must be fresh: original=%d first=%d second=%d", original.Ref, first.Ref, second.Ref)
			}
			if machine.Globals["alias"].Ref != first.Ref {
				t.Fatal("local alias lost the cloned root Ref")
			}
			if machine.localOnlyObjectRefs[first.Ref] == escaped || !machine.localOnlyObjectRefs[second.Ref] {
				t.Fatalf("unexpected clone locality: escaped=%t first=%t second=%t", escaped, machine.localOnlyObjectRefs[first.Ref], machine.localOnlyObjectRefs[second.Ref])
			}
			if len(first.Fields) != 0 || len(second.Fields) != 0 || !original.Fields["total"].Equal(Int(0)) {
				t.Fatalf("clone columns or source changed: original=%#v first=%#v second=%#v", original.Fields, first.Fields, second.Fields)
			}
			if escaped {
				for name, value := range map[string]Value{
					"list": machine.Globals["listAlias"].List[0],
					"map":  machine.Globals["mapAlias"].Map[mapKey(String("clone"))],
					"set":  machine.Globals["setAlias"].Set[0],
				} {
					if value.Ref != first.Ref || len(value.Fields) != 0 {
						t.Errorf("%s lost the empty cloned root: %#v", name, value)
					}
				}
			}
		})
	}
}

func TestJSONLocalOnlyAliasScalarRoots(t *testing.T) {
	for _, method := range []string{"deserialize", "deserializeStrict"} {
		for _, scalar := range []struct {
			typeName string
			payload  string
			expected string
		}{
			{"Date", `"2024-02-29"`, "Date.newInstance(2024, 2, 29)"},
			{"Datetime", `"2024-02-29T12:34:56Z"`, "Datetime.newInstanceGmt(2024, 2, 29, 12, 34, 56)"},
			{"Time", `"05:06:07"`, "Time.newInstance(5, 6, 7, 0)"},
			{"Id", `"001B000001DVM9t"`, "(Id)'001B000001DVM9tIAH'"},
			{"Blob", `"YWJj"`, "Blob.valueOf('abc')"},
			{"UUID", `"00112233-4455-6677-8899-aabbccddeeff"`, "UUID.fromString('00112233-4455-6677-8899-aabbccddeeff')"},
		} {
			t.Run(method+"/"+scalar.typeName, func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, fmt.Sprintf(`
Object root = JSON.%[2]s('%[3]s', %[1]s.class);
Object alias = root;
Object separate = JSON.%[2]s('%[3]s', %[1]s.class);
%[1]s expected = %[4]s;
ScalarBox box = (ScalarBox)JSON.%[2]s('{"Value":%[3]s}', ScalarBox.class);
Object escaped = JSON.%[2]s('%[3]s', %[1]s.class);
`, scalar.typeName, method, scalar.payload, scalar.expected), Class{
					Name:   "ScalarBox",
					Fields: map[string]Field{"Value": {Name: "Value", Type: scalar.typeName}},
				})
				root, alias, separate := machine.Globals["root"], machine.Globals["alias"], machine.Globals["separate"]
				if root.Kind != ValueObject || root.Ref == 0 || root.Ref != alias.Ref || separate.Ref == 0 || separate.Ref == root.Ref || !root.Equal(separate) {
					t.Fatalf("scalar value or reference identity changed: root=%#v alias=%#v separate=%#v", root, alias, separate)
				}
				if !machine.localOnlyObjectRefs[root.Ref] || !machine.localOnlyObjectRefs[separate.Ref] || !machine.localOnlyObjectRefs[machine.Globals["escaped"].Ref] {
					t.Fatal("fresh scalar object roots were not registered")
				}
				nested, escaped := machine.Globals["box"].Fields["Value"], machine.Globals["escaped"]
				if nested.Kind != ValueObject || nested.Ref == 0 || !nested.Equal(root) || machine.localOnlyObjectRefs[nested.Ref] {
					t.Fatalf("nested scalar wrapper must be equal and unregistered: %#v", nested)
				}
				// Inspect raw JSON roots before passing them to assertion methods,
				// which conservatively mark object arguments escaped. Object locals
				// also avoid Date's existing typed-assignment normalization copies.
				checks, err := CompileAnonymous(`
List<Object> listAlias = new List<Object>();
listAlias.add(escaped);
System.assertEquals(expected, root);
System.assertEquals(expected, alias);
System.assertEquals(expected, separate);
System.assertEquals(expected, box.Value);
`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(checks); err != nil {
					t.Fatal(err)
				}
				if machine.localOnlyObjectRefs[escaped.Ref] || machine.Globals["listAlias"].List[0].Ref != escaped.Ref || !escaped.Equal(root) {
					t.Fatal("escaped scalar root lost its value, shared Ref, or escaped status")
				}
			})
		}
	}
}

func runDynamicObjectAliasProgram(t *testing.T, source string, classes ...Class) (*VM, ScopeAliasPerfCounters) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["NumberOfEmployees"] = storage.Field{APIName: "NumberOfEmployees", Type: storage.FieldInteger}
	account.Definition.Fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000002AAA", Active: true, Available: true, Default: true}}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	for _, class := range classes {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	machine.SetTraceEnabled(false)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	return machine, recorder.Snapshot().ScopeAlias
}
