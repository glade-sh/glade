package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native record/date rows: record_field_hash/org.tsv:2-7 F04.
func TestRecordFieldHashWarmProjectRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/record_field_hash/project")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FieldHash62Test.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"FieldHash62Test"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
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
			if !ok || json.Unmarshal([]byte(payload), &observed) != nil {
				t.Fatalf("record list hashes: %s", payload)
			}
			for key, want := range map[string]string{
				"F01": "[0,-624577096,-538177096,-232183411,86399999,1521001894,-1679829923,-454286711]",
				"F04": "[822176235,-970321423,822176266,-970321361]",
				"F07": "[31,-624577065,-538177065,-232183380,86400030,-624576135,-624577034]",
				"F08": "[2112947465,-538176135]",
			} {
				if string(observed[key]) != want {
					t.Errorf("%s = %s; native = %s", key, observed[key], want)
				}
			}
		})
	}
}
