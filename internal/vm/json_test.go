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

func TestExecJSONAliasSupportsDeserializeStrict(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> parsed = (Map<String,Object>)Json.deserializeStrict('{"Name":"Acme"}', Map<String,Object>.class);
System.assertEquals('Acme', (String)parsed.get('Name'));
Map<String,Object> parsedSystem = (Map<String,Object>)System.JSON.deserialize('{"Name":"Trail"}', Map<String,Object>.class);
System.assertEquals('Trail', (String)parsedSystem.get('Name'));
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
Cart__c cart = (Cart__c)JSON.deserialize('{"Data__c":"{}"}', Cart__c.class);
System.assertEquals('{}', (String)cart.get('Data__c'));
Cart__c strictCart = (Cart__c)JSON.deserializeStrict('{"Data__c":"strict"}', Cart__c.class);
System.assertEquals('strict', (String)strictCart.get('Data__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
	System.assertEquals('JSONException', e.getTypeName());
	System.assert(e.getMessage().contains('JSONGenerator is closed'));
}
System.assert(caught);
caught = false;
try {
	gen.writeRawValue('{bad');
} catch (JSONException e) {
	caught = true;
	System.assertEquals('JSONException', e.getTypeName());
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
Object missing = JSON.deserialize('null', String.class);
System.assertEquals(null, missing);
Date dateValue = JSON.deserialize('"2024-02-29"', Date.class);
System.assertEquals(Date.newInstance(2024, 2, 29), dateValue);
Datetime whenValue = JSON.deserialize('"2024-02-29T12:34:56Z"', Datetime.class);
System.assertEquals(Datetime.newInstance(2024, 2, 29, 12, 34, 56), whenValue);
Time timeValue = JSON.deserialize('"05:06:07"', Time.class);
System.assertEquals(Time.newInstance(5, 6, 7, 0), timeValue);
Id idValue = JSON.deserialize('"001B000001DVM9t"', Id.class);
System.assertEquals('001B000001DVM9t', idValue.toString());
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
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
	Type mapType = Type.forName('Map<Integer,String>');
	Object value = JSON.deserialize('{"1":"one"}', mapType);
} catch (JSONException e) {
	keyCaught = e.getMessage();
}
System.assert(keyCaught.contains('Map keys only for String/Object targets'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
Type mapType = Type.forName('Map<Integer,Object>');
Object value = JSON.deserialize('{"1":"one"}', mapType);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "Map keys only for String/Object targets") {
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

func TestExecJSONMalformedDeserializeErrorsAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	Account decoded = JSON.deserialize('{"Name":', Account.class);
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:unexpected EOF'));
caught = '';
try {
	Account decoded = JSON.deserializeStrict('{"Name":"First","Name":"Second"}', Account.class);
} catch (JSONException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assert(caught.contains('JSONException:JSON.deserializeStrict found duplicate field "Name"'));
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
root.put('missing', null);
List<Object> items = new List<Object>();
items.add(1);
items.add(null);
root.put('items', items);
String compact = JSON.serialize(root, true);
System.assertEquals('{"items":[1,null],"missing":null,"name":"Acme"}', compact);
String pretty = JSON.serializePretty(root, true);
System.assert(pretty.contains('  "items": ['));
System.assert(pretty.contains('    1,'));
System.assert(pretty.contains('    null'));
System.assert(pretty.contains('  "missing": null'));
System.assert(pretty.contains('  "name": "Acme"'));
Account account = new Account(Name = 'NoNull', Phone = null);
System.assert(!JSON.serializePretty(account, true).contains('Phone'));
System.assert(JSON.serializePretty(account, false).contains('"Phone": null'));

JSONGenerator gen = JSON.createGenerator(true);
gen.writeStartObject();
gen.writeObjectField('root', root);
gen.writeRawField('raw', '{"ok":true}');
gen.writeEndObject();
String generated = gen.getAsString();
System.assert(generated.contains('  "root": {"items":[1,null],"missing":null,"name":"Acme"}'));
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
System.assert(pretty.contains('  "items": ['));
System.assert(pretty.contains('      "kept": "yes"'));
System.assert(pretty.contains('      "missing": null'));
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
