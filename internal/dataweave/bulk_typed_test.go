package dataweave

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTypedExtendedProtocolRoundTrip(t *testing.T) {
	for _, v := range []TypedValue{
		{Kind: "long", Type: "Long", Text: "9223372036854775807"},
		{Kind: "date", Type: "Date", Text: "2024-02-29"},
		{Kind: "datetime", Type: "Datetime", Text: "2024-02-29T01:02:03.004Z"},
		{Kind: "time", Type: "Time", Text: "01:02:03.004"},
		{Kind: "blob", Type: "Blob", Text: "AP+A"},
	} {
		encoded, err := encodeTyped(v)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeTyped(encoded)
		if err != nil || !reflect.DeepEqual(v, decoded) {
			t.Fatalf("roundtrip=%+v err=%v want=%+v", decoded, err, v)
		}
	}
	for _, v := range []TypedValue{{Kind: "long", Text: "9223372036854775808"}, {Kind: "date", Text: "2023-02-29"}, {Kind: "time", Text: "24:00:00"}, {Kind: "datetime", Text: "2024-02-29"}, {Kind: "blob", Text: "AA=A"}} {
		if _, err := encodeTyped(v); err == nil {
			t.Fatalf("accepted invalid extended value %+v", v)
		}
	}
}

func TestOfficialEngineExtendedTypedValues(t *testing.T) {
	cp, javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH"), os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("explicit DataWeave engine and Java17 test toolchain required")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx := context.Background()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input      TypedValue
		expr, want string
	}{
		{TypedValue{Kind: "date", Text: "2024-02-28"}, "(payload + |P1D|) as String", `"2024-02-29"`},
		{TypedValue{Kind: "datetime", Text: "2024-02-28T23:59:59Z"}, "(payload + |PT1S|) as String {format: \"yyyy-MM-dd HH:mm:ss\"}", `"2024-02-29 00:00:00"`},
		{TypedValue{Kind: "time", Text: "01:02:03.004"}, "(payload + |PT1S|) as String {format: \"HH:mm:ss.SSS\"}", `"01:02:04.004"`},
		{TypedValue{Kind: "long", Text: "9007199254740991"}, "payload + 2", `9007199254740993`},
	} {
		result, err := rt.Execute(ctx, Request{Name: "ownedExtendedInput", APIVersion: "67.0", Source: "%dw 2.0\ninput payload application/java\noutput application/json indent=false\n---\n" + tc.expr, Inputs: map[string]Input{"payload": {Typed: &tc.input}}})
		if err != nil || string(result.Data) != tc.want {
			t.Fatalf("input=%s result=%+v err=%v want=%s", tc.input.Kind, result, err, tc.want)
		}
	}
	for _, tc := range []struct{ source, kind, want string }{
		{"|2024-02-29|", "date", "2024-02-29"},
		{"|2024-02-29T01:02:03.004Z|", "datetime", "2024-02-29T01:02:03Z"},
		{"|01:02:03.004|", "time", "01:02:03.004"},
		{"\"owned-bytes\" as Binary", "blob", "b3duZWQtYnl0ZXM="},
	} {
		result, err := rt.Execute(ctx, Request{Name: "ownedExtendedOutput", APIVersion: "67.0", Source: "%dw 2.0\noutput application/apex\n---\n" + tc.source})
		if err != nil || result.Typed == nil || result.Typed.Kind != tc.kind || result.Typed.Text != tc.want || len(result.Data) != 0 {
			t.Fatalf("output=%s result=%+v err=%v", tc.kind, result, err)
		}
	}
	_, err := rt.Execute(ctx, Request{Name: "ownedMissingClass", APIVersion: "67.0", Source: "%dw 2.0\noutput application/apex\n---\n{label:\"owned\"}"})
	var engine *EngineError
	if !errors.As(err, &engine) || engine.Phase != "execute" || !strings.Contains(engine.Message, "Need to specify Apex object type with 'as' clause") {
		t.Fatalf("missing class must be engine execute error: %v", err)
	}
}
