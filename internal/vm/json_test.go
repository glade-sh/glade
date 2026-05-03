package vm

import (
	"strings"
	"testing"
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
