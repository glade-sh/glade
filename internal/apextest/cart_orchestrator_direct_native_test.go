package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// org.tsv:12-15 captures the direct call and caught null receiver at both APIs.
func TestCartOrchestratorDirectIsTestNativeRoutes(t *testing.T) {
	source, err := os.ReadFile("../sema/testdata/cart_orchestrator_visibility/DirectVisibilityTest.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			path := filepath.Join(root, "force-app/main/default/classes/DirectVisibilityTest.cls")
			writeFile(t, path, string(source))
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, mode := range []string{"cold", "warm"} {
				t.Run(mode, func(t *testing.T) {
					result := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					summary := result.Summary()
					if summary.Total != 1 || summary.Passed != 1 || len(result.ExcludedClasses) != 0 {
						t.Fatalf("summary=%+v excluded=%+v problem=%s", summary, result.ExcludedClasses, firstRunProblem(result))
					}
				})
			}
		})
	}
}
