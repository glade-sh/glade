package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Native API61 named rows cover both Object-view casts and List.sort.
// Keep newer concrete-record hypotheses on the isolated pending branch.
func TestRecordComparatorObjectViewAPI61(t *testing.T) {
	for _, api := range []string{"61.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, name := range []string{"RecordOrderingFixture", "RecordOrderingRegression"} {
				source, err := os.ReadFile(filepath.Join("testdata/record_comparator_object_view", name+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, target, string(source))
				writeFile(t, target+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			result := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"RecordOrderingRegression"}, NoDiskCache: true, Parallelism: 1})
			if s := result.Summary(); s.Total != 6 || s.Passed != 6 {
				raw, _ := json.Marshal(result)
				t.Fatalf("record comparator regression: %s", raw)
			}
		})
	}
}
