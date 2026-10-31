package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native cleared/null Id rows: cleared_record_hash/org.tsv:2-7 C05.
func TestClearedRecordHashWarmProjectRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/cleared_record_hash/project")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ClearedHash62Test.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"ClearedHash62Test"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			Run(index, opts)
			ResetPerfCounters()
			run := Run(index, opts)
			if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
				t.Fatalf("warm project route did not reuse its runtime: %+v", phases)
			}
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
			var observed map[string]json.RawMessage
			if !ok || json.Unmarshal([]byte(payload), &observed) != nil || string(observed["C05"]) != "[[true,-1304169783,-1304169783,-1304169752,-1304169752],[true,515749503,515749503,515749534,515749534],[true,515749503,515749503,515749534,515749534],[true,-1304169783,-1304169783,-1304169752,-1304169752],[true,2363,2363,2394,2394]]" {
				t.Fatalf("record list hashes: %s", payload)
			}
		})
	}
}
