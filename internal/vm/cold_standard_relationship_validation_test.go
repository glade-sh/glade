package vm

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Every construction route reaches relationship methods and isSet with name/token arguments.
// The project test replays the same native rows through anonymous and @IsTest entry points.
func TestColdStandardRelationshipValidationNativeRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/cold_standard_relationship_validation"
	source, err := os.ReadFile(fixture + "/project/force-app/main/default/classes/ColdRelationshipValidationProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "  String payload=")
	end := strings.Index(string(source), "  Map<String,String> result=")
	if start < 0 || end < start {
		t.Fatal("missing probe body")
	}
	construction := string(source[start:end])
	operations := map[string]string{}
	for _, line := range strings.Split(string(source[end:]), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "try {") {
			continue
		}
		_, rest, ok := strings.Cut(line, "result.put('")
		if !ok {
			t.Fatal("operation without observation")
		}
		operation, _, ok := strings.Cut(rest, "'")
		if !ok {
			t.Fatal("operation without name")
		}
		operations[operation] = line
	}
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
			var want map[string]any
			if err := json.Unmarshal([]byte(columns[3]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != len(operations) {
				t.Fatal("incomplete operation matrix")
			}
			for operation, expected := range want {
				body := construction + "Map<String,String> result=new Map<String,String>();\n" + operations[operation] + "\nSystem.debug('RELATIONSHIP_VALIDATION|'+JSON.serialize(result));"
				for _, loaded := range []bool{false, true} {
					state := "cold"
					if loaded {
						state = "loaded"
					}
					t.Run(api+"/"+route+"/"+state+"/"+operation, func(t *testing.T) {
						org := storage.NewOrgState()
						storage.EnsureStandardObject(&org, "Account")
						storage.EnsureStandardObject(&org, "Contact")
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
						payload := strings.TrimPrefix(result.Debug[0], "RELATIONSHIP_VALIDATION|")
						var got map[string]any
						if err := json.Unmarshal([]byte(payload), &got); err != nil {
							t.Fatalf("invalid observation: %s", payload)
						}
						if len(got) != 1 || !reflect.DeepEqual(got[operation], expected) {
							t.Errorf("native=%v; Glade=%v", expected, got[operation])
						}
						if machine.Org.RuntimeSchemaStamp != stamp || machine.Org.RuntimeGenerationID() != generation || machine.Org.SchemaGeneration() != schemaGeneration {
							t.Error("relationship validation changed schema generation")
						}
						if _, present := machine.Org.Objects["Asset"]; present != loaded {
							t.Error("relationship validation installed a cold standard object")
						}
						if _, present := template.Org.Objects["Asset"]; present != loaded {
							t.Error("relationship validation changed the base schema")
						}
					})
				}
			}
		}
	}
}
