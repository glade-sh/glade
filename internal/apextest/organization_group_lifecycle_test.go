package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// The native plain/Communities rows are unchanged. Exercise their query shape
// after local fixture, shape, platform, snapshot and reset lifecycle operations.
func TestOrganizationGroupLifecycleNativeProjectRoutes(t *testing.T) {
	file, err := os.Open("testdata/conformance/setup_object_read/groups.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native group matrix: %v, rows=%d", err, len(rows))
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			var native []struct{ DeveloperName, Type string }
			if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
				t.Fatal(err)
			}
			var checks strings.Builder
			for _, group := range native {
				fmt.Fprintf(&checks, "System.assertEquals(1, [SELECT COUNT() FROM Group WHERE DeveloperName='%s' AND Type='%s']);\n", group.DeveloperName, group.Type)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, row[0]))
			helper := filepath.Join(root, "force-app/main/default/classes/MeadowGroupReader.cls")
			writeFile(t, helper, "public with sharing class MeadowGroupReader { public static void verify() {"+checks.String()+"} public static void empty() {System.assertEquals(0, [SELECT COUNT() FROM Group]);} }")
			writeFile(t, helper+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			testPath := filepath.Join(root, "force-app/main/default/classes/MeadowGroupReaderTest.cls")
			writeFile(t, testPath, "@IsTest private class MeadowGroupReaderTest { @IsTest static void observe() {MeadowGroupReader.verify();} @IsTest static void snapshotUnchanged() {MeadowGroupReader.empty();} }")
			writeFile(t, testPath+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"MeadowGroupReaderTest"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			if len(cases) != 2 {
				t.Fatalf("named routes=%d, want 2", len(cases))
			}
			for _, tc := range cases {
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("compile %s: %v %v", key, methodErrors[key], programErrors[key])
				}
			}
			run := func(t *testing.T, org storage.OrgState, empty bool, route string) {
				t.Helper()
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if route == "anonymous" {
					call := "MeadowGroupReader.verify();"
					if empty {
						call = "MeadowGroupReader.empty();"
					}
					program, err := vm.CompileAnonymousWithOptions(call, vm.CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				} else {
					name := "observe"
					if empty {
						name = "snapshotUnchanged"
					}
					for _, tc := range cases {
						if tc.MethodName != name {
							continue
						}
						key := testCaseKey(tc)
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						machine.SetTestSeeAllData(tc.SeeAllData)
						if _, err := machine.ExecuteInClass(programs[key], tc.ClassName); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			for _, lifecycle := range []string{"template/platform", "template/shape", "template/fixture", "rollback/platform", "rollback/shape", "rollback/fixture", "reset/data", "reset/all", "reset/platform"} {
				for _, entry := range []string{"anonymous", "isTest"} {
					t.Run(lifecycle+"/"+entry, func(t *testing.T) {
						source := storage.NewOrgState()
						source.APIVersion = row[0]
						features := []string{}
						if row[1] == "communities" {
							features = []string{"Communities"}
						}
						storage.ApplyOrgShape(&source, features)
						storage.EnsureDeterministicPlatformData(&source)
						parts := strings.Split(lifecycle, "/")
						var target storage.OrgState
						if parts[0] == "reset" {
							target = source.Clone()
							switch parts[1] {
							case "data":
								storage.ResetNonPlatformData(&target)
							case "all":
								storage.ResetData(&target)
							case "platform":
								storage.ResetPlatformData(&target)
							}
						} else {
							// An older fixture may have the Group schema but no organization rows.
							state := source.Objects["Group"]
							state.Records = make(map[storage.ID]storage.Record)
							source.Objects["Group"] = state
							if parts[0] == "template" {
								target = storage.NewRuntimeTemplate(source).CloneSnapshotOrg()
							} else {
								target = storage.SnapshotRuntimeOrg(&source)
							}
							switch parts[1] {
							case "platform":
								storage.EnsureDeterministicPlatformData(&target)
							case "shape":
								storage.ApplyOrgShape(&target, features)
							case "fixture":
								fixture := storage.NewFixture()
								fixture.Org.Features = features
								if err := storage.ApplyFixture(&target, fixture); err != nil {
									t.Fatal(err)
								}
							}
							t.Run("sourceUnchanged", func(t *testing.T) { run(t, source, true, entry) })
						}
						run(t, target, false, entry)
					})
				}
			}
		})
	}
}
