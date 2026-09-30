package dataweave

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTypedProtocolRejectsAmbiguousOrUnboundedValues(t *testing.T) {
	for _, value := range []TypedValue{{Kind: "decimal", Text: "NaN"}, {Kind: "decimal", Text: "1/2"}, {Kind: "integer", Text: "2147483648"}, {Kind: "object"}, {Kind: "map", Fields: []TypedField{{Name: "x", Value: TypedValue{Kind: "null"}}, {Name: "x", Value: TypedValue{Kind: "null"}}}}, {Kind: "blob", Text: "!invalid-base64!"}} {
		if _, err := encodeTyped(value); err == nil {
			t.Fatalf("accepted invalid typed value %+v", value)
		}
	}
	// Craft output containing two valid null-valued fields with the same name.
	// Do not use encodeTyped here: its input validation would reject the packet.
	var duplicate bytes.Buffer
	duplicate.WriteByte(6) // map
	writeField(&duplicate, nil)
	binary.Write(&duplicate, binary.BigEndian, uint32(2))
	for i := 0; i < 2; i++ {
		writeField(&duplicate, []byte("x"))
		duplicate.WriteByte(0) // null
		writeField(&duplicate, nil)
	}
	if _, err := decodeTyped(duplicate.Bytes()); err == nil {
		t.Fatal("accepted duplicate typed output fields")
	}
	value := TypedValue{Kind: "null"}
	for i := 0; i < 66; i++ {
		value = TypedValue{Kind: "list", Elements: []TypedValue{value}}
	}
	if _, err := encodeTyped(value); err == nil {
		t.Fatal("accepted excessive nested typed value")
	}
	if _, err := decodeTyped([]byte{255, 0, 0, 0, 0}); err == nil {
		t.Fatal("accepted unknown output kind")
	}
	if _, err := encodeRequest(Request{Inputs: map[string]Input{"payload": {Data: []byte("raw"), Typed: &TypedValue{Kind: "null"}}}}); err == nil {
		t.Fatal("accepted ambiguous raw/typed input")
	}
}

func TestOfficialEngineTypedApexInputsAndOutput(t *testing.T) {
	cp, javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH"), os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("explicit DataWeave engine and Java17 test toolchain required")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx := context.Background()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	integer := func(n string) TypedValue { return TypedValue{Kind: "integer", Type: "Integer", Text: n} }
	tests := []struct {
		name, expression, want string
		input                  Input
	}{
		{"integer", "payload + 2", "7", Input{Typed: &TypedValue{Kind: "integer", Type: "Integer", Text: "5"}}},
		{"decimal", "payload * 2", "2.5", Input{Typed: &TypedValue{Kind: "decimal", Type: "Decimal", Text: "1.25"}}},
		{"boolean", "not payload", "false", Input{Typed: &TypedValue{Kind: "boolean", Type: "Boolean", Boolean: true}}},
		{"null", "payload default \"fallback\"", "\"fallback\"", Input{Typed: &TypedValue{Kind: "null"}}},
		{"rawString", "payload default \"fallback\"", "\"owned\"", Input{Data: []byte("owned")}},
		{"list", "sum(payload map ($ * 2))", "10", Input{Typed: &TypedValue{Kind: "list", Type: "List<Integer>", Elements: []TypedValue{integer("2"), integer("3")}}}},
		{"map", "payload.left + payload.right", "7", Input{Typed: &TypedValue{Kind: "map", Type: "Map<String,Object>", Fields: []TypedField{{Name: "left", Value: integer("3")}, {Name: "right", Value: integer("4")}}}}},
		{"account", "upper(payload.Name)", "\"FIRST OWNED\"", Input{Typed: &TypedValue{Kind: "object", Type: "Account", Fields: []TypedField{{Name: "Name", Value: TypedValue{Kind: "string", Type: "String", Text: "first owned"}}}}}},
		{"preciseDecimal", "payload * 2", "2469135780246913578024691356.25", Input{Typed: &TypedValue{Kind: "decimal", Type: "Decimal", Text: "1234567890123456789012345678.125"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := rt.Execute(ctx, Request{Name: "ownedTyped" + test.name, APIVersion: "67.0", Source: "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\n" + test.expression, Inputs: map[string]Input{"payload": test.input}})
			if err != nil || string(result.Data) != test.want {
				t.Fatalf("result=%+v err=%v want=%s", result, err, test.want)
			}
		})
	}
	result, err := rt.Execute(ctx, Request{Name: "gladeC5TypedApex", APIVersion: "67.0", Source: "%dw 2.0\ninput records application/csv\noutput application/apex\n---\nrecords map(record) -> {Name: record.name, AnnualRevenue: record.revenue as Number} as Object {class: \"Account\"}\n", Inputs: map[string]Input{"records": {Data: []byte("name,revenue\nFirst Owned,1.25\nSecond Owned,2.5")}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Typed == nil || result.Typed.Kind != "list" || len(result.Typed.Elements) != 2 || len(result.Data) != 0 {
		t.Fatalf("typed output=%+v", result)
	}
	for i, expected := range []struct{ name, revenue string }{{"First Owned", "1.25"}, {"Second Owned", "2.5"}} {
		value := result.Typed.Elements[i]
		if value.Kind != "object" || value.Type != "Account" {
			t.Fatalf("account type=%+v", value)
		}
		fields := map[string]TypedValue{}
		for _, field := range value.Fields {
			fields[field.Name] = field.Value
		}
		if fields["Name"].Kind != "string" || fields["Name"].Text != expected.name || fields["AnnualRevenue"].Kind != "decimal" || fields["AnnualRevenue"].Text != expected.revenue {
			t.Fatalf("account fields=%+v", fields)
		}
	}
	// Type conversion lacking an admitted mapping must be a local host failure.
	_, err = rt.Execute(ctx, Request{Name: "unprovedPeriodOutput", APIVersion: "67.0", Source: "%dw 2.0\noutput application/apex\n---\n|P1D|"})
	var host *HostError
	if !errors.As(err, &host) || host.Kind != "unsupported-typed-output" {
		t.Fatalf("unproved output=%v", err)
	}
}

func TestOfficialEngineUnprovedJavaOutputIsHostError(t *testing.T) {
	cp, javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH"), os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("explicit DataWeave engine and Java17 test toolchain required")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx := context.Background()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"%dw 2.0\noutput application/java\n---\n7", "%dw 2.0\noutput application/json\n---\nwrite(7, \"application/java\")"} {
		result, err := rt.Execute(ctx, Request{Name: "ownedJavaOutput", APIVersion: "67.0", Source: source})
		var host *HostError
		if !errors.As(err, &host) || host.Kind != "unsupported-output-format" {
			t.Errorf("Java output must not expose transport bytes: result=%+v err=%v", result, err)
		}
		if len(result.Data) != 0 || result.Typed != nil {
			t.Errorf("unsupported output returned data: %+v", result)
		}
	}
}
