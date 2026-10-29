package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Fifteen- and eighteen-character IDs identify the same assigned records;
// this fixture asserts no new permission or configuration defaults.
func TestPermissionAssignmentIdentityConformance(t *testing.T) {
	var fixture struct {
		APIs, Sources, Boundaries []string
		TestClass, Source         string
		MethodCount               int
		Files                     map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/permission_assignment_identity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fixture.APIs, []string{"62.0", "67.0", "68.0"}) || fixture.MethodCount != 6 || len(fixture.Files) != 3 || len(fixture.Sources) != 3 || fixture.Source == "" {
		t.Fatal("incomplete documented metadata fixture")
	}
	for _, api := range fixture.APIs {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for path, contents := range fixture.Files {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), contents)
			}
			path := filepath.Join(root, "force-app/main/default/classes", fixture.TestClass+".cls")
			writeFile(t, path, fixture.Source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{fixture.TestClass}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != fixture.MethodCount || summary.Passed != fixture.MethodCount {
				data, _ := json.Marshal(run)
				t.Fatalf("documented metadata grant: %s", data)
			}
		})
	}
	for _, boundary := range fixture.Boundaries {
		t.Log("UNCHANGED", boundary)
	}
}
