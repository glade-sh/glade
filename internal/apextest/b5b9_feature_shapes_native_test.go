package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type b5b9CapturedShape struct{ Fields []struct{ Name string } }

func b5b9ShapeBody(object string, fields []string) string {
	var body strings.Builder
	body.WriteString(`Map<String,Schema.SObjectField> f=Schema.getGlobalDescribe().get('` + object + `').getDescribe().fields.getMap();`)
	for _, field := range fields {
		body.WriteString(`System.assert(f.containsKey('` + field + `'),'` + object + `.` + field + `');`)
	}
	body.WriteString(`System.assertEquals(` + strconv.Itoa(len(fields)) + `,f.size(),'` + object + ` count');`)
	return body.String()
}

// Native anonymous describes and independently captured named @IsTest field
// maps are both retained; two DocGen objects have distinct named-only fields.
func TestB5B9FeatureObjectShapeNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	profiles := []struct {
		name, definition string
		snapshots        []string
	}{
		{"gift", "npc-minimal.json", []string{"npc-min"}},
		{"doc", "docgen-minimal.json", []string{"docgen-min", "template"}},
		{"reg", "publicsector.json", []string{"publicsector"}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, profile := range profiles {
			objects := map[string]b5b9CapturedShape{}
			for _, snapshot := range profile.snapshots {
				data, err := os.ReadFile(dir + snapshot + "-" + api + ".json")
				if err != nil {
					t.Fatal(err)
				}
				var captured map[string]*b5b9CapturedShape
				if err := json.Unmarshal(data, &captured); err != nil {
					t.Fatal(err)
				}
				for name, object := range captured {
					if object != nil {
						objects[name] = *object
					}
				}
			}
			named := map[string][]string{}
			if profile.name == "doc" {
				data, err := os.ReadFile(dir + "named-shapes/doc-named-" + api + ".json")
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &named); err != nil {
					t.Fatal(err)
				}
			}
			names := make([]string, 0, len(objects))
			for name := range objects {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, object := range names {
				t.Run(api+"/"+profile.name+"/"+object, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					data, err := os.ReadFile(dir + profile.definition)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
					fields := make([]string, 0, len(objects[object].Fields))
					for _, field := range objects[object].Fields {
						fields = append(fields, field.Name)
					}
					namedFields := fields
					if profile.name == "doc" {
						var ok bool
						namedFields, ok = named[object]
						if !ok {
							t.Fatalf("missing named native row for %s", object)
						}
					}
					b5b9RunShapeRoutes(t, root, api, b5b9ShapeBody(object, fields), b5b9ShapeBody(object, namedFields))
				})
			}
		}
	}
}

func b5b9RunShapeRoutes(t *testing.T, root, api, anonymous, named string) {
	t.Helper()
	for name, body := range map[string]string{
		"OwnedShapeNamed": "public class OwnedShapeNamed {public static void probe(){" + named + "}}",
		"OwnedShapeTest":  "@IsTest private class OwnedShapeTest {@IsTest static void probe(){" + named + "}}",
	} {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, body)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("named compile: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	analysis = sema.AnalyzeAnonymous(index, anonymous, api)
	if analysis.HasErrors() {
		t.Fatalf("anonymous compile: %v", analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	namedProgram, err := vm.CompileAnonymousWithOptions("OwnedShapeNamed.probe();", vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"cold", "warm"} {
		t.Run(state, func(t *testing.T) {
			if _, err := runner.execute(program); err != nil {
				t.Fatalf("anonymous: %v", err)
			}
			if _, err := runner.execute(namedProgram); err != nil {
				t.Fatalf("named: %v", err)
			}
			result := Run(index, Options{SelectedClasses: []string{"OwnedShapeTest"}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
			if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("test runner: %+v", result.Summary())
			}
		})
	}
}
