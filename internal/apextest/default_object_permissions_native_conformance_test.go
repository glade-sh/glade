package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

const defaultPermissionFixture = "../vm/testdata/default_object_permissions"

func TestDefaultObjectPermissionsNativeDMLRoutes(t *testing.T) {
	testDefaultObjectPermissionsMatrix(t, []string{"userFieldInsert", "inlineUserFieldInsert", "userFieldUpdate", "inlineUserFieldUpdate"})
}

func testDefaultObjectPermissionsMatrix(t *testing.T, fields []string) {
	t.Helper()
	var fixture struct {
		Files                  map[string]string `json:"files"`
		NativeObservationCount int               `json:"nativeObservationCount"`
	}
	raw, err := os.ReadFile(filepath.Join(defaultPermissionFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.NativeObservationCount != 39 || len(fixture.Files) != 13 {
		t.Fatal("incomplete native permission sources")
	}
	readSource := func(name string) string {
		t.Helper()
		source, ok := fixture.Files[name]
		if !ok {
			t.Fatalf("missing native source %s", name)
		}
		return source
	}

	f, err := os.Open(filepath.Join(defaultPermissionFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 40 || strings.Join(rows[0], "|") != "source_api|configuration|route|actor|observation" {
		t.Fatal("incomplete native permission matrix")
	}
	seen := map[string]bool{}
	wants := map[string]map[string]any{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:4], "/")
		if seen[key] {
			t.Fatalf("duplicate native cell %s", key)
		}
		seen[key] = true
		var value map[string]any
		if err := json.Unmarshal([]byte(row[4]), &value); err != nil {
			t.Fatal(err)
		}
		wants[key] = value
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, configuration := range []string{"baseline", "granted", "object-granted"} {
			t.Run(api+"/"+configuration, func(t *testing.T) {
				root := t.TempDir()
				permissionName, permissionDir := "AccessSlipAccess", "grant"
				if configuration == "object-granted" {
					permissionName, permissionDir = "AccessSlipObjectAccess", "object_grant"
				}
				for name, source := range fixture.Files {
					if relative, ok := strings.CutPrefix(name, "project/"); ok {
						writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(source, "62.0", api))
					}
				}
				for _, dir := range []string{"profiles", "permissionsets"} {
					if _, err := os.Stat(filepath.Join(root, "force-app/main/default", dir)); !os.IsNotExist(err) {
						t.Fatalf("baseline contains %s metadata", dir)
					}
				}
				if configuration != "baseline" {
					source := readSource(permissionDir + "/permissionsets/" + permissionName + ".permissionset-meta.xml")
					writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets", permissionName+".permissionset-meta.xml"), string(source))
				}
				classPath := filepath.Join(root, "force-app/main/default/classes/DefaultPermissionRouteTest.cls")
				source, err := os.ReadFile(classPath)
				if err != nil {
					t.Fatal(err)
				}
				class := string(source)
				var setup strings.Builder
				setup.WriteString("static void emit(String actor,String value) { String expectedText;\n")
				for _, actor := range []string{"admin", "adminRunAs", "standardRunAs"} {
					key := api + "/" + configuration + "/istest/" + actor
					want, ok := wants[key]
					if !ok {
						t.Fatalf("missing %s", key)
					}
					expected := make(map[string]any, len(want))
					for k, v := range want {
						if len(fields) == 0 || slices.Contains(fields, k) {
							expected[k] = v
						}
					}
					if len(fields) > 0 && len(expected) != len(fields) {
						t.Fatalf("missing native DML field in %s", key)
					}
					// The plain COUNT exception dispatch is an existing carry, independent
					// of the captured CRUD/FLS defaults. Keep its native result in org.tsv.
					if configuration == "baseline" && actor == "standardRunAs" && api != "62.0" {
						delete(expected, "plainCount")
						t.Log("carry plainCount: native SecurityException; existing local QueryException dispatch")
					}
					raw, _ := json.Marshal(expected)
					fmt.Fprintf(&setup, "if(actor=='%s') expectedText='%s';\n", actor, strings.ReplaceAll(string(raw), "'", "\\'"))
				}
				setup.WriteString("Map<String,Object> expected=(Map<String,Object>)JSON.deserializeUntyped(expectedText); Map<String,Object> actual=(Map<String,Object>)JSON.deserializeUntyped(value); for(String key:expected.keySet()) { System.assertEquals(JSON.serialize(expected.get(key)),JSON.serialize(actual.get(key)),actor+':'+key); } }")
				class = strings.Replace(class, "static void emit(String value) { System.assert(false,'OBSERVED|'+value); }", setup.String(), 1)
				for _, actor := range []string{"admin", "adminRunAs", "standardRunAs"} {
					class = strings.Replace(class, "emit(DefaultPermissionProbe.observe());", "emit('"+actor+"',DefaultPermissionProbe.observe());", 1)
				}
				if configuration != "baseline" {
					class = strings.Replace(class, "insert actor;", "insert actor; insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='"+permissionName+"'].Id);", 1)
				}
				writeFile(t, classPath, class)
				if configuration != "baseline" {
					writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: ["+permissionName+"]\n")
				}
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatalf("index: %v", index.Diagnostics)
				}
				t.Run("named-istest-runner-admin-and-runAs-twins", func(t *testing.T) {
					options := Options{SelectedClasses: []string{"DefaultPermissionRouteTest"}, NoDiskCache: true, Parallelism: 1}

					run := Run(index, options)
					if summary := run.Summary(); summary.Total != 3 || summary.Passed != 3 || !summary.OK() {
						data, _ := json.Marshal(run)
						t.Fatalf("summary=%+v report=%s", summary, data)
					}
				})
				t.Run("anonymous-named-helper", func(t *testing.T) {
					org := orgFromIndex(index)
					org.APIVersion = api
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					if err := applyConfiguredTestPermissions(&org, p); err != nil {
						t.Fatal(err)
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					body := readSource("anonymous.apex")
					program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.execute(program)
					if err != nil {
						t.Fatal(err)
					}
					var got map[string]any
					for _, line := range result.Debug {
						if strings.HasPrefix(line, "OBSERVED|") {
							if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "OBSERVED|")), &got); err != nil {
								t.Fatal(err)
							}
						}
					}
					want := wants[api+"/"+configuration+"/anonymous/admin"]
					if len(fields) > 0 {
						actualFields := map[string]any{}
						expectedFields := map[string]any{}
						for _, field := range fields {
							actualFields[field] = got[field]
							expectedFields[field] = want[field]
						}
						got, want = actualFields, expectedFields
					}
					if want == nil || !reflect.DeepEqual(got, want) {
						a, _ := json.Marshal(got)
						b, _ := json.Marshal(want)
						t.Fatalf("anonymous got=%s; native=%s", a, b)
					}
				})
				if configuration == "baseline" {
					t.Run("anonymous-runAs-rejection", func(t *testing.T) {
						want := wants[api+"/baseline/anonymous/runAs-rejection"]
						if want["compiled"] != true || want["success"] != false {
							t.Fatal("missing native rejection")
						}
						body := readSource("runas_rejection.apex")
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := storage.NewOrgState()
						machine := vm.New(nil)
						machine.SetOrg(&org)
						_, err = machine.Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || strings.TrimPrefix(runtimeErr.Type, "System.") != "TypeException" {
							t.Fatalf("native rejects runAs: %v", err)
						}
					})
				}
			})
		}
	}
}
