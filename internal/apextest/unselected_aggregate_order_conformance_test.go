package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Replay the unchanged owned named Apex probe observed at each source API.
func TestRunUnselectedAggregateOrderConformance(t *testing.T) {
	source, err := os.ReadFile("testdata/conformance/unselected_aggregate_order.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default/classes/OwnedAggregateOrderProbe")
			writeFile(t, base+".cls", string(source))
			writeFile(t, base+".cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("aggregate order probe failed: %s", data)
			}
		})
	}
}
