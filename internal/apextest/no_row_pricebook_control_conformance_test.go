package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNoRowPricebookControlsNativeConformance(t *testing.T) {
	carries := map[string]nativeObservationCarry{}
	for _, api := range []string{"62.0", "67.0"} {
		carries[api+"/allData"] = nativeObservationCarry{LocalJSON: `{"standardCount":1,"accountCount":0}`, Reason: "native org had an explicitly inserted Account; local project has no preexisting Account"}
	}
	runNativeObservationProjectWithCarries(t, "no_row_seeds/followup", "NoRowPricebookControlTest", []string{"62.0", "67.0"}, 2, carries)
}

// Replay the captured isolated row after memory and disk runtime restoration.
func TestNoRowPricebookControlsCachedRoutes(t *testing.T) {
	runNoRowPricebookCachedRoutes(t, "followup", "NoRowPricebookControlTest")
}

func runNoRowPricebookCachedRoutes(t *testing.T, family, class string) {
	t.Helper()
	fixture := filepath.Join("testdata/conformance/no_row_seeds", family)
	restore := EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			InvalidateRuntimeCaches()
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			f, err := os.Open(filepath.Join(fixture, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r := csv.NewReader(f)
			r.Comma = '\t'
			r.LazyQuotes = true
			rows, err := r.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			for _, row := range rows[1:] {
				if row[0] == api && row[1] == "isolated" {
					if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want == nil {
				t.Fatal("missing native row")
			}
			for pass := 0; pass < 3; pass++ {
				if pass == 2 {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				report := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "isolated", Parallelism: 1, PerfCounters: true})
				if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 || report.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", report.Summary())
				}
				message := report.Suites[0].Cases[0].Problem.Message
				_, payload, ok := strings.Cut(message, "OBSERVED|isolated|")
				var got map[string]any
				if !ok || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%v; local=%s", want, message)
				}
				phases := SnapshotPerfCounters().Phases
				if pass == 1 && phases.MemoryCacheHits != 1 {
					t.Fatalf("no memory cache hit: %+v", phases)
				}
				if pass == 2 && phases.DiskCacheHits != 1 {
					t.Fatalf("no disk cache hit: %+v", phases)
				}
			}
		})
	}
}
