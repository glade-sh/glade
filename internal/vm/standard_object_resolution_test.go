package vm

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native project covers these same operations through anonymous and @IsTest entry points.
func TestStandardObjectResolutionKeepsSharedSchemaRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/standard_object_resolution"
	source, err := os.ReadFile(fixture + "/project/force-app/main/default/classes/StandardResolutionProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "  String payload=")
	end := strings.LastIndex(string(source), "\n }\n}")
	if start < 0 || end < start {
		t.Fatal("missing probe body")
	}
	routes, err := os.ReadFile(fixture + "/cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range strings.Fields(string(routes))[1:] {
			t.Run(api+"/"+route, func(t *testing.T) {
				base, template := describeSharePrimedBase(t)
				machine := describeShareClone(base, template)
				if _, present := machine.Org.Objects["Asset"]; present {
					t.Fatal("cold standard object already loaded")
				}
				sharedMap := mapIdentity(describeShareFieldMapOf(t, machine, "Account"))
				stamp, generation, schemaGeneration := machine.Org.RuntimeSchemaStamp, machine.Org.RuntimeGenerationID(), machine.Org.SchemaGeneration()
				childCache, describeCache := machine.jsonChildRelTypeCache, machine.describeShare
				program, err := CompileAnonymousWithOptions("String route='"+route+"';\n"+string(source[start:end]), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				if _, present := machine.Org.Objects["Asset"]; present {
					t.Error("read-only standard object resolution installed a schema object")
				}
				if machine.Org.RuntimeSchemaStamp != stamp || machine.Org.RuntimeGenerationID() != generation || machine.Org.SchemaGeneration() != schemaGeneration {
					t.Error("read-only standard object resolution changed the schema generation")
				}
				if machine.jsonChildRelTypeCache != childCache || machine.describeShare != describeCache {
					t.Error("read-only standard object resolution detached shared metadata caches")
				}
				if mapIdentity(describeShareFieldMapOf(t, machine, "Account")) != sharedMap {
					t.Error("unrelated Account field map was rebuilt")
				}
				sibling := describeShareClone(base, template)
				if _, present := sibling.Org.Objects["Asset"]; present {
					t.Error("standard object leaked into a sibling")
				}
				if mapIdentity(describeShareFieldMapOf(t, sibling, "Account")) != sharedMap {
					t.Error("sibling stopped sharing Account fields")
				}
			})
		}
	}
}

// Cold resolution preserves the relationship behavior of an installed definition.
func TestStandardObjectResolutionColdLoadedRelationshipRoutes(t *testing.T) {
	for _, route := range []struct{ name, source string }{
		{"jsonParent", `SObject row=(SObject)JSON.deserialize('{"attributes":{"type":"Asset"},"Account":{"attributes":{"type":"Account"},"Id":"001000000000001AAA","Name":"parent"}}',SObject.class); System.debug(row.get('AccountId')); System.debug(row.getSObject('Account').get('Name'));`},
		{"jsonUserDefaults", `SObject row=(SObject)JSON.deserialize('{"attributes":{"type":"User"},"Username":"probe@example.invalid"}',SObject.class); System.debug(JSON.serialize(row)); System.debug(row.get('IsActive'));`},
		{"jsonSelfChildren", `SObject row=(SObject)JSON.deserialize('{"attributes":{"type":"Asset"},"ChildAssets":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Asset"},"Name":"child"}]}}',SObject.class); System.debug(row.getSObjects('ChildAssets').size()); System.debug(row.getSObjects('ChildAssets')[0].get('Name'));`},
	} {
		t.Run(route.name, func(t *testing.T) {
			var observations [2][]string
			for i, loaded := range []bool{true, false} {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				if loaded {
					objectName := "Asset"
					if route.name == "jsonUserDefaults" {
						objectName = "User"
					}
					storage.EnsureStandardObject(&org, objectName)
				}
				template := storage.NewRuntimeTemplate(org)
				PrimeRuntimeTemplateSchema(&template)
				base := New(nil)
				base.PrimeMetadataSchema(&template.Org)
				machine := base.CloneRuntime(nil)
				runtimeOrg := template.CloneRuntimeOrg()
				machine.SetRuntimeTemplateOrg(&runtimeOrg)
				program, err := CompileAnonymousWithOptions(route.source, CompileOptions{APIVersion: "67.0"})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("loaded=%t: %v", loaded, err)
				}
				observations[i] = result.Debug
			}
			if !reflect.DeepEqual(observations[0], observations[1]) {
				t.Fatalf("loaded=%v; cold=%v", observations[0], observations[1])
			}
		})
	}
}

func TestStandardObjectResolutionEmptyOrgJSONRoutes(t *testing.T) {
	for _, strict := range []bool{false, true} {
		route := "deserialize"
		if strict {
			route = "deserializeStrict"
		}
		t.Run(route, func(t *testing.T) {
			org := storage.OrgState{}
			machine := New(nil)
			machine.SetOrg(&org)
			value, err := machine.sObjectValueFromJSON(map[string]any{
				"attributes": map[string]any{"type": "Asset"}, "Name": "probe",
			}, strict)
			if err != nil {
				t.Fatal(err)
			}
			if value.Type != "Asset" || value.Fields["Name"].Text != "probe" {
				t.Fatalf("unexpected record: %v", value)
			}
			if org.Objects != nil {
				t.Error("JSON allocated an org object map")
			}
		})
	}
}
