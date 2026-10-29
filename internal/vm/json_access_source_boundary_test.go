package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Native rows cover persisted source classes and both JSON operations. The
// JsonAccess annotation contract defines 'never' independently of namespace or
// package membership (Apex Developer Guide, JsonAccess Annotation).
func TestJSONNeverAccessSourceContexts(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/json_access_runtime_boundaries/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[1] != "anonymous" {
			continue
		}
		var native map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatal(err)
		}
		for _, namespace := range []string{"", "sample"} {
			for _, dependency := range []bool{false, true} {
				for _, self := range []bool{false, true} {
					name := row[0] + "/" + namespace
					if dependency {
						name += "/dependency"
					}
					if self {
						name += "/self"
					}
					t.Run(name, func(t *testing.T) {
						machine := New(nil)
						for _, class := range []Class{
							{Name: "RuntimeDeniedPayload", Namespace: namespace, Dependency: dependency, APIVersion: row[0], Access: "global", Modifiers: []string{"JsonAccess(serializable='never' deserializable='never')"}},
							{Name: "RuntimeBoundaryCaller", Namespace: namespace, APIVersion: row[0], Access: "public"},
						} {
							if err := machine.RegisterClass(class); err != nil {
								t.Fatal(err)
							}
						}
						var body strings.Builder
						for _, operation := range []struct{ key, expression string }{
							{"blockedDecode", "JSON.deserialize('{}', RuntimeDeniedPayload.class)"},
							{"blockedEncode", "JSON.serialize(new RuntimeDeniedPayload())"},
						} {
							var expected string
							if err := json.Unmarshal(native[operation.key], &expected); err != nil {
								t.Fatal(err)
							}
							body.WriteString("{ Boolean rejected=false; try { " + operation.expression + "; } catch (Exception e) { rejected=true; System.assertEquals('" + expected + "', e.getTypeName()+'|'+e.getMessage()); } System.assert(rejected); }\n")
						}
						program, err := CompileAnonymousWithOptions(body.String(), CompileOptions{APIVersion: row[0]})
						if err != nil {
							t.Fatal(err)
						}
						caller := "RuntimeBoundaryCaller"
						if self {
							caller = "RuntimeDeniedPayload"
						}
						if namespace != "" {
							caller = namespace + "." + caller
						}
						if _, err := machine.ExecuteInClass(program, caller); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
