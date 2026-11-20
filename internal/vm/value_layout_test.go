package vm

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestValueWireRoundTrips(t *testing.T) {
	var exported []string
	typ := reflect.TypeOf(Value{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			exported = append(exported, field.Name)
		}
	}
	wantOrder := []string{"Kind", "Int", "Decimal", "Bool", "Text", "Type", "Static", "Runtime", "Ref", "ExplicitScale", "Fields", "List", "Set", "Map", "MapKeys", "MapOrder"}
	if !reflect.DeepEqual(exported, wantOrder) {
		t.Fatalf("exported field order = %v, want %v", exported, wantOrder)
	}
	value := Value{Kind: ValueObject, Int: 42, Decimal: 1.25, Bool: true, Text: "text", Type: "Thing", Static: "static", Runtime: "runtime", Ref: 7, ExplicitScale: true,
		Fields: map[string]Value{"a": {Kind: ValueString, Text: "field"}}, List: []Value{{Kind: ValueInt, Int: 1}}, Set: []Value{{Kind: ValueBool, Bool: true}}, Map: map[string]Value{"b": {Kind: ValueInt, Int: 2}}, MapKeys: map[string]Value{"b": {Kind: ValueString, Text: "b"}}, MapOrder: []string{"b"}}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	const wantJSON = `{"kind":"object","int":42,"decimal":1.25,"bool":true,"text":"text","type":"Thing","fields":{"a":{"kind":"string","text":"field"}},"list":[{"kind":"integer","int":1}],"set":[{"kind":"boolean","bool":true}],"map":{"b":{"kind":"integer","int":2}}}`
	if string(encoded) != wantJSON {
		t.Fatalf("JSON = %s, want %s", encoded, wantJSON)
	}
	var jsonValue Value
	if err := json.Unmarshal(encoded, &jsonValue); err != nil {
		t.Fatal(err)
	}
	jsonExpected := value
	jsonExpected.Static, jsonExpected.Runtime, jsonExpected.Ref, jsonExpected.ExplicitScale = "", "", 0, false
	jsonExpected.MapKeys, jsonExpected.MapOrder = nil, nil
	if !reflect.DeepEqual(jsonValue, jsonExpected) {
		t.Fatalf("JSON round trip mismatch: got %#v, want %#v", jsonValue, jsonExpected)
	}
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(value); err != nil {
		t.Fatal(err)
	}
	var gobValue Value
	if err := gob.NewDecoder(&data).Decode(&gobValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gobValue, value) {
		t.Fatalf("gob round trip mismatch: got %#v, want %#v", gobValue, value)
	}
}

var valueFieldsBenchmarkSink map[string]Value

func BenchmarkValueFieldMap(b *testing.B) {
	var keys [128]string
	for i := range keys {
		keys[i] = fmt.Sprintf("Field%d", i)
	}
	b.ReportAllocs()
	for b.Loop() {
		fields := make(map[string]Value, len(keys))
		for _, key := range keys {
			fields[key] = Value{Kind: ValueString, Text: key}
		}
		valueFieldsBenchmarkSink = fields
	}
}
