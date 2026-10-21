package vm

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Owned named source-67/transport-62 deployment 0AfRL00000iqt3t0AA isolates
// every canonical property. Only null recordIds rejects during deserialize;
// other null lists/numbers retain their empty/zero defaults in both modes.
func TestExecHeaderJSONNullNative(t *testing.T) {
	fields := []struct {
		name, typ, expected string
		initial             any
	}{
		{"entityName", "String", "'Account'", "Account"},
		{"changeType", "String", "'UPDATE'", "UPDATE"},
		{"changeOrigin", "String", "'owned'", "owned"},
		{"transactionKey", "String", "'owned-tx'", "owned-tx"},
		{"sequenceNumber", "Integer", "7", 7},
		{"commitTimestamp", "Long", "1700000000123L", int64(1700000000123)},
		{"commitUser", "String", "'005000000000001AAA'", "005000000000001AAA"},
		{"commitNumber", "Long", "42L", int64(42)},
		{"recordIds", "List<String>", "new List<String>{'001000000000001AAA','001000000000002AAA'}", []string{"001000000000001AAA", "001000000000002AAA"}},
		{"changedFields", "List<String>", "new List<String>{'Name'}", []string{"Name"}},
		{"nulledFields", "List<String>", "new List<String>{'Industry'}", []string{"Industry"}},
		{"diffFields", "List<String>", "new List<String>{'Description'}", []string{"Description"}},
	}
	cells := []string{"baseline", "omitted", "rootNull", "priorComposite", "allNull"}
	for _, field := range fields {
		cells = append(cells, field.name)
	}
	for _, linked := range []bool{false, true} {
		for _, method := range []string{"deserialize", "deserializeStrict"} {
			for _, cell := range cells {
				t.Run(fmt.Sprintf("linked=%t/%s/%s", linked, method, cell), func(t *testing.T) {
					machine := New(nil)
					if linked {
						declared := map[string]Field{}
						for _, field := range fields {
							declared[field.name] = Field{Name: field.name, Type: field.typ}
						}
						if err := machine.RegisterClass(Class{Name: "EventBus.ChangeEventHeader", PlatformType: true, Fields: declared}); err != nil {
							t.Fatal(err)
						}
					}
					payload := map[string]any{}
					for _, field := range fields {
						payload[field.name] = field.initial
					}
					switch cell {
					case "omitted":
						payload = map[string]any{}
					case "rootNull":
						payload = nil
					case "priorComposite":
						payload = map[string]any{"entityName": nil, "sequenceNumber": nil, "commitTimestamp": nil, "recordIds": nil}
					case "allNull":
						for name := range payload {
							payload[name] = nil
						}
					case "baseline":
					default:
						payload[cell] = nil
					}
					encoded, err := json.Marshal(payload)
					if err != nil {
						t.Fatal(err)
					}
					call := fmt.Sprintf("JSON.%s('%s',EventBus.ChangeEventHeader.class)", method, encoded)
					var source strings.Builder
					if cell == "recordIds" || cell == "priorComposite" || cell == "allNull" {
						fmt.Fprintf(&source, `Boolean caught=false; try { Object decoded=%s; } catch(JSONException issue) { caught=true; System.assertEquals('N/A',issue.getMessage()); } System.assert(caught);`, call)
					} else if cell == "rootNull" {
						fmt.Fprintf(&source, "System.assertEquals(null,%s);", call)
					} else {
						fmt.Fprintf(&source, "EventBus.ChangeEventHeader header=(EventBus.ChangeEventHeader)%s;\n", call)
						for _, field := range fields {
							expected := field.expected
							if cell == field.name || cell == "omitted" {
								switch field.typ {
								case "Integer":
									expected = "0"
								case "Long":
									expected = "0L"
								case "List<String>":
									expected = "new List<String>()"
								default:
									expected = "null"
								}
							}
							fmt.Fprintf(&source, "System.assertEquals(%s,header.%s);\n", expected, field.name)
						}
					}
					program, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: "67.0"})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// A same-name source class retains ordinary declared-field JSON behavior.
// This local ownership control does not assert native namespace behavior.
func TestHeaderJSONNullKeepsSourceClassShadow(t *testing.T) {
	machine := New(nil)
	machine.currentMethod.APIVersion = "67.0"
	if err := machine.RegisterClass(Class{Name: "EventBus"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "EventBus.ChangeEventHeader", Fields: map[string]Field{
		"recordIds":       {Name: "recordIds", Type: "List<String>"},
		"commitTimestamp": {Name: "commitTimestamp", Type: "Long"},
	}}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.typedValueFromJSON("EventBus.ChangeEventHeader", jsonTypedInput{value: map[string]any{"recordIds": nil, "commitTimestamp": nil}, root: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.Fields["recordIds"].Kind != ValueNull || value.Fields["commitTimestamp"].Kind != ValueNull {
		t.Fatalf("platform null semantics leaked into source class: %#v", value.Fields)
	}
}

// These local preservation controls keep recursive mapping and JSONParser on
// their previous paths; the native null-isolation packet covers root calls.
func TestJSONHeaderNullKeepsRecursiveAndParserOrigins(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprintf("direct/rootFalse=%t", wrapped), func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = "67.0"
			var raw any = map[string]any{"recordIds": nil, "sequenceNumber": nil, "commitTimestamp": nil, "changedFields": nil}
			if wrapped {
				raw = jsonTypedInput{value: raw, root: false}
			}
			value, err := machine.typedValueFromJSON("EventBus.ChangeEventHeader", raw, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"recordIds", "sequenceNumber", "commitTimestamp", "changedFields"} {
				if value.Fields[field].Kind != ValueNull {
					t.Fatalf("nonroot field %s changed: %#v", field, value.Fields[field])
				}
			}
		})
	}
	for _, origin := range []string{"nested", "readValueAs", "readValueAsStrict"} {
		t.Run(origin, func(t *testing.T) {
			const payload = `{"recordIds":null,"sequenceNumber":null,"commitTimestamp":null,"changedFields":null}`
			var source string
			if origin == "nested" {
				source = fmt.Sprintf(`List<EventBus.ChangeEventHeader> headers=(List<EventBus.ChangeEventHeader>)JSON.deserialize('[%s]',List<EventBus.ChangeEventHeader>.class); EventBus.ChangeEventHeader header=headers[0];`, payload)
			} else {
				source = fmt.Sprintf(`JSONParser parser=JSON.createParser('%s'); parser.nextToken(); EventBus.ChangeEventHeader header=(EventBus.ChangeEventHeader)parser.%s(EventBus.ChangeEventHeader.class);`, payload, origin)
			}
			source += `System.assertEquals(null,header.recordIds); System.assertEquals(null,header.sequenceNumber); System.assertEquals(null,header.commitTimestamp); System.assertEquals(null,header.changedFields);`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJSONHeaderNullKeepsUnobservedShapes(t *testing.T) {
	for _, api := range []string{"", "37.0", "61.0", "63.0", "66.0", "69.0"} {
		for _, field := range []string{"recordIds", "sequenceNumber"} {
			t.Run(api+"/"+field, func(t *testing.T) {
				machine := New(nil)
				machine.currentMethod.APIVersion = api
				if handled, err := machine.skipJSONHeaderNullField("EventBus.ChangeEventHeader", field, nil); handled || err != nil {
					t.Fatalf("unobserved source %q changed: handled=%t err=%v", api, handled, err)
				}
			})
		}
	}
	for _, shape := range []struct {
		name, typ, field string
		value            any
	}{
		{"case variant", "EventBus.ChangeEventHeader", "RecordIds", nil},
		{"lowercase variant", "EventBus.ChangeEventHeader", "recordids", nil},
		{"other property", "EventBus.ChangeEventHeader", "otherIds", nil},
		{"string null", "EventBus.ChangeEventHeader", "entityName", nil},
		{"non-null list", "EventBus.ChangeEventHeader", "recordIds", []any{}},
		{"non-null number", "EventBus.ChangeEventHeader", "sequenceNumber", 1},
		{"other type", "OwnedHeader", "recordIds", nil},
	} {
		t.Run(shape.name, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = "67.0"
			if handled, err := machine.skipJSONHeaderNullField(shape.typ, shape.field, shape.value); handled || err != nil {
				t.Fatalf("unobserved shape changed: handled=%t err=%v", handled, err)
			}
		})
	}
}
