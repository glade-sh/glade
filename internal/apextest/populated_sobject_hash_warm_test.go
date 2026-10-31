package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Supplemental warm-route coverage for adjacent_empty_sobject_hash/org.tsv H02.
// API 68 expectations: sobject_hash_version/org.tsv:6-7.
func TestPopulatedSObjectHashWarmProjectRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/adjacent_empty_sobject_hash/project")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/HashAdjacent62Test.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"HashAdjacent62Test"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
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
			if !ok || json.Unmarshal([]byte(payload), &observed) != nil || string(observed["H02"]) != "[-1304172146,515747140,2420395,-1394955679]" {
				t.Fatalf("empty hashes: %s", payload)
			}
		})
	}
}
