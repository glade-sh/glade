package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Every construction route reaches dynamic get/put with name and field-token arguments.
// The project test replays the same native rows through anonymous and @IsTest entry points.
func TestColdStandardFieldValidationNativeRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/cold_standard_field_validation"
	source, err := os.ReadFile(fixture + "/project/force-app/main/default/classes/ColdFieldValidationProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "  String payload=")
	end := strings.LastIndex(string(source), "\n }\n}")
	if start < 0 || end < start {
		t.Fatal("missing probe body")
	}
	body := strings.Replace(string(source[start:end]), "return JSON.serialize(result);", "System.debug('FIELD_VALIDATION|'+JSON.serialize(result));", 1)
	for _, api := range []string{"62.0", "67.0"} {
		table, err := os.ReadFile(fixture + "/native/api" + api[:2] + "/org.tsv")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range strings.Split(strings.TrimSpace(string(table)), "\n")[1:] {
			columns := strings.SplitN(row, "\t", 4)
			if len(columns) != 4 || columns[0] != api {
				t.Fatalf("invalid native row: %s", row)
			}
			if columns[1] != "anonymous" {
				continue
			}
			route := columns[2]
			for _, loaded := range []bool{false, true} {
				state := "cold"
				if loaded {
					state = "loaded"
				}
				t.Run(api+"/"+route+"/"+state, func(t *testing.T) {
					org := storage.NewOrgState()
					storage.EnsureStandardObject(&org, "Account")
					if loaded {
						storage.EnsureStandardObject(&org, "Asset")
					}
					template := storage.NewRuntimeTemplate(org)
					PrimeRuntimeTemplateSchema(&template)
					base := New(nil)
					base.PrimeMetadataSchema(&template.Org)
					machine := base.CloneRuntime(nil)
					runtimeOrg := template.CloneRuntimeOrg()
					machine.SetRuntimeTemplateOrg(&runtimeOrg)
					stamp, generation, schemaGeneration := machine.Org.RuntimeSchemaStamp, machine.Org.RuntimeGenerationID(), machine.Org.SchemaGeneration()
					program, err := CompileAnonymousWithOptions("String route='"+route+"';\n"+body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 {
						t.Fatalf("missing observation: %v", result.Debug)
					}
					payload := strings.TrimPrefix(result.Debug[0], "FIELD_VALIDATION|")
					var got, want map[string]string
					if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil {
						t.Fatalf("invalid observation: %s", payload)
					}
					for operation, expected := range want {
						t.Run(operation, func(t *testing.T) {
							if got[operation] != expected {
								t.Errorf("native=%q; Glade=%q", expected, got[operation])
							}
						})
					}
					if len(got) != len(want) {
						t.Errorf("observation keys: got=%v; want=%v", got, want)
					}
					if machine.Org.RuntimeSchemaStamp != stamp || machine.Org.RuntimeGenerationID() != generation || machine.Org.SchemaGeneration() != schemaGeneration {
						t.Error("field validation changed schema generation")
					}
					if _, present := machine.Org.Objects["Asset"]; present != loaded {
						t.Error("field validation installed a cold standard object")
					}
					if _, present := template.Org.Objects["Asset"]; present != loaded {
						t.Error("field validation changed the base schema")
					}
				})
			}
		}
	}
}
