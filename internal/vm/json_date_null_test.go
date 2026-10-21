package vm

import (
	"fmt"
	"testing"
)

// Existing owned named and anonymous API62/63/67 captures observe the same
// JSONException for a root Date target receiving JSON null. Nested Date nulls
// and the quoted string "null" are handled as separate boundaries.
func TestJSONDeserializeRootDateNullNative(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		for _, isTest := range []bool{false, true} {
			t.Run(fmt.Sprintf("api%s/isTest=%t", api, isTest), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Date ordinary=(Date)JSON.deserialize('"2004-02-29"',Date.class);
System.assertEquals('"2004-02-29"',JSON.serialize(ordinary));
String observed;
try {
    Date value=(Date)JSON.deserialize(JSON.serialize((String)null),Date.class);
    observed='returned';
} catch(Exception problem) {
    observed=problem.getTypeName()+'|'+problem.getMessage();
}
System.assertEquals('System.JSONException|Invalid format: "null"',observed);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if isTest {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Source63 named rows distinguish root JSON null and the exact quoted string
// "null" for both entry points. Recursive Date nulls remain separate controls.
func TestJSONDeserializeRootDateNullNamed63Variants(t *testing.T) {
	for _, cell := range []struct{ name, method, document string }{
		{"quoted", "deserialize", `"null"`},
		{"strict-null", "deserializeStrict", `null`},
		{"strict-quoted", "deserializeStrict", `"null"`},
	} {
		t.Run(cell.name, func(t *testing.T) {
			source := fmt.Sprintf(`
String observed;
try {
    Date value=(Date)JSON.%s('%s',Date.class);
    observed='returned';
} catch(Exception problem) {
    observed=problem.getTypeName()+'|'+problem.getMessage();
}
System.assertEquals('System.JSONException|Invalid format: "null"',observed);
`, cell.method, cell.document)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "63.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJSONRootDateNullKeepsRecursiveAndOtherScalarNulls(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "DateNullEnvelope", APIVersion: api, Fields: map[string]Field{
				"value": {Name: "value", Type: "Date", Property: true, HasGetter: true},
			}}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(`
DateNullEnvelope envelope=(DateNullEnvelope)JSON.deserialize('{"value":null}',DateNullEnvelope.class);
System.assertEquals(null,envelope.value);
List<Date> dates=(List<Date>)JSON.deserialize('[null]',List<Date>.class);
System.assertEquals(1,dates.size());
System.assertEquals(null,dates[0]);
Map<String,Date> byName=(Map<String,Date>)JSON.deserialize('{"owned":null}',Map<String,Date>.class);
System.assertEquals(1,byName.size());
System.assert(byName.containsKey('owned'));
System.assertEquals(null,byName.get('owned'));
System.assertEquals(null,JSON.deserialize('null',String.class));
System.assertEquals(null,JSON.deserialize('null',Integer.class));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// These local preservation controls bound the fix. They make no native parity
// claim about uncaptured source versions, source classes or DateTime nulls.
func TestJSONRootDateNullKeepsUnobservedContexts(t *testing.T) {
	for _, cell := range []struct {
		name, api, target string
		root, sourceClass bool
		value             any
	}{
		{name: "source61", api: "61.0", target: "Date", root: true},
		{name: "source68", api: "68.0", target: "Date", root: true},
		{name: "unknown-source", target: "Date", root: true},
		{name: "recursive", api: "63.0", target: "Date"},
		{name: "recursive-quoted", api: "63.0", target: "Date", value: "null"},
		{name: "datetime", api: "63.0", target: "Datetime", root: true},
		{name: "source-class", api: "63.0", target: "Date", root: true, sourceClass: true},
		{name: "source-class-quoted", api: "63.0", target: "Date", root: true, sourceClass: true, value: "null"},
		{name: "other-text", api: "63.0", target: "Date", root: true, value: "NULL"},
		{name: "whitespace-text", api: "63.0", target: "Date", root: true, value: " null "},
		{name: "numeric", api: "63.0", target: "Date", root: true, value: 0},
	} {
		t.Run(cell.name, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod = Method{APIVersion: cell.api}
			if cell.sourceClass {
				if err := machine.RegisterClass(Class{Name: "Date", APIVersion: cell.api}); err != nil {
					t.Fatal(err)
				}
			}
			if err := machine.jsonRootDateNullError(cell.target, jsonTypedInput{value: cell.value, root: cell.root}); err != nil {
				t.Fatalf("unobserved context changed: %v", err)
			}
		})
	}
}
