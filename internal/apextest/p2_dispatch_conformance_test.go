package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// P2 dispatch rows were captured on a scratch org at API 62.0, 67.0 and 68.0
// with identical results. Each row is a synthetic project whose single test
// asserts the native value, so a row passes when every test case passes and no
// class is excluded by the compile gate.
func TestP2DispatchConformance(t *testing.T) {
	var data struct {
		Rows []struct {
			ID          string            `json:"id"`
			APIVersions []string          `json:"apiVersions"`
			Classes     map[string]string `json:"classes"`
			Test        string            `json:"test"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/p2_dispatch.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 8 {
		t.Fatalf("native row count = %d", len(data.Rows))
	}
	for _, row := range data.Rows {
		for _, api := range row.APIVersions {
			t.Run(row.ID+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for name, source := range row.Classes {
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				run := Run(loadTestIndex(t, root), Options{})
				summary := run.Summary()
				if summary.Total != 1 || summary.Passed != 1 {
					var problem any
					if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
						problem = run.Suites[0].Cases[0].Problem
					}
					t.Fatalf("summary=%#v problem=%#v excluded=%#v", summary, problem, run.ExcludedClasses)
				}
			})
		}
	}
}
