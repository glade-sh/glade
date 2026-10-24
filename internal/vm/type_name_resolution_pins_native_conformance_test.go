package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorTypeNamePrecedenceNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if len(row) != 5 || row[1] != "anonymous" || row[3] != "true" {
			continue
		}
		if row[2] != "inner_before_top_level" && row[2] != "project_class_before_platform" {
			continue
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			machine := newTypeNamePinMachine(t, row[0])
			body := "TypeNamePinToken value = new TypeNamePinToken(); System.debug('RESULT|' + value.marker);"
			if row[2] == "project_class_before_platform" {
				body = "URL value = new URL(); System.debug('RESULT|' + value.marker);"
			}
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.ExecuteInClass(program, "NameResolutionPinProbe")
			if err != nil {
				t.Fatal(err)
			}
			got := typeNamePinDebugResult(result.Debug)
			if got != row[4] {
				t.Fatalf("native=%q; VM=%q", row[4], got)
			}
		})
	}
}

func newTypeNamePinMachine(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "NameResolutionPinProbe"}); err != nil {
		t.Fatal(err)
	}
	register := func(name, value string) {
		class := Class{Name: name, APIVersion: api, Fields: map[string]Field{
			"marker": {Name: "marker", Type: "String", Access: "public", InitialValue: String(value)},
		}}
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	register("NameResolutionPinProbe.TypeNamePinToken", "inner")
	register("TypeNamePinToken", "top-level")
	register("URL", "project-url")
	return machine
}

func typeNamePinDebugResult(lines []string) string {
	for _, line := range lines {
		if at := strings.Index(line, "RESULT|"); at >= 0 {
			return line[at+len("RESULT|"):]
		}
	}
	return ""
}
