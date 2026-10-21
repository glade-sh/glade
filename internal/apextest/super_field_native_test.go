package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSuperFieldNativeConformance(t *testing.T) {
	var fixture struct {
		APIs     []string          `json:"apis"`
		Sources  map[string]string `json:"sources"`
		Observed []string          `json:"observed"`
	}
	raw, err := os.ReadFile("testdata/conformance/super_field_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(fixture.Observed)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.APIs) != 3 || len(fixture.Observed) != 29 {
		t.Fatal("incomplete native fixture")
	}
	for _, api := range fixture.APIs {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default/classes")
			for name, source := range fixture.Sources {
				writeFile(t, filepath.Join(base, name+".cls"), source)
				writeFile(t, filepath.Join(base, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "SuperFieldReplayTest.cls"), `@IsTest private class SuperFieldReplayTest {
@IsTest static void replay(){System.assertEquals('`+string(expected)+`',JSON.serialize(SuperFieldBoundary17.capture()));}
}`)
			writeFile(t, filepath.Join(base, "SuperFieldReplayTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"SuperFieldReplayTest"}, NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 || got.Unsupported != 0 {
				t.Fatalf("native replay: summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
