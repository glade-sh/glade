package apextest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Assignment is part of the native fixture, independently of whether a row
// happens to touch a protected field. Check every shared factory, including
// families whose existing assertions would pass without the captured grants.
func TestSharedFixturePermissionReplay(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		version := "api" + strings.TrimSuffix(api, ".0")
		type fixture struct {
			name, permission, source, receipt string
			copy                              func(*testing.T, string) string
		}
		var fixtures []fixture
		for _, family := range platformTransactionFamilies {
			fixtures = append(fixtures, fixture{
				name: "transaction/" + family, permission: "CaptureFixture",
				source:  platformTransactionFixture + "/setup/force-app/main/default/permissionsets/CaptureFixture.permissionset-meta.xml",
				receipt: filepath.Join(platformTransactionFixture, family, version, "native/anonymous/RUN-FACTS.txt"),
				copy:    func(t *testing.T, api string) string { return copyPlatformTransactionProject(t, family, api) },
			})
		}
		for _, factory := range []struct {
			name, family string
			copy         func(*testing.T, string) string
		}{
			{"path_extensions", "flow_paths", copyPathExtensionProject},
			{"assignment_extensions", "assignment_rules", copyAssignmentExtensionProject},
		} {
			fixtures = append(fixtures, fixture{
				name: factory.name, permission: "OracleFixture", copy: factory.copy,
				source:  pathExtensionFixture + "/project/force-app/main/default/permissionsets/OracleFixture.permissionset-meta.xml",
				receipt: filepath.Join(pathExtensionFixture, factory.family, "native", version, "anonymous/RUN-FACTS.txt"),
			})
		}
		fixtures = append(fixtures, fixture{
			name: "event_elements", permission: "ElementFixture", copy: eventElementsProject,
			source:  eventElementsFixture + "/project/force-app/main/default/permissionsets/ElementFixture.permissionset-meta.xml",
			receipt: filepath.Join(eventElementsFixture, "native", version, "RUN-FACTS.txt"),
		})
		for _, f := range fixtures {
			t.Run(api+"/"+f.name, func(t *testing.T) {
				native, err := os.ReadFile(f.source)
				if err != nil {
					t.Fatal(err)
				}
				record, err := os.ReadFile(f.receipt)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(record), "fixture_permission_set_assigned=true\n") ||
					!strings.Contains(string(record), fmt.Sprintf("=%x\n", sha256.Sum256(native))) {
					t.Fatal("missing native assignment or exact permission source digest")
				}
				root := f.copy(t, api)
				p := loadProjectForTest(t, root)
				if !reflect.DeepEqual(p.Test.PermissionSets, []string{f.permission}) {
					t.Fatalf("native assigned %s; replay assigns %v", f.permission, p.Test.PermissionSets)
				}
				copied, err := os.ReadFile(filepath.Join(root, "force-app/main/default/permissionsets", f.permission+".permissionset-meta.xml"))
				if err != nil || !bytes.Equal(copied, native) {
					t.Fatalf("replay did not retain the complete native permission metadata: %v", err)
				}
				// An unrelated deployed set is deliberately unassigned. The setup
				// contract is an allowlist of captured assignments, never all metadata.
				writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/UnassignedReplayControl.permissionset-meta.xml"), `<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>Unassigned replay control</label></PermissionSet>`)
				source := capturedPermissionAssertions(t, f.permission, native)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/FixtureReplayTest.cls"), "@IsTest private class FixtureReplayTest { @IsTest static void exactGrants() {"+source+"} }")
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/FixtureReplayTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				org.APIVersion = api
				if err := applyConfiguredTestPermissions(&org, p); err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, route := range []string{"anonymous", "named_istest"} {
					t.Run(route, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						if route == "named_istest" {
							machine.EnableTestContext()
							if err := RegisterTestMethodForRequest(machine, index, "FixtureReplayTest", "exactGrants"); err != nil {
								t.Fatal(err)
							}
							program, err = vm.CompileAnonymousWithOptions("FixtureReplayTest.exactGrants();", vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = machine.ExecuteTestInClass(program, "FixtureReplayTest")
						} else {
							_, err = machine.Execute(program)
						}
						if err != nil {
							t.Fatal(err)
						}
					})
				}
				for _, route := range []string{"project_cold", "project_warm"} {
					t.Run(route, func(t *testing.T) {
						ResetPerfCounters()
						report := Run(index, Options{SelectedClasses: []string{"FixtureReplayTest"}, NoDiskCache: true, Parallelism: 1, PerfCounters: true})
						if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
							t.Fatalf("permission replay: %+v; suites=%+v", summary, report.Suites)
						}
						if phases := SnapshotPerfCounters().Phases; route == "project_warm" && (phases.MemoryCacheHits != 1 || phases.CacheMisses != 0) {
							t.Fatalf("expected cached fixture replay: %+v", phases)
						}
					})
				}
			})
		}
	}
}

// Duplicate-rule projections also share the original transaction/extension
// captures. The standard-object variants retain assignment success per API in
// their capture manifest; do not infer it from successful DML.
func TestDuplicateFixturePermissionReplay(t *testing.T) {
	data, _ := readDuplicateNativeData(t)
	raw, err := os.ReadFile(filepath.Join(duplicateEnforcementFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Captures map[string][]struct {
			API      string `json:"api"`
			Assigned bool   `json:"permission_set_assigned"`
		} `json:"captures"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, fixture := range []string{"existing", "extension", "standard", "standard_extended"} {
			t.Run(api+"/"+fixture, func(t *testing.T) {
				name := "StandardDuplicateFixture"
				if fixture == "existing" {
					name = "CaptureFixture"
				}
				if fixture == "extension" {
					name = "OracleFixture"
				}
				if strings.HasPrefix(fixture, "standard") {
					assigned := false
					for _, c := range manifest.Captures[fixture] {
						if c.API == api && c.Assigned {
							assigned = true
						}
					}
					if !assigned {
						t.Fatal("missing recorded native assignment")
					}
				}
				root := duplicateProject(t, data, fixture, api)
				p := loadProjectForTest(t, root)
				if !reflect.DeepEqual(p.Test.PermissionSets, []string{name}) {
					t.Fatalf("native assigned %s; replay assigns %v", name, p.Test.PermissionSets)
				}
				if fixture == "existing" {
					return
				} // Exact metadata and four routes covered above.
				path := "force-app/main/default/permissionsets/" + name + ".permissionset-meta.xml"
				native := []byte(data.Projects[fixture][path])
				if len(native) == 0 {
					t.Fatal("missing captured permission metadata")
				}
				copied, err := os.ReadFile(filepath.Join(root, path))
				if err != nil || !bytes.Equal(copied, native) {
					t.Fatal("replay changed native permission metadata")
				}
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				org.APIVersion = api
				if err := applyConfiguredTestPermissions(&org, p); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(capturedPermissionAssertions(t, name, native), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func capturedPermissionAssertions(t *testing.T, name string, raw []byte) string {
	t.Helper()
	var metadata struct {
		Classes []struct {
			Name    string `xml:"apexClass"`
			Enabled bool   `xml:"enabled"`
		} `xml:"classAccesses"`
		Fields []struct {
			Field string `xml:"field"`
			Read  bool   `xml:"readable"`
			Edit  bool   `xml:"editable"`
		} `xml:"fieldPermissions"`
		Objects []struct {
			Object    string `xml:"object"`
			Read      bool   `xml:"allowRead"`
			Create    bool   `xml:"allowCreate"`
			Edit      bool   `xml:"allowEdit"`
			Delete    bool   `xml:"allowDelete"`
			ViewAll   bool   `xml:"viewAllRecords"`
			ModifyAll bool   `xml:"modifyAllRecords"`
		} `xml:"objectPermissions"`
	}
	if err := xml.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	fmt.Fprintf(&source, `System.assertEquals(1,[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='%s']);
System.assertEquals(0,[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='UnassignedReplayControl']);
Id grantId=[SELECT Id FROM PermissionSet WHERE Name='%s'].Id;
System.assertEquals(%d,[SELECT COUNT() FROM FieldPermissions WHERE ParentId=:grantId]);
System.assertEquals(%d,[SELECT COUNT() FROM ObjectPermissions WHERE ParentId=:grantId]);
`, name, name, len(metadata.Fields), len(metadata.Objects))
	for _, f := range metadata.Fields {
		fmt.Fprintf(&source, "System.assertEquals(1,[SELECT COUNT() FROM FieldPermissions WHERE ParentId=:grantId AND Field='%s' AND PermissionsRead=%t AND PermissionsEdit=%t]);\n", f.Field, f.Read, f.Edit)
	}
	for _, o := range metadata.Objects {
		fmt.Fprintf(&source, "System.assertEquals(1,[SELECT COUNT() FROM ObjectPermissions WHERE ParentId=:grantId AND SobjectType='%s' AND PermissionsRead=%t AND PermissionsCreate=%t AND PermissionsEdit=%t AND PermissionsDelete=%t AND PermissionsViewAllRecords=%t AND PermissionsModifyAllRecords=%t]);\n", o.Object, o.Read, o.Create, o.Edit, o.Delete, o.ViewAll, o.ModifyAll)
	}
	classCount := 0
	for i, c := range metadata.Classes {
		count := 0
		if c.Enabled {
			count = 1
			classCount++
		}
		fmt.Fprintf(&source, "Id classId%d=[SELECT Id FROM ApexClass WHERE Name='%s'].Id; System.assertEquals(%d,[SELECT COUNT() FROM SetupEntityAccess WHERE ParentId=:grantId AND SetupEntityType='ApexClass' AND SetupEntityId=:classId%d]);\n", i, c.Name, count, i)
	}
	fmt.Fprintf(&source, "System.assertEquals(%d,[SELECT COUNT() FROM SetupEntityAccess WHERE ParentId=:grantId AND SetupEntityType='ApexClass']);\n", classCount)
	return source.String()
}
