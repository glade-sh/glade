package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestExecJSONGeneratorWritesObjectsArraysAndScalars(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.writeStringField('name', 'Acme');
gen.writeNumberField('n', 7);
gen.writeBooleanField('ok', true);
gen.writeNullField('missing');
gen.writeFieldName('items');
gen.writeStartArray();
gen.writeString('x');
gen.writeNumber(2);
gen.writeBoolean(false);
gen.writeNull();
gen.writeEndArray();
gen.writeEndObject();
System.assertEquals('{"name":"Acme","n":7,"ok":true,"missing":null,"items":["x",2,false,null]}', gen.getAsString());
System.assert(gen.isClosed());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserGeneratorMethodsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = json.CREATEGENERATOR(false);
gen.WriteStartObject();
gen.WRITESTRINGFIELD('name', 'Acme');
gen.writeFieldName('items');
gen.WriteStartArray();
gen.WriteNumber(7);
gen.writeEndArray();
gen.WRITEENDOBJECT();
System.assertEquals('{"name":"Acme","items":[7]}', gen.GETASSTRING());
System.assert(gen.ISCLOSED());

JSONParser parser = JSON.createparser('{"name":"Acme"}');
System.assertEquals(JSONToken.START_OBJECT, parser.NEXTTOKEN());
System.assertEquals(jsontoken.FIELD_NAME, parser.nexttoken());
System.assertEquals('name', parser.GETTEXT());
System.assertEquals(JSONToken.VALUE_STRING, parser.NextValue());
System.assertEquals('Acme', parser.getText());
parser.ClearCurrentToken();
System.assertEquals(null, parser.GetCurrentToken());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONAliasSupportsDeserializeStrict(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> parsed = (Map<String,Object>)Json.deserializeStrict('{"Name":"Acme"}', Map<String,Object>.class);
System.assertEquals('Acme', (String)parsed.get('Name'));
Map<String,Object> parsedSystem = (Map<String,Object>)System.JSON.deserialize('{"Name":"Trail"}', Map<String,Object>.class);
System.assertEquals('Trail', (String)parsedSystem.get('Name'));
Map<String,Object> parsedLower = (Map<String,Object>)json.deserialize('{"Name":"Lower"}', Map<String,Object>.class);
System.assertEquals('Lower', (String)parsedLower.get('Name'));
Map<String,List<String>> parsedNested = (Map<String,List<String>>)JSON.deserialize('{"Account":["Id","Name"]}', Map<String,List<String>>.Class);
System.assertEquals('Name', parsedNested.get('Account')[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeLowercaseIDSupportsGenericUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Old');
insert existing;
List<sObject> records = (List<sObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"id":"' + existing.Id + '","Name":"New"}]', List<sObject>.class);
List<Database.SaveResult> results = Database.update(records, false);
System.assertEquals(true, results[0].isSuccess());
Account loaded = [SELECT Name FROM Account WHERE Id = :existing.Id];
System.assertEquals('New', loaded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeDuplicateIDCasePrefersCanonicalID(t *testing.T) {
	program, err := CompileAnonymous(`
Account canonical = new Account(Name = 'Canonical');
Account lowercase = new Account(Name = 'Lowercase');
insert new List<Account>{canonical, lowercase};
List<sObject> records = (List<sObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"id":"' + lowercase.Id + '","Id":"' + canonical.Id + '","Name":"Changed"}]', List<sObject>.class);
List<Database.SaveResult> results = Database.update(records, false);
System.assertEquals(true, results[0].isSuccess());
Account canonicalLoaded = [SELECT Name FROM Account WHERE Id = :canonical.Id];
Account lowercaseLoaded = [SELECT Name FROM Account WHERE Id = :lowercase.Id];
System.assertEquals('Changed', canonicalLoaded.Name);
System.assertEquals('Lowercase', lowercaseLoaded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeEmptyCanonicalIDFallsBackToLowercaseID(t *testing.T) {
	program, err := CompileAnonymous(`
Account lowercase = new Account(Name = 'Lowercase');
insert lowercase;
List<sObject> records = (List<sObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"Id":"","id":"' + lowercase.Id + '","Name":"Changed"}]', List<sObject>.class);
List<Database.SaveResult> results = Database.update(records, false);
System.assertEquals(true, results[0].isSuccess());
Account loaded = [SELECT Name FROM Account WHERE Id = :lowercase.Id];
System.assertEquals('Changed', loaded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeListSObjectUsesAttributesType(t *testing.T) {
	program, err := CompileAnonymous(`
List<sObject> records = (List<sObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"Name":"Local Probe"}]', List<sObject>.class);
System.assertEquals(1, records.size());
System.assertEquals('Local Probe', (String)records[0].get('Name'));
List<string> ids = (List<string>)JSON.deserialize('["001000000000001AAA"]', List<string>.class);
System.assertEquals('001000000000001AAA', ids[0]);
	Widget__c widget = (Widget__c)JSON.deserialize('{"Data__c":"{}"}', Widget__c.class);
System.assertEquals('{}', (String)widget.get('Data__c'));
Widget__c strictWidget = (Widget__c)JSON.deserializeStrict('{"Data__c":"strict"}', Widget__c.class);
System.assertEquals('strict', (String)strictWidget.get('Data__c'));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeUntypedMapRemoveReturnsRemovedValue(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> untyped = (Map<String,Object>)JSON.deserializeUntyped('{"items":[{"name":"one"}],"nextCursor":null}');
List<Object> items = (List<Object>)untyped.remove('items');
System.assertEquals(1, items.size());
System.assertEquals(false, untyped.containsKey('items'));
try {
	List<Object> bad = (List<Object>)untyped;
	System.assert(false, 'cast should fail');
} catch (System.TypeException e) {
	System.assertEquals('Invalid conversion from runtime type Map<String,ANY> to List<ANY>', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONUntypedListCastsMapsToTypedSObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = JSON.deserializeUntyped('[{"attributes":{"type":"Account"},"Name":"Acme"}]');
List<Account> accounts = (List<Account>)raw;
System.assertEquals(1, accounts.size());
System.assertEquals('Acme', accounts.get(0).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectSystemFields(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c widget = (Widget__c)JSON.deserializeStrict('{"CreatedDate":"2026-05-13T10:30:00Z","IsDeleted":false}', Widget__c.class);
System.assertEquals(Date.newInstance(2026, 5, 13), widget.CreatedDate.date());
System.assertEquals(false, widget.IsDeleted);
Widget__c withParent = (Widget__c)JSON.deserializeStrict('{"Account__r":{"attributes":{"type":"Account"},"Name":"Parent"}}', Widget__c.class);
System.assertEquals('Parent', withParent.Account__r.Name);
JSON.deserializeStrict('{"UnmodeledLines__r":{"totalSize":0,"done":true,"records":[]}}', Widget__c.class);
		`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "pkg"
	org.Objects["pkg__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"pkg__Data__c":    {APIName: "pkg__Data__c", Type: storage.FieldString},
				"pkg__Account__c": {APIName: "pkg__Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "pkg__Account__r"},
			},
			Relations: []storage.Relationship{
				{Field: "pkg__Account__c", ParentObjects: []string{"Account"}, ParentRelationship: "pkg__Account__r", ChildRelationship: "Widgets__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorPrettyPrints(t *testing.T) {
	program, err := CompileAnonymous(`
	JSONGenerator gen = JSON.createGenerator(true);
gen.writeStartObject();
gen.writeStringField('name', 'Acme');
gen.writeFieldName('items');
gen.writeStartArray();
gen.writeString('x');
gen.writeNumber(2);
gen.writeEndArray();
gen.writeEndObject();
String text = gen.getAsString();
System.assert(text.contains('  "name": "Acme"'));
System.assert(text.contains('  "items": ['));
System.assert(text.contains('    "x"'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorRejectsInvalidOrder(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeString('first');
gen.writeNumber(2);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "root value already written") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONGeneratorReportsPendingFieldName(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.writeFieldName('firstName');
gen.writeFieldName('lastName');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	err = func() error {
		_, execErr := machine.Execute(program)
		return execErr
	}()
	if err == nil || !strings.Contains(err.Error(), `field "firstName" is missing a value`) {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONGeneratorRejectsFieldNameInArrayAsJSONException(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
String caught = '';
try {
	gen.writeFieldName('bad');
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator.writeFieldName cannot be called inside an array'));
gen.writeString('ok');
gen.writeEndArray();
System.assertEquals('["ok"]', gen.getAsString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorRejectsEndObjectInArrayAsJSONException(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
String caught = '';
try {
	gen.writeEndObject();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator.writeEndObject cannot be called inside an array'));
gen.writeString('ok');
gen.writeEndArray();
System.assertEquals('["ok"]', gen.getAsString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorRejectsEndArrayInObjectAsJSONException(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
String caught = '';
try {
	gen.writeEndArray();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator.writeEndArray cannot be called inside an object'));
gen.writeStringField('ok', 'yes');
gen.writeEndObject();
System.assertEquals('{"ok":"yes"}', gen.getAsString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorUnhandledEndObjectInArrayHasJSONExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
gen.writeEndObject();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v, want RuntimeError", err)
	}
	if runtimeErr.Type != "JSONException" {
		t.Fatalf("type = %q, want JSONException", runtimeErr.Type)
	}
	if !strings.Contains(runtimeErr.Message, "JSONGenerator.writeEndObject cannot be called inside an array") {
		t.Fatalf("message = %q", runtimeErr.Message)
	}
}

func TestExecJSONGeneratorUnhandledEndArrayInObjectHasJSONExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.writeEndArray();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v, want RuntimeError", err)
	}
	if runtimeErr.Type != "JSONException" {
		t.Fatalf("type = %q, want JSONException", runtimeErr.Type)
	}
	if !strings.Contains(runtimeErr.Message, "JSONGenerator.writeEndArray cannot be called inside an object") {
		t.Fatalf("message = %q", runtimeErr.Message)
	}
}

func TestExecJSONGeneratorUnhandledFieldNameInArrayHasJSONExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
gen.writeFieldName('bad');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v, want RuntimeError", err)
	}
	if runtimeErr.Type != "JSONException" {
		t.Fatalf("type = %q, want JSONException", runtimeErr.Type)
	}
	if !strings.Contains(runtimeErr.Message, "JSONGenerator.writeFieldName cannot be called inside an array") {
		t.Fatalf("message = %q", runtimeErr.Message)
	}
}

func TestExecJSONGeneratorRejectsCloseAndClosedStateEdges(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "close with no root",
			source: `
JSONGenerator gen = JSON.createGenerator(false);
gen.close();
`,
			want: "cannot close before writing a root value",
		},
		{
			name: "close with open object",
			source: `
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.close();
`,
			want: "cannot close with open JSON containers",
		},
		{
			name: "write after close",
			source: `
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
gen.writeEndArray();
gen.close();
gen.writeNull();
`,
			want: "JSONGenerator is closed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecJSONGeneratorWriteAfterGetAsStringIsCatchableJSONException(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
gen.writeString('done');
gen.writeEndArray();
System.assertEquals('["done"]', gen.getAsString());
System.assert(gen.isClosed());
Boolean caught = false;
try {
	gen.writeNull();
} catch (JSONException e) {
	caught = true;
	System.assertEquals('System.JSONException', e.getTypeName());
	System.assert(e.getMessage().contains('JSONGenerator is closed'));
}
System.assert(caught);
caught = false;
try {
	gen.writeRawValue('{bad');
} catch (JSONException e) {
	caught = true;
	System.assertEquals('System.JSONException', e.getTypeName());
	System.assert(e.getMessage().contains('JSONGenerator is closed'));
}
System.assert(caught);
System.assert(gen.isClosed());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorStateErrorsAreCatchableAndRecoverable(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator objectGen = JSON.createGenerator(false);
objectGen.writeStartObject();
String caught = '';
try {
	objectGen.writeNull();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator object value requires writeFieldName first'));
objectGen.writeStringField('ok', 'yes');
objectGen.writeEndObject();
System.assertEquals('{"ok":"yes"}', objectGen.getAsString());

JSONGenerator pendingGen = JSON.createGenerator(false);
pendingGen.writeStartObject();
pendingGen.writeFieldName('first');
caught = '';
try {
	pendingGen.writeStringField('second', 'bad');
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator field "first" is missing a value'));
pendingGen.writeString('fixed');
pendingGen.writeEndObject();
System.assertEquals('{"first":"fixed"}', pendingGen.getAsString());

JSONGenerator rootGen = JSON.createGenerator(false);
rootGen.writeObject(null);
caught = '';
try {
	rootGen.writeObject('bad');
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator root value already written'));
System.assertEquals('null', rootGen.getAsString());

JSONGenerator endGen = JSON.createGenerator(false);
caught = '';
try {
	endGen.writeEndArray();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONGenerator.writeEndArray has no open array'));
endGen.writeStartArray();
endGen.writeNull();
endGen.writeEndArray();
System.assertEquals('[null]', endGen.getAsString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserNavigatesTokensAndAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('{"name":"Acme","n":7,"ratio":1.5,"ok":true,"missing":null,"items":["x",2]}');
System.assertEquals(JSONToken.START_OBJECT, parser.nextToken());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('name', parser.getText());
System.assertEquals(JSONToken.VALUE_STRING, parser.nextToken());
System.assertEquals('name', parser.getCurrentName());
System.assertEquals('Acme', parser.getText());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('n', parser.getText());
System.assertEquals(JSONToken.VALUE_NUMBER_INT, parser.nextValue());
System.assertEquals(7, parser.getIntegerValue());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals(JSONToken.VALUE_NUMBER_FLOAT, parser.nextValue());
System.assertEquals(1.5, parser.getDecimalValue());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals(JSONToken.VALUE_TRUE, parser.nextValue());
System.assertEquals(true, parser.getBooleanValue());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals(JSONToken.VALUE_NULL, parser.nextValue());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('items', parser.getText());
System.assertEquals(JSONToken.START_ARRAY, parser.nextToken());
parser.skipChildren();
System.assertEquals(JSONToken.END_ARRAY, parser.getCurrentToken());
System.assertEquals(JSONToken.END_OBJECT, parser.nextToken());
System.assertEquals(null, parser.nextToken());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserPlatformAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('{"date":"2024-02-29","when":"2024-02-29T12:34:56Z","clock":"05:06:07","id":"001B000001DVM9t","blob":"YWJj"}');
parser.nextToken();
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals(JSONToken.VALUE_STRING, parser.nextValue());
Date dateValue = parser.getDateValue();
System.assertEquals(Date.newInstance(2024, 2, 29), dateValue);
parser.nextToken();
parser.nextValue();
Datetime whenValue = parser.getDatetimeValue();
System.assertEquals(Datetime.newInstance(2024, 2, 29, 12, 34, 56), whenValue);
parser.nextToken();
parser.nextValue();
Time clockValue = parser.getTimeValue();
System.assertEquals(Time.newInstance(5, 6, 7, 0), clockValue);
parser.nextToken();
parser.nextValue();
Id idValue = parser.getIdValue();
System.assertEquals('001B000001DVM9t', idValue.toString());
System.assertEquals('001B000001DVM9t', idValue.to15());
parser.nextToken();
parser.nextValue();
Blob blobValue = parser.getBlobValue();
System.assertEquals('abc', blobValue.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserRejectsWrongAccessorState(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('{"n":"not-number"}');
parser.nextToken();
parser.nextToken();
parser.nextValue();
Integer n = parser.getIntegerValue();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "requires VALUE_NUMBER_INT") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONParserErrorsAreCatchableAndStatePreserving(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	JSONParser bad = JSON.createParser('{"broken":');
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONParser invalid JSON input'));

JSONParser parser = JSON.createParser('[null,true,"not-a-date"]');
caught = '';
try {
	parser.getText();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONParser.getText requires a current token'));
System.assertEquals(JSONToken.START_ARRAY, parser.nextToken());
System.assertEquals(JSONToken.VALUE_NULL, parser.nextToken());
caught = '';
try {
	parser.getBooleanValue();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONParser.getBooleanValue requires VALUE_TRUE or VALUE_FALSE'));
System.assertEquals(JSONToken.VALUE_TRUE, parser.nextToken());
System.assertEquals(true, parser.getBooleanValue());
System.assertEquals(JSONToken.VALUE_STRING, parser.nextToken());
caught = '';
try {
	parser.getDateValue();
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSONParser.getDateValue cannot parse Date'));
System.assertEquals(JSONToken.END_ARRAY, parser.nextToken());
System.assertEquals(null, parser.nextToken());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorWritesRawValues(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.writeRawField('config', '{"enabled":true,"nums":[1,2]}');
gen.writeFieldName('items');
gen.writeStartArray();
gen.writeString('first');
gen.writeRawValue('{"raw":true}');
gen.writeRaw('[false,null]');
gen.writeEndArray();
gen.writeEndObject();
System.assertEquals('{"config":{"enabled":true,"nums":[1,2]},"items":["first",{"raw":true},[false,null]]}', gen.getAsString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorRejectsInvalidRawValue(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartArray();
gen.writeRawValue('{bad');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "expects valid raw JSON value") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONParserClearCurrentTokenAndNumericAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('{"whole":12,"fraction":1.25}');
System.assertEquals(null, parser.getCurrentName());
System.assertEquals(JSONToken.START_OBJECT, parser.nextToken());
System.assertEquals(null, parser.getCurrentName());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('whole', parser.getCurrentName());
System.assertEquals(JSONToken.VALUE_NUMBER_INT, parser.nextValue());
System.assertEquals('whole', parser.getCurrentName());
System.assertEquals(12, parser.getLongValue());
parser.clearCurrentToken();
System.assertEquals(null, parser.getCurrentToken());
System.assertEquals(null, parser.getCurrentName());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals(JSONToken.VALUE_NUMBER_FLOAT, parser.nextValue());
System.assertEquals(1.25, parser.getDoubleValue());
System.assertEquals(1.25, parser.getDecimalValue());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserClearAndSkipChildrenStateEdges(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('{"outer":{"inner":1},"tail":2}');
System.assertEquals(JSONToken.START_OBJECT, parser.nextToken());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('outer', parser.getCurrentName());
System.assertEquals(JSONToken.START_OBJECT, parser.nextValue());
System.assertEquals('outer', parser.getCurrentName());
parser.skipChildren();
System.assertEquals(JSONToken.END_OBJECT, parser.getCurrentToken());
System.assertEquals('outer', parser.getCurrentName());
parser.clearCurrentToken();
System.assertEquals(null, parser.getCurrentToken());
System.assertEquals(null, parser.getCurrentName());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('tail', parser.getCurrentName());
System.assertEquals(JSONToken.VALUE_NUMBER_INT, parser.nextValue());
System.assertEquals('tail', parser.getCurrentName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeTypedPrimitiveCollectionAndPlatformScalars(t *testing.T) {
	program, err := CompileAnonymous(`
Integer n = JSON.deserialize('7', Integer.class);
System.assertEquals(7, n);
Long big = JSON.deserialize('9223372036854775807', Long.class);
System.assertEquals(9223372036854775807, big);
Decimal ratio = JSON.deserialize('1.25', Decimal.class);
System.assertEquals(1.25, ratio);
Boolean ok = JSON.deserialize('true', Boolean.class);
System.assertEquals(true, ok);
String text = JSON.deserialize('"Acme"', String.class);
System.assertEquals('Acme', text);
String numericText = JSON.deserialize('12.5', String.class);
System.assertEquals('12.5', numericText);
Object missing = JSON.deserialize('null', String.class);
System.assertEquals(null, missing);
Date dateValue = JSON.deserialize('"2024-02-29"', Date.class);
System.assertEquals(Date.newInstance(2024, 2, 29), dateValue);
Date looseDateValue = JSON.deserialize('"2024-2-29 00:00:00"', Date.class);
System.assertEquals(Date.newInstance(2024, 2, 29), looseDateValue);
Datetime whenValue = JSON.deserialize('"2024-02-29T12:34:56Z"', Datetime.class);
System.assertEquals(Datetime.newInstance(2024, 2, 29, 12, 34, 56), whenValue);
Time timeValue = JSON.deserialize('"05:06:07"', Time.class);
System.assertEquals(Time.newInstance(5, 6, 7, 0), timeValue);
Id idValue = JSON.deserialize('"001B000001DVM9t"', Id.class);
System.assertEquals('001B000001DVM9t', idValue.toString());
UUID uuidValue = JSON.deserialize('"00112233-4455-6677-8899-aabbccddeeff"', UUID.class);
System.assertEquals('00112233-4455-6677-8899-aabbccddeeff', uuidValue.toString());
Blob blobValue = JSON.deserialize('"YWJj"', Blob.class);
System.assertEquals('abc', blobValue.toString());
Type listType = Type.forName('List<Integer>');
List<Integer> nums = JSON.deserialize('[1,2,null]', listType);
System.assertEquals(3, nums.size());
System.assertEquals(2, nums.get(1));
System.assertEquals(null, nums.get(2));
Type mapType = Type.forName('Map<String,Integer>');
Map<String,Integer> counts = JSON.deserialize('{"a":1,"b":null}', mapType);
System.assertEquals(1, counts.get('a'));
System.assertEquals(null, counts.get('b'));
Type idMapType = Type.forName('Map<Id,String>');
Map<Id,String> byId = JSON.deserialize('{"001B000001DVM9t":"Acme"}', idMapType);
System.assertEquals('Acme', byId.get(idValue));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONRoundTripsTypedIdSObjectMap(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001B000001DVM9tIAH';
Map<Id, Account> accounts = new Map<Id, Account>();
accounts.put(accountId, new Account(FirstName = 'NewFirst'));
String raw = JSON.serialize(accounts);
Map<Id, Account> decoded = (Map<Id, Account>)JSON.deserialize(raw, Map<Id, Account>.class);
System.assertEquals(1, decoded.size());
System.assert(decoded.containsKey(accountId));
System.assertEquals('NewFirst', decoded.get(accountId).FirstName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONRoundTripTypedIdSObjectMapCanUpdateRecord(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Old');
insert account;
Map<Id, Account> updates = new Map<Id, Account>();
updates.put(account.Id, new Account(Name = 'New'));
String raw = JSON.serialize(updates);
Map<Id, Account> decoded = (Map<Id, Account>)JSON.deserialize(raw, Map<Id, Account>.class);
Account updateRecord = decoded.get(account.Id);
updateRecord.Id = account.Id;
update updateRecord;
Account loaded = [SELECT Name FROM Account WHERE Id = :account.Id];
System.assertEquals('New', loaded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONRoundTripTypedIdSObjectMapCanClearRecordField(t *testing.T) {
	program, err := CompileAnonymous(`
Membership__c membership = new Membership__c();
insert membership;
Account account = new Account(Name = 'Old', Membership__c = membership.Id);
insert account;
Map<Id, Account> updates = new Map<Id, Account>();
Account updateAccount = new Account();
updateAccount.put('Membership__c', null);
updates.put(account.Id, updateAccount);
String raw = JSON.serialize(updates);
Map<Id, Account> decoded = (Map<Id, Account>)JSON.deserialize(raw, Map<Id, Account>.class);
Account updateRecord = decoded.get(account.Id);
updateRecord.Id = account.Id;
update updateRecord;
Account loaded = [SELECT Membership__c FROM Account WHERE Id = :account.Id];
System.assertEquals(null, loaded.Membership__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Account"].Definition.Fields["Membership__c"] = storage.Field{APIName: "Membership__c", Type: storage.FieldReference, ReferenceTo: []string{"Membership__c"}}
	org.Objects["Membership__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Membership__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Id": {APIName: "Id", Type: storage.FieldID},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONRoundTripsClassWithTypedIdSObjectMap(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001B000001DVM9tIAH';
SavedState state = new SavedState();
state.NewValues = new Map<Id, Account>();
state.NewValues.put(accountId, new Account(FirstName = 'NewFirst'));
String raw = JSON.serialize(state);
SavedState decoded = (SavedState)JSON.deserialize(raw, SavedState.class);
System.assertEquals(1, decoded.NewValues.size());
System.assert(decoded.NewValues.containsKey(accountId));
System.assertEquals('NewFirst', decoded.NewValues.get(accountId).FirstName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "SavedState",
		Fields: map[string]Field{
			"NewValues": {Name: "NewValues", Type: "Map<Id,Account>"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONRoundTripsExplicitNullSObjectFieldInTypedIdMap(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001B000001DVM9tIAH';
SavedState state = new SavedState();
state.NewValues = new Map<Id, Account>();
state.NewValues.put(accountId, new Account(Membership__c = null));
String raw = JSON.serialize(state);
SavedState decoded = (SavedState)JSON.deserialize(raw, SavedState.class);
Account updateRecord = decoded.NewValues.get(accountId);
System.assert(updateRecord.getPopulatedFieldsAsMap().containsKey('Membership__c'));
System.assertEquals(null, updateRecord.Membership__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Account"].Definition.Fields["Membership__c"] = storage.Field{APIName: "Membership__c", Type: storage.FieldReference, ReferenceTo: []string{"Membership__c"}}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "SavedState",
		Fields: map[string]Field{
			"NewValues": {Name: "NewValues", Type: "Map<Id,Account>"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeTypedSetsAndNestedCollections(t *testing.T) {
	program, err := CompileAnonymous(`
Type setType = Type.forName('Set<String>');
Set<String> tags = JSON.deserialize('["red","blue","red",null]', setType);
System.assertEquals(3, tags.size());
System.assert(tags.contains('red'));
System.assert(tags.contains('blue'));
System.assert(tags.contains(null));

Type nestedType = Type.forName('Map<String,List<Set<Integer>>>');
Map<String,List<Set<Integer>>> nested = JSON.deserialize('{"one":[[1,2,2],[3,null]],"empty":[]}', nestedType);
System.assertEquals(2, nested.get('one').size());
System.assertEquals(2, nested.get('one').get(0).size());
System.assert(nested.get('one').get(0).contains(1));
System.assert(nested.get('one').get(0).contains(2));
System.assert(nested.get('one').get(1).contains(3));
System.assert(nested.get('one').get(1).contains(null));
System.assertEquals(0, nested.get('empty').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeApexClassUsesFieldDeclarationOrder(t *testing.T) {
	program, err := CompileAnonymous(`
OrderedPayload payload = new OrderedPayload();
payload.parameters = new List<String>{'one'};
payload.failureReason = 'bad';
payload.failureCode = 'Unauthorized';
payload.trigger = 'Manual';
payload.status = 'Failed';
payload.completed = 'done';
payload.started = 'start';
payload.source = 'Caqh';
payload.providerId = 'provider';
payload.id = 'id';
System.assertEquals('{"parameters":["one"],"failureReason":"bad","failureCode":"Unauthorized","trigger":"Manual","status":"Failed","completed":"done","started":"start","source":"Caqh","providerId":"provider","id":"id"}', JSON.serialize(payload));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "OrderedPayload",
		Fields: map[string]Field{
			"parameters":    {Name: "parameters", Type: "List<String>"},
			"failureReason": {Name: "failureReason", Type: "String"},
			"failureCode":   {Name: "failureCode", Type: "String"},
			"trigger":       {Name: "trigger", Type: "String"},
			"status":        {Name: "status", Type: "String"},
			"completed":     {Name: "completed", Type: "String"},
			"started":       {Name: "started", Type: "String"},
			"source":        {Name: "source", Type: "String"},
			"providerId":    {Name: "providerId", Type: "String"},
			"id":            {Name: "id", Type: "String"},
		},
		FieldOrder: []string{"parameters", "failureReason", "failureCode", "trigger", "status", "completed", "started", "source", "providerId", "id"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeTypedApexClassNestedFields(t *testing.T) {
	program, err := CompileAnonymous(`
JsonPerson person = JSON.deserialize('{"ExternalId":"E-7","Name":"Ada","Primary":{"City":"Delta","Zip":99501},"Addresses":[{"City":"Port","Zip":1},{"City":"Lake","Zip":2}],"AddressBook":{"home":{"City":"Cabin","Zip":3}},"Tags":["north","north","south"],"Scores":{"math":9,"trail":10},"OptionalAddress":null}', JsonPerson.class);
System.assertEquals('E-7', person.ExternalId);
System.assertEquals('Ada', person.Name);
System.assertEquals('Delta', person.Primary.City);
System.assertEquals(99501, person.Primary.Zip);
System.assertEquals(2, person.Addresses.size());
JsonAddress lake = person.Addresses.get(1);
System.assertEquals('Lake', lake.City);
JsonAddress home = person.AddressBook.get('home');
System.assertEquals('Cabin', home.City);
System.assertEquals(2, person.Tags.size());
System.assert(person.Tags.contains('north'));
System.assert(person.Tags.contains('south'));
System.assertEquals(10, person.Scores.get('trail'));
System.assertEquals(null, person.OptionalAddress);
System.assertEquals(null, person.Missing);

JSONParser parser = JSON.createParser('{"ExternalId":"E-9","Name":"Parser","Primary":{"City":"Root","Zip":1}}');
JsonPerson parsed = (JsonPerson)parser.readValueAs(JsonPerson.class);
System.assertEquals('E-9', parsed.ExternalId);
System.assertEquals('Parser', parsed.Name);
System.assertEquals('Root', parsed.Primary.City);

Boolean strictCaught = false;
try {
	JsonPerson bad = JSON.deserializeStrict('{"ExternalId":"E-8","Nope":"x"}', JsonPerson.class);
} catch (JSONException e) {
	strictCaught = true;
	System.assert(e.getMessage().contains('unknown field "Nope"'));
}
System.assert(strictCaught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "JsonBase",
		Fields: map[string]Field{
			"ExternalId": {Name: "ExternalId", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "JsonAddress",
		Fields: map[string]Field{
			"City": {Name: "City", Type: "String"},
			"Zip":  {Name: "Zip", Type: "Integer"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "JsonPerson",
		SuperClass: "JsonBase",
		Fields: map[string]Field{
			"Name":            {Name: "Name", Type: "String"},
			"Primary":         {Name: "Primary", Type: "JsonAddress"},
			"Addresses":       {Name: "Addresses", Type: "List<JsonAddress>"},
			"AddressBook":     {Name: "AddressBook", Type: "Map<String,JsonAddress>"},
			"Tags":            {Name: "Tags", Type: "Set<String>"},
			"Scores":          {Name: "Scores", Type: "Map<String,Integer>"},
			"OptionalAddress": {Name: "OptionalAddress", Type: "JsonAddress"},
			"Missing":         {Name: "Missing", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestTypedValueFromJSONResolvesNestedFieldTypes(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "SetupDataMapping.Rows",
		Fields: map[string]Field{
			"sfField": {Name: "sfField", Type: "String"},
			"tpField": {Name: "tpField", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "SetupDataMapping.License",
		Fields: map[string]Field{
			"rows": {Name: "rows", Type: "List<Rows>"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.typedValueFromJSON("SetupDataMapping.License", map[string]any{
		"rows": []any{
			map[string]any{"sfField": "Name", "tpField": "name"},
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	rows := value.Fields["rows"]
	if rows.Kind != ValueList || len(rows.List) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	if got := rows.List[0].Type; got != "SetupDataMapping.Rows" {
		t.Fatalf("row type = %q", got)
	}
	if got := rows.List[0].Fields["sfField"].Text; got != "Name" {
		t.Fatalf("sfField = %q", got)
	}
}

func TestExecJSONDeserializeDatabaseResultDTOs(t *testing.T) {
	program, err := CompileAnonymous(`
Database.SaveResult saved = JSON.deserialize('{"success":false,"id":null,"errors":[{"statusCode":"UNABLE_TO_LOCK_ROW","message":"locked","fields":["Name"]}]}', Database.SaveResult.class);
System.assert(!saved.isSuccess());
System.assertEquals(null, saved.getId());
System.assertEquals(1, saved.getErrors().size());
System.assertEquals('locked', saved.getErrors()[0].getMessage());
System.assertEquals('UNABLE_TO_LOCK_ROW', String.valueOf(saved.getErrors()[0].getStatusCode()));
System.assertEquals('Name', saved.getErrors()[0].getFields()[0]);

Database.DeleteResult deleted = JSON.deserialize('{"success":true,"id":"001B000001DVM9t","errors":[]}', Database.DeleteResult.class);
System.assert(deleted.isSuccess());
System.assertEquals('001B000001DVM9t', deleted.getId());
System.assertEquals(0, deleted.getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeTypedMappingErrorsAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	Type mapType = Type.forName('Map<String,Integer>');
	Object value = JSON.deserialize('{"bad":"not-a-number"}', mapType);
} catch (JSONException e) {
	caught = e.getMessage();
}
System.assert(caught.contains('JSON.deserialize cannot map JSON String to Integer'));

String keyCaught = '';
try {
	Type mapType = Type.forName('Map<List<String>,String>');
	Object value = JSON.deserialize('{"1":"one"}', mapType);
} catch (JSONException e) {
	keyCaught = e.getMessage();
}
System.assert(keyCaught.contains('Map keys only for scalar/String/Object targets'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeAcceptsApexEscapedDoubleQuotes(t *testing.T) {
	program, err := CompileAnonymous(`
String body = '{\"Name\":\"Acme\"}';
Account decoded = JSON.deserialize(body, Account.class);
System.assertEquals('Acme', decoded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectUsesSchemaFieldTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserialize('{"Name":"Acme","RenewalDate__c":"2024-02-29","AnnualRevenue":12.5,"LastSeen__c":"2024-02-29T12:34:56Z","Score__c":7,"Active__c":true,"ParentId":"001B000001DVM9t"}', Account.class);
Date renewal = decoded.RenewalDate__c;
System.assertEquals(Date.newInstance(2024, 2, 29), renewal);
Decimal revenue = decoded.AnnualRevenue;
System.assertEquals(12.5, revenue);
Datetime lastSeen = decoded.LastSeen__c;
System.assertEquals(Datetime.newInstance(2024, 2, 29, 12, 34, 56), lastSeen);
Integer score = decoded.Score__c;
System.assertEquals(7, score);
Boolean active = decoded.Active__c;
System.assertEquals(true, active);
Id parent = decoded.ParentId;
System.assertEquals('001B000001DVM9t', parent.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	account.Definition.Fields["LastSeen__c"] = storage.Field{APIName: "LastSeen__c", Type: storage.FieldDateTime}
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Type: storage.FieldBoolean}
	account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectAllowsFabricatedIdValue(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserialize('{"Id":"id-1","Name":"Acme"}', Account.class);
System.assertEquals('id-1', decoded.Id);
System.assertEquals('Acme', decoded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectAllowsFabricatedReferenceValue(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserialize('{"ParentId":"Manual","Name":"Acme"}', Account.class);
System.assertEquals('Manual', decoded.ParentId);
System.assertEquals('Acme', decoded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectChildRelationshipRecords(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserialize('{"Name":"Acme","NumberOfEmployees":"7","Contacts":{"totalSize":"2","done":"true","records":[{"attributes":{"type":"Contact"},"LastName":"One","DoNotCall":"true"},{"attributes":{"type":"Contact"},"LastName":"Two","DoNotCall":"false"}]}}', Account.class);
Integer employees = decoded.NumberOfEmployees;
System.assertEquals(7, employees);
List<Contact> contacts = decoded.Contacts;
System.assertEquals(2, contacts.size());
System.assertEquals('One', contacts[0].LastName);
System.assertEquals(true, contacts[0].DoNotCall);
System.assertEquals('Two', contacts[1].LastName);
System.assertEquals(false, contacts[1].DoNotCall);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["NumberOfEmployees"] = storage.Field{APIName: "NumberOfEmployees", Type: storage.FieldInteger}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Contact",
			Label:       "Contact",
			PluralLabel: "Contacts",
			KeyPrefix:   "003",
			Fields:      map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}, "DoNotCall": {APIName: "DoNotCall", Type: storage.FieldBoolean}, "AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"}},
			Relations:   []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeCustomChildRelationshipRecordsUseChildType(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c decoded = JSON.deserialize('{"Children__r":{"totalSize":1,"done":true,"records":[{"Parent__c":"001000000000001AAA"}]}}', Parent__c.class);
System.assertEquals(null, decoded.Children__r[0].Parent__r.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName:   "Account",
				KeyPrefix: "001",
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a01",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":           {APIName: "Id", Type: storage.FieldID},
					"Container__c": {APIName: "Container__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Children"},
					"Parent__c":    {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Accounts"},
				},
				Relations: []storage.Relationship{
					{
						Field:              "Container__c",
						ParentObjects:      []string{"Parent__c"},
						ParentRelationship: "Children",
						ChildRelationship:  "Children__r",
					},
					{
						Field:              "Parent__c",
						ParentObjects:      []string{"Account"},
						ParentRelationship: "Accounts",
					},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeNamespacedChildRecordParentShellUsesUnqualifiedLookup(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c decoded = JSON.deserialize('{"Children__r":{"totalSize":1,"done":true,"records":[{"Product2__c":"a02000000000001AAA"}]}}', Parent__c.class);
System.assertEquals(null, decoded.Children__r[0].Product2__r.Event2__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a01",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Product__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Product__c",
				KeyPrefix: "a02",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Event2__c": {APIName: "Event2__c", Type: storage.FieldReference, ReferenceTo: []string{"Event__c"}, RelationshipName: "Event2__r"},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"NU__Container__c": {APIName: "NU__Container__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Children"},
					"NU__Product2__c":  {APIName: "NU__Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product2__r"},
				},
				Relations: []storage.Relationship{
					{
						Field:              "NU__Container__c",
						ParentObjects:      []string{"Parent__c"},
						ParentRelationship: "Children",
						ChildRelationship:  "Children__r",
					},
					{
						Field:              "NU__Product2__c",
						ParentObjects:      []string{"Product__c"},
						ParentRelationship: "Product2__r",
					},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectChildRelationshipFromSerializedMapSupportsGetSObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> payload = new Map<String, Object>();
payload.put('Id', 'a00000000000001AAA');
payload.put('verifiable__EducationLast__r', new Map<String, Object>{
	'records' => new List<SObject>{
		new Education__c(Id = 'a01000000000001AAA', SchoolName__c = 'Test School')
	},
	'totalSize' => 1,
	'done' => true
});
Sanction_Exclusion_Scan__c decoded = (Sanction_Exclusion_Scan__c)JSON.deserialize(JSON.serialize(payload), Sanction_Exclusion_Scan__c.class);
List<Education__c> records = (List<Education__c>)decoded.getSObjects('verifiable__EducationLast__r');
System.assertEquals(1, records.size());
System.assertEquals('Test School', records[0].SchoolName__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.OrgState{Namespace: "verifiable", Objects: map[string]storage.ObjectState{
		"Sanction_Exclusion_Scan__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Sanction_Exclusion_Scan__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Education__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Education__c",
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":                  {APIName: "Id", Type: storage.FieldID},
					"SchoolName__c":       {APIName: "SchoolName__c", Type: storage.FieldString},
					"LastVerification__c": {APIName: "LastVerification__c", Type: storage.FieldReference, ReferenceTo: []string{"Sanction_Exclusion_Scan__c"}, RelationshipName: "LastVerification__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "LastVerification__c",
					ParentObjects:      []string{"Sanction_Exclusion_Scan__c"},
					ParentRelationship: "LastVerification__r",
					ChildRelationship:  "EducationLast",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeStrictRejectsDuplicateFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserializeStrict('{"Name":"First","Name":"Second"}', Account.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), `duplicate field "Name"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONDeserializeAllowsDuplicateFieldsOutsideStrict(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserialize('{"Name":"First","Name":"Second"}', Account.class);
System.assertEquals('Second', decoded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeNonStrictKeepsObjectWhenFieldTargetUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
LooseContainer decoded = JSON.deserialize('{"name":"root","opaque":{"id":"x"}}', LooseContainer.class);
System.assertNotEquals(null, decoded);
System.assertEquals('root', decoded.name);
System.assertNotEquals(null, decoded.opaque);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "LooseContainer",
		Fields: map[string]Field{
			"name":   {Name: "name", Type: "String"},
			"opaque": {Name: "opaque", Type: "MissingTarget"},
		},
		FieldOrder: []string{"name", "opaque"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeStrictRejectsUnknownApexClassFieldsAsJSONException(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	Object decoded = JSON.deserializeStrict('{"Name":"Acme","Extra__c":"x"}', JsonDTO.class);
} catch (JSONException e) {
	caught = e.getMessage();
}
System.assert(caught.contains('unknown field "Extra__c"'));
System.assert(caught.contains('JsonDTO'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "JsonDTO",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeKeepsNonStrictApexClassUnknownFieldBehavior(t *testing.T) {
	program, err := CompileAnonymous(`
Object decoded = JSON.deserialize('{"Name":"Acme","Extra__c":"x"}', JsonDTO.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "JsonDTO",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	decoded := machine.Globals["decoded"]
	if decoded.Type != "JsonDTO" {
		t.Fatalf("decoded.Type = %q, want JsonDTO", decoded.Type)
	}
	if got := decoded.Fields["Extra__c"]; got.Kind != ValueString || got.Text != "x" {
		t.Fatalf("Extra__c = %#v, want string x", got)
	}
}

func TestExecJSONDeserializeTypedRejectsMismatchedShapes(t *testing.T) {
	program, err := CompileAnonymous(`
Object n = JSON.deserialize('"not-a-number"', Integer.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "JSON.deserialize cannot map JSON String to Integer") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONDeserializeTypedRejectsUnsupportedMapKeyTargets(t *testing.T) {
	program, err := CompileAnonymous(`
Type mapType = Type.forName('Map<List<String>,Object>');
Object value = JSON.deserialize('{"1":"one"}', mapType);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "Map keys only for scalar/String/Object targets") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONDeserializeRejectsUnsupportedObjectTargets(t *testing.T) {
	program, err := CompileAnonymous(`
Object value = JSON.deserialize('{"Name":"Acme"}', UnknownJsonShape.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	err = func() error {
		_, execErr := machine.Execute(program)
		return execErr
	}()
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
		t.Fatalf("err = %#v, want UnsupportedFeature", err)
	}
	if runtimeErr.Message != `unsupported call "JSON.deserialize local class/SObject mapping for UnknownJsonShape"` {
		t.Fatalf("message = %q", runtimeErr.Message)
	}
}

func TestExecJSONDeserializeUntypedEdgesAndCatchableMalformedInput(t *testing.T) {
	program, err := CompileAnonymous(`
Object root = JSON.deserializeUntyped('{"name":"Acme","ok":true,"missing":null,"whole":12,"big":9223372036854775808,"ratio":1.25,"items":[1,"two",false,{"inner":null}]}');
System.assertEquals('Acme', root.get('name'));
System.assertEquals(true, root.get('ok'));
System.assertEquals(null, root.get('missing'));
System.assertEquals(12, root.get('whole'));
System.assertEquals(9223372036854775808.0, root.get('big'));
System.assertEquals(1.25, root.get('ratio'));
Object items = root.get('items');
System.assertEquals(4, items.size());
System.assertEquals(1, items.get(0));
System.assertEquals('two', items.get(1));
System.assertEquals(false, items.get(2));
System.assertEquals(null, items.get(3).get('inner'));
Object firstRootOnly = JSON.deserializeUntyped('{"provider":{"sfObject":"User"}},"license":{"sfObject":"Contact"}');
System.assertEquals(true, firstRootOnly.containsKey('provider'));
System.assertEquals(false, firstRootOnly.containsKey('license'));
String caught = '';
try {
	JSON.deserializeUntyped('{"broken":');
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSON.deserializeUntyped invalid JSON input'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeUntypedMapPreservesIdLikeStringKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> values = (Map<String, Object>)JSON.deserializeUntyped('{"positiveParameters":[{"type":"Npi"}],"negativeParameters":[]}');
String encoded = JSON.serialize(values);
System.assert(encoded.contains('positiveParameters'), encoded);
System.assert(encoded.contains('negativeParameters'), encoded);
System.assert(!encoded.contains('positiveparametAAA'), encoded);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeUntypedKeepsEscapedNewlineValid(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> values = (Map<String, Object>)JSON.deserializeUntyped('{"address":"796 N ST\\nLORTON VA 22079"}');
String address = (String)values.get('address');
System.assert(address.contains('\n'));
String encoded = JSON.serialize(values);
System.assert(encoded.contains('\\n'), encoded);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeDatetimeIncludesMillisecondsAndRejectsSObjectField(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('"2024-02-29T12:34:56.000Z"', JSON.serialize(Datetime.newInstanceGmt(2024, 2, 29, 12, 34, 56)));
Boolean caught = false;
try {
	JSON.serialize(Account.Description, false);
} catch (Exception e) {
	caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONMalformedDeserializeErrorsAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	Account decoded = JSON.deserialize('{"Name":', Account.class);
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:Unexpected end-of-input'), caught);
caught = '';
try {
	Account decoded = JSON.deserialize('}]', Account.class);
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:malformed JSON:'), caught);
caught = '';
try {
	Account decoded = JSON.deserializeStrict('{"Name":"First","Name":"Second"}', Account.class);
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSON.deserializeStrict found duplicate field "Name"'), caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializePrettyAndGeneratorOutputEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> root = new Map<String,Object>();
root.put('name', 'Acme');
root.put('scriptName', 'X');
root.put('missing', null);
List<Object> items = new List<Object>();
items.add(1);
items.add(null);
root.put('items', items);
root.put('webhooks', new List<String>{ 'DatasetScanMatchesChanged' });
String compact = JSON.serialize(root, true);
System.assertEquals('{"name":"Acme","scriptName":"X","missing":null,"items":[1,null],"webhooks":["DatasetScanMatchesChanged"]}', compact);
String pretty = JSON.serializePretty(root, true);
System.assert(pretty.contains('  "items" : [ 1, null ]'));
System.assert(pretty.contains('  "scriptName" : "X"'));
System.assert(pretty.contains('"webhooks" : [ "DatasetScanMatchesChanged" ]'));
System.assert(pretty.contains('  "missing" : null'));
System.assert(pretty.contains('  "name" : "Acme"'));
Account account = new Account(Name = 'NoNull', Phone = null);
System.assert(!JSON.serializePretty(account, true).contains('Phone'));
System.assert(JSON.serializePretty(account, false).contains('"Phone" : null'));

JSONGenerator gen = JSON.createGenerator(true);
gen.writeStartObject();
gen.writeObjectField('root', root);
gen.writeRawField('raw', '{"ok":true}');
gen.writeEndObject();
String generated = gen.getAsString();
System.assert(generated.contains('  "root": {"name":"Acme","scriptName":"X","missing":null,"items":[1,null],"webhooks":["DatasetScanMatchesChanged"]}'));
System.assert(generated.contains('"raw": {"ok":true}'));
System.assert(gen.isClosed());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONParserAndTokenRemainingEdges(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('[{"id":"001B000001DVM9t","blob":"YWJj"},false]');
System.assertEquals(JSONToken.START_ARRAY, parser.nextToken());
System.assertEquals(JSONToken.START_OBJECT, parser.nextToken());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('id', parser.getCurrentName());
System.assertEquals(JSONToken.VALUE_STRING, parser.nextValue());
System.assertEquals('001B000001DVM9t', parser.getIdValue().toString());
System.assertEquals(JSONToken.FIELD_NAME, parser.nextToken());
System.assertEquals('blob', parser.getCurrentName());
System.assertEquals(JSONToken.VALUE_STRING, parser.nextValue());
System.assertEquals('abc', parser.getBlobValue().toString());
System.assertEquals(JSONToken.END_OBJECT, parser.nextToken());
System.assertEquals(JSONToken.VALUE_FALSE, parser.nextToken());
System.assertEquals(false, parser.getBooleanValue());
System.assertEquals(JSONToken.END_ARRAY, parser.nextToken());
System.assertEquals(null, parser.nextToken());
System.assertEquals(null, parser.getCurrentToken());
System.assertEquals(null, parser.getCurrentName());
System.assertEquals('START_OBJECT', JSONToken.START_OBJECT.name());
System.assertEquals('START_OBJECT', JSONToken.START_OBJECT.toString());
System.assertEquals(0, JSONToken.START_OBJECT.ordinal());
System.assertEquals(JSONToken.VALUE_NUMBER_FLOAT, JSONToken.VALUE_NUMBER_FLOAT);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONGeneratorWritesPlatformAndObjectValues(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator gen = JSON.createGenerator(false);
gen.writeStartObject();
gen.writeDateField('date', Date.newInstance(2024, 2, 29));
gen.writeDateTimeField('when', Datetime.newInstance(2024, 2, 29, 12, 34, 56));
gen.writeTimeField('clock', Time.newInstance(5, 6, 7, 0));
gen.writeIdField('id', Id.valueOf('001B000001DVM9t'));
gen.writeBlobField('blob', Blob.valueOf('abc'));
gen.writeFieldName('nested');
List<Object> nested = new List<Object>();
nested.add(true);
gen.writeObject(nested);
gen.writeEndObject();
String text = gen.getAsString();
System.assert(text.contains('"date":"2024-02-29"'));
System.assert(text.contains('"when":"2024-02-29T12:34:56Z"'));
System.assert(text.contains('"clock":"05:06:07"'));
System.assert(text.contains('"id":"001B000001DVM9t"'));
System.assert(text.contains('"blob":"YWJj"'));
System.assert(text.contains('"nested"'));
System.assert(text.contains('[true]'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeSuppressNullEdgesAndNestedMaps(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> nested = new Map<String,Object>();
nested.put('kept', 'yes');
nested.put('missing', null);
List<Object> items = new List<Object>();
items.add(nested);
items.add(null);
Map<String,Object> root = new Map<String,Object>();
root.put('items', items);
String compact = JSON.serialize(root, true);
System.assert(compact.contains('"items":[{'));
System.assert(compact.contains('"kept":"yes"'));
System.assert(compact.contains('"missing":null'));
System.assert(compact.contains('},null]'));
String pretty = JSON.serializePretty(root, true);
System.assert(pretty.contains('  "items" : ['));
System.assert(pretty.contains('      "kept" : "yes"'));
System.assert(pretty.contains('      "missing" : null'));
Account account = new Account(Name = 'NoNull', Phone = null);
String objectSuppressed = JSON.serialize(account, true);
System.assert(objectSuppressed.contains('"Name":"NoNull"'));
System.assert(!objectSuppressed.contains('Phone'));
String objectIncluded = JSON.serialize(account, false);
System.assert(objectIncluded.contains('"Phone":null'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeEnumValueInUntypedMap(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> root = new Map<String,Object>();
root.put('statusCode', System.StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION);
String body = JSON.serialize(root);
System.assertEquals('{"statusCode":"FIELD_CUSTOM_VALIDATION_EXCEPTION"}', body);
Database.Error decoded = (Database.Error)JSON.deserialize(body, Database.Error.class);
System.assertEquals(System.StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION.name(), decoded.getStatusCode().name());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeTypedPropertySetterPersistsReceiverMutation(t *testing.T) {
	getter, err := CompileAnonymous("return backingRows;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous("backingRows = value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
MappingContainer decoded = (MappingContainer)JSON.deserialize('{"rows":[{"tpField":"gender","sfField":"LeadSource"}]}', MappingContainer.class);
System.assertEquals(1, decoded.rows.size());
System.assertEquals('gender', decoded.rows.get(0).tpField);
System.assertEquals('LeadSource', decoded.rows.get(0).sfField);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "MappingRow",
		Fields: map[string]Field{
			"tpField": {Name: "tpField", Type: "String"},
			"sfField": {Name: "sfField", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "MappingContainer",
		Fields: map[string]Field{
			"backingRows": {Name: "backingRows", Type: "List<MappingRow>"},
			"rows": {
				Name:     "rows",
				Type:     "List<MappingRow>",
				Property: true,
				Getter:   &Method{Name: "MappingContainer.rows.get", ClassName: "MappingContainer", ReturnType: "List<MappingRow>", Program: getter},
				Setter:   &Method{Name: "MappingContainer.rows.set", ClassName: "MappingContainer", Params: []Param{{Name: "value", Type: "List<MappingRow>"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
