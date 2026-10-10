package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSObjectContainersShadowingUserClassDirectAssignment(t *testing.T) {
	for _, tc := range []struct {
		name, expression, valueType string
	}{
		{"map", "new Map<String,String>{'key'=>'value'}", "Map<String,String>"},
		{"list", "new List<String>{'value'}", "List<String>"},
		{"set", "new Set<String>{'value'}", "Set<String>"},
		{"record", "new Account(Name='value')", "Account"},
	} {
		for _, kind := range []string{"projectClass", "dependencyClass", "sobject"} {
			userClass := kind != "sobject"
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, "")
				storage.EnsureStandardObject(machine.Org, "Folder")
				machine.SetOrg(machine.Org)
				if userClass {
					// public class Folder { public Object Name; }
					if err := machine.RegisterClass(Class{Name: "Folder", Dependency: kind == "dependencyClass", Fields: map[string]Field{
						"Name": {Name: "Name", Type: "Object", Access: "public"},
					}}); err != nil {
						t.Fatal(err)
					}
				}
				aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
Folder folder = new Folder();
Object payload = %s;
String exceptionType;
String message;
try { folder.Name = payload; }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
`, tc.expression))
				if userClass {
					if machine.Globals["exceptionType"].Kind != ValueNull {
						t.Fatalf("user-class assignment threw: %s", machine.Globals["message"].Text)
					}
					if got, want := machine.Globals["folder"].Fields["Name"], machine.Globals["payload"]; got.Ref != want.Ref || !got.Equal(want) {
						t.Fatalf("user-class field lost assigned payload: %#v, want %#v", got, want)
					}
					if _, handled, err := machine.callSObjectMember(machine.Globals["folder"], "put", []Value{String("Name"), machine.Globals["payload"]}); !handled || err != nil {
						t.Fatalf("user-class put applied SObject restrictions: handled=%t err=%v", handled, err)
					}
					// A real record and the shadowing class coexist in this VM.
					aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
SObject record = Schema.getGlobalDescribe().get('Folder').newSObject();
String recordExceptionType;
String recordMessage;
try { record.Name = payload; }
catch (SObjectException e) { recordExceptionType = e.getTypeName(); recordMessage = e.getMessage(); }
System.assertEquals('System.SObjectException', recordExceptionType);
System.assertEquals('Illegal assignment from %s to String', recordMessage);
recordExceptionType = null;
recordMessage = null;
try { record.put('Name', payload); }
catch (SObjectException e) { recordExceptionType = e.getTypeName(); recordMessage = e.getMessage(); }
System.assertEquals('System.SObjectException', recordExceptionType);
System.assertEquals('Illegal assignment from %s to String', recordMessage);
`, tc.valueType, tc.valueType))
				} else {
					// SC001-SC004: scalar record fields reject these runtime values.
					aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Illegal assignment from %s to String', message);
System.assertEquals(null, folder.Name);
`, tc.valueType))
				}
			})
		}
	}
}

// Expected Apex name-resolution behavior; native shadowing rows are pending.
func TestSObjectContainersSchemaQualifiedShadowingClass(t *testing.T) {
	for _, expression := range []string{
		"(SObject)Type.forName('Schema.Folder').newInstance()",
		"Type.forName('Schema.Folder').newInstance()",
		"Schema.Folder.class.newInstance()",
		"Type.forName('schema.folder').newInstance()",
		"new Schema.Folder()",
		"Schema.Folder.SObjectType.newSObject()",
		`JSON.deserialize('{"Name":"before"}', Schema.Folder.class)`,
		`JSON.deserialize('{"Name":"before"}', Type.forName('Schema.Folder'))`,
		`JSON.deserialize('{"Folder":{"Name":"before"}}', Type.forName('Schema.Folder'))`,
	} {
		t.Run(expression, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, "")
			storage.EnsureStandardObject(machine.Org, "Folder")
			machine.SetOrg(machine.Org)
			if err := machine.RegisterClass(Class{Name: "Folder", Access: "public", Fields: map[string]Field{
				"Name": {Name: "Name", Type: "Object", Access: "public"},
			}}); err != nil {
				t.Fatal(err)
			}
			aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
Object original = %s;
Boolean isRecord = original instanceof SObject;
SObject record = (SObject)original;
Schema.Folder qualified = (Schema.Folder)original;
System.assertEquals('Folder', record.getSObjectType().getDescribe().getName());
System.assertEquals('Folder', qualified.getSObjectType().getDescribe().getName());
String exceptionType;
String message;
try { record.put('Name', new Map<String,String>()); }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Illegal assignment from Map<String,String> to String', message);
System.assertEquals(true, isRecord);
Assert.isInstanceOfType(original, Type.forName('Schema.Folder'));
Object preferred = Type.forName('Folder').newInstance();
System.assertEquals(true, preferred instanceof SObject);
Object instance = Type.forName('', 'Folder').newInstance();
System.assertEquals(false, instance instanceof SObject);
System.assertEquals(false, instance instanceof Schema.Folder);
Assert.isNotInstanceOfType(instance, Type.forName('Schema.Folder'));
String castException;
try { SObject invalid = (SObject)instance; }
catch (TypeException e) { castException = e.getTypeName(); }
System.assertEquals('System.TypeException', castException);
castException = null;
try { Schema.Folder invalid = (Schema.Folder)instance; }
catch (TypeException e) { castException = e.getTypeName(); }
System.assertEquals('System.TypeException', castException);
Folder holder = (Folder)instance;
holder.Name = new Map<String,String>{'key'=>'value'};
System.assertEquals('value', ((Map<String,String>)holder.Name).get('key'));
`, expression))
			if strings.Contains(expression, "before") && machine.Globals["record"].Fields["Name"].Text != "before" {
				t.Fatal("qualified JSON record lost its Name value")
			}
			for _, name := range []string{"original", "record", "qualified"} {
				if machine.Globals[name].classInstance {
					t.Fatalf("%s acquired class-instance provenance", name)
				}
			}
			if !machine.Globals["instance"].classInstance {
				t.Fatal("empty-namespace reflection lost class-instance provenance")
			}
		})
	}
}

// Scope controls preserve the pre-fix class/null rules for names that are not
// known records. These assertions are not additional native-conformance rows.
func TestSObjectContainersTypeForNamePreservesUnknownNames(t *testing.T) {
	machine := New(nil)
	for _, name := range []string{"ForNameMissing__c", "ForNameMissing__e", "ForNameMissing__mdt", "ForNameMissingChangeEvent"} {
		if value := machine.typeForName("", name, false); value.Kind != ValueNull {
			t.Fatalf("unknown name %s returned %v", name, value)
		}
	}
	if err := machine.RegisterClass(Class{Name: "ForNameUser__c", Access: "public"}); err != nil {
		t.Fatal(err)
	}
	value := machine.typeForName("", "ForNameUser__c", false)
	if identity := typeValueIdentityName(value); identity != "ForNameUser__c" {
		t.Fatalf("non-record class identity=%q", identity)
	}
}

// Canonical scope control, not an additional Salesforce-conformance row.
func TestSObjectContainersTypeForNameMissingSchemaNameReturnsNull(t *testing.T) {
	for _, withOrg := range []bool{false, true} {
		t.Run(fmt.Sprintf("withOrg=%t", withOrg), func(t *testing.T) {
			machine := New(nil)
			if withOrg {
				machine, _ = runDynamicObjectAliasProgram(t, "")
				storage.EnsureStandardObject(machine.Org, "Folder")
				machine.SetOrg(machine.Org)
			}
			for _, name := range []string{"ForNameMissing__c", "ForNameMissing__e", "ForNameMissing__mdt", "ForNameMissingChangeEvent"} {
				for _, prefix := range []string{"Schema.", "schema."} {
					if value := machine.typeForName("", prefix+name, false); value.Kind != ValueNull {
						t.Fatalf("unknown name %s returned %v", prefix+name, value)
					}
				}
			}
		})
	}
}

// Canonical scope control, not an additional Salesforce-conformance row.
func TestSObjectContainersTypedMapInstanceOfPreservesEntryMatching(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	aliasSObjectPruneExecute(t, machine, `
Object empty = new Map<String,String>();
System.assertEquals(true, empty instanceof Map<Integer,Account>);
Object populated = new Map<String,Object>{'key'=>'value'};
System.assertEquals(true, populated instanceof Map<String,String>);
System.assertEquals(false, populated instanceof Map<Integer,String>);
System.assertEquals(false, populated instanceof Map<String,Account>);
System.assertEquals(true, populated instanceof Map<String,Object>);
`)
}

func TestSObjectContainersSchemaQualifiedListShadowingClass(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	storage.EnsureStandardObject(machine.Org, "Folder")
	machine.SetOrg(machine.Org)
	if err := machine.RegisterClass(Class{Name: "Folder", Access: "public", Fields: map[string]Field{
		"Name": {Name: "Name", Type: "Object", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	// Qualifiers distinguish record and class generic arguments even when empty.
	// Native shadowing rows for these expected-value assertions are pending.
	aliasSObjectPruneExecute(t, machine, `
Object classes = new List<Folder>();
Object records = new List<Schema.Folder>();
System.assertEquals(false, classes instanceof List<Schema.Folder>);
System.assertEquals(true, records instanceof List<Schema.Folder>);
`)
}

func TestSObjectContainersSchemaQualifiedTypeIdentity(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	storage.EnsureStandardObject(machine.Org, "Folder")
	machine.SetOrg(machine.Org)
	aliasSObjectPruneExecute(t, machine, `
Type qualified = Type.forName('Schema.Folder');
Type unqualified = Type.forName('Folder');
System.assertEquals(true, qualified.equals(unqualified));
System.assertEquals(true, qualified.equals(Folder.class));
System.assertEquals(qualified.hashCode(), unqualified.hashCode());
System.assertEquals(qualified.hashCode(), Schema.Folder.class.hashCode());
Map<Type,String> types = new Map<Type,String>{qualified=>'record'};
System.assertEquals('record', types.get(unqualified));
System.assertEquals('record', types.get(Folder.class));
`)
	if err := machine.RegisterClass(Class{Name: "Folder", Access: "public"}); err != nil {
		t.Fatal(err)
	}
	aliasSObjectPruneExecute(t, machine, `
Type recordType = Type.forName('Schema.Folder');
Type classType = Type.forName('', 'Folder');
System.assertEquals(false, recordType.equals(classType));
System.assertEquals(null, types.get(classType));
SObject retained = (SObject)qualified.newInstance();
System.assertEquals(true, retained instanceof SObject);
System.assertEquals('Folder', retained.getSObjectType().getDescribe().getName());
`)
	if !machine.typeAssignableTo("Schema.Folder", "System.Object") {
		t.Fatal("schema record lost System.Object assignability")
	}
}

func TestSObjectContainersSchemaQualifiedBaseType(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	aliasSObjectPruneExecute(t, machine, `
System.assertEquals(true, SObject.class.isAssignableFrom(Account.class));
System.assertEquals(true, Object.class.isAssignableFrom(Account.class));
List<SObject> records = (List<SObject>)Type.forName('List<SObject>').newInstance();
records.add(new Account(Name='record'));
System.assertEquals(1, records.size());
`)
}

func TestSObjectContainersShadowingUserClassJSON(t *testing.T) {
	for _, tc := range []struct{ payload, fieldType string }{
		{`{"Name":{"key":"value"}}`, "Map<String,String>"},
		{`{"Name":["value"]}`, "List<String>"},
		{`{"Name":{"key":"value"}}`, "Object"},
		{`{"Name":["value"]}`, "Object"},
	} {
		payload := tc.payload
		for _, method := range []string{"deserialize", "deserializeStrict"} {
			for _, userClass := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/%s/userClass=%t", tc.fieldType, payload, method, userClass), func(t *testing.T) {
					machine, _ := runDynamicObjectAliasProgram(t, "")
					storage.EnsureStandardObject(machine.Org, "Folder")
					machine.SetOrg(machine.Org)
					if userClass {
						// Object fields retain the ordinary class JSON unsupported-type
						// contract. Typed container fields exercise successful mapping.
						if err := machine.RegisterClass(Class{Name: "Folder", Fields: map[string]Field{
							"Name": {Name: "Name", Type: tc.fieldType, Access: "public"},
						}}); err != nil {
							t.Fatal(err)
						}
					}
					aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
Folder folder;
String exceptionType;
String message;
try { folder = (Folder)JSON.%s('%s', Folder.class); }
catch (JSONException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
`, method, payload))
					if userClass {
						if tc.fieldType == "Object" {
							aliasSObjectPruneExecute(t, machine, `
System.assertEquals('System.JSONException', exceptionType);
System.assertEquals('Apex Type unsupported in JSON: Object', message);
`)
							return
						}
						if machine.Globals["exceptionType"].Kind != ValueNull {
							t.Fatalf("user-class JSON mapping threw: %s", machine.Globals["message"].Text)
						}
						name := machine.Globals["folder"].Fields["Name"]
						if strings.Contains(payload, "[") {
							if name.Kind != ValueList || len(name.List) != 1 || name.List[0].Text != "value" {
								t.Fatalf("user-class JSON list = %#v", name)
							}
						} else if name.Kind != ValueMap || name.Map[mapKey(String("key"))].Text != "value" {
							t.Fatalf("user-class JSON map = %#v", name)
						}
					} else {
						// SC014-SC016: scalar record fields reject object/array tokens.
						message := "Cannot deserialize instance of string from START_ARRAY value [line:1, column:2]"
						if !strings.Contains(payload, "[") {
							message = "Cannot deserialize instance of string from START_OBJECT value { or request may be missing a required field at [line:1, column:2]"
						}
						aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
System.assertEquals('System.JSONException', exceptionType);
System.assertEquals('%s', message);
`, message))
					}
				})
			}
		}
	}
}

func TestSObjectContainersJSONRecordWithShadowingUserClass(t *testing.T) {
	for _, method := range []string{"deserialize", "deserializeStrict"} {
		t.Run(method, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, "")
			storage.EnsureStandardObject(machine.Org, "Folder")
			machine.SetOrg(machine.Org)
			if err := machine.RegisterClass(Class{Name: "Folder", Fields: map[string]Field{
				"Name": {Name: "Name", Type: "Object", Access: "public"},
			}}); err != nil {
				t.Fatal(err)
			}
			aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
SObject record = (SObject)JSON.%s('{"attributes":{"type":"Folder"},"Name":"before"}', SObject.class);
String exceptionType;
String message;
try { record.put('Name', new Map<String,String>()); }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Illegal assignment from Map<String,String> to String', message);
System.assertEquals('before', record.get('Name'));
`, method))
		})
	}
}

func TestSObjectContainersRelationshipPutWithShadowingUserClass(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	storage.EnsureStandardObject(machine.Org, "Account")
	storage.EnsureStandardObject(machine.Org, "Contact")
	machine.SetOrg(machine.Org)
	if err := machine.RegisterClass(Class{Name: "Account", Fields: map[string]Field{
		"Parent":   {Name: "Parent", Type: "Object", Access: "public"},
		"Contacts": {Name: "Contacts", Type: "Object", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	aliasSObjectPruneExecute(t, machine, `Account holder = new Account(); SObject record = Schema.getGlobalDescribe().get('Account').newSObject();`)
	for _, field := range []string{"Parent", "Contacts"} {
		if _, handled, err := machine.callSObjectMember(machine.Globals["holder"], "put", []Value{String(field), Map()}); !handled || err != nil {
			t.Fatalf("class %s put = handled %t, err %v", field, handled, err)
		}
		aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
String exceptionType;
String message;
try { record.put('%s', new Map<String,String>()); }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Invalid field %s for Account', message);
`, field, field))
	}
}

func TestSObjectContainersJSONRelationshipsWithShadowingUserClass(t *testing.T) {
	for _, tc := range []struct {
		name, root, shadow, field, envelope, bind string
	}{
		{"parent", "Contact", "Account", "Name", `{"attributes":{"type":"Contact"},"Account":{"Name":%s}}`, "root.getSObject('Account')"},
		{"child", "Account", "Contact", "LastName", `{"attributes":{"type":"Account"},"Contacts":{"totalSize":1,"done":true,"records":[{"LastName":%s}]}}`, "root.getSObjects('Contacts')[0]"},
		{"dotted", "Contact", "Account", "Name", `{"attributes":{"type":"Contact"},"Account.Name":%s}`, "root.getSObject('Account')"},
	} {
		for _, target := range []string{"SObject", tc.root} {
			for _, method := range []string{"deserialize", "deserializeStrict"} {
				if tc.name == "dotted" && method == "deserializeStrict" {
					continue
				}
				t.Run(tc.name+"/"+target+"/"+method, func(t *testing.T) {
					machine, _ := runDynamicObjectAliasProgram(t, "")
					storage.EnsureStandardObject(machine.Org, "Account")
					storage.EnsureStandardObject(machine.Org, "Contact")
					machine.SetOrg(machine.Org)
					if err := machine.RegisterClass(Class{Name: tc.shadow, Fields: map[string]Field{
						tc.field: {Name: tc.field, Type: "String", Access: "public"},
					}}); err != nil {
						t.Fatal(err)
					}
					badPayload := fmt.Sprintf(tc.envelope, `{"key":"value"}`)
					fieldKey := tc.field
					if tc.name == "dotted" {
						fieldKey = "Account.Name"
					}
					column := strings.Index(badPayload, `"`+fieldKey+`"`) + 1
					aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
String exceptionType;
String message;
try { Object bad = JSON.%s('%s', %s.class); }
catch (JSONException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.JSONException', exceptionType);
System.assertEquals('Cannot deserialize instance of string from START_OBJECT value { or request may be missing a required field at [line:1, column:%d]', message);
SObject root = (SObject)JSON.%s('%s', %s.class);
SObject related = %s;
exceptionType = null;
message = null;
try { related.put('%s', new Map<String,String>()); }
catch (SObjectException e) { exceptionType = e.getTypeName(); message = e.getMessage(); }
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Illegal assignment from Map<String,String> to String', message);
System.assertEquals('before', related.get('%s'));
`, method, badPayload, target, column, method, fmt.Sprintf(tc.envelope, `"before"`), target, tc.bind, tc.field, tc.field))
				})
			}
		}
	}
}

func TestSObjectContainersJSONFieldPositions(t *testing.T) {
	// SC014-SC016 establish the String token templates. These additional
	// placements exercise source propagation, not independently captured rows.
	for _, tc := range []struct {
		name, payload, target, token, location string
	}{
		{"object", `{"Name":{}}`, "Account", "START_OBJECT", "line:1, column:2"},
		{"array", `{"Name":[]}`, "Account", "START_ARRAY", "line:1, column:2"},
		{"multiline", "{\n  \"Name\":{}}", "Account", "START_OBJECT", "line:2, column:3"},
		{"utf16", `{"Ignored":"😀","Name":[]}`, "Account", "START_ARRAY", "line:1, column:17"},
		{"generic", `{"attributes":{"type":"Account"},"Name":[]}`, "SObject", "START_ARRAY", "line:1, column:34"},
		{"list", `[{"Name":{}}]`, "List<Account>", "START_OBJECT", "line:1, column:3"},
		{"parent", `{"Parent":{"Name":{}}}`, "Account", "START_OBJECT", "line:1, column:12"},
		{"dotted", `{"Parent.Name":[]}`, "Account", "START_ARRAY", "line:1, column:2"},
		{"wrapped", `{"Account":{"Name":{}}}`, "Account", "START_OBJECT", "line:1, column:13"},
	} {
		for _, method := range []string{"deserialize", "deserializeStrict"} {
			// Root envelopes and dotted names are deserialize-only contracts.
			if (tc.name == "wrapped" || tc.name == "dotted") && method == "deserializeStrict" {
				continue
			}
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, "")
				storage.EnsureStandardObject(machine.Org, "Account")
				account := machine.Org.Objects["Account"]
				account.Definition.Fields["Ignored"] = storage.Field{APIName: "Ignored", Type: storage.FieldString}
				machine.Org.Objects["Account"] = account
				machine.SetOrg(machine.Org)
				message := "Cannot deserialize instance of string from " + tc.token + " value"
				if tc.token == "START_OBJECT" {
					message += " { or request may be missing a required field at"
				}
				message += " [" + tc.location + "]"
				payload := strings.ReplaceAll(strings.ReplaceAll(tc.payload, "\\", "\\\\"), "'", "\\'")
				aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
String exceptionType;
String message;
try {
    Object value = JSON.%s('%s', %s.class);
} catch (JSONException e) {
    exceptionType = e.getTypeName();
    message = e.getMessage();
}
System.assertEquals('System.JSONException', exceptionType);
System.assertEquals('%s', message);
`, method, payload, tc.target, message))
			})
		}
	}
}

func TestSObjectContainersRelationshipPutRejectsEveryValue(t *testing.T) {
	// SC006/SC007 establish the parent-name policy; SC040-SC042 capture child
	// list/null/map rejection. Other scalar values extend the same task policy.
	for _, field := range []string{"Parent", "Contacts"} {
		for _, value := range []string{"null", "'x'", "1", "new Account()", "new Map<String,String>()", "new List<Contact>()"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, "")
				storage.EnsureStandardObject(machine.Org, "Account")
				storage.EnsureStandardObject(machine.Org, "Contact")
				machine.SetOrg(machine.Org)
				aliasSObjectPruneExecute(t, machine, fmt.Sprintf(`
Account record = new Account(Name='before');
String exceptionType;
String message;
try {
    record.put('%s', %s);
} catch (SObjectException e) {
    exceptionType = e.getTypeName();
    message = e.getMessage();
}
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('Invalid field %s for Account', message);
System.assertEquals('before', record.Name);
`, field, value, field))
			})
		}
	}
}
