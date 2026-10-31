package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Supplemental warm route for required_name_insert/org.tsv C137_099/113.
func TestRequiredNameInsertWarmProjectRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/required_name_insert/project")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/RequiredNameProbeTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"RequiredNameProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			Run(index, opts)
			ResetPerfCounters()
			run := Run(index, opts)
			if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
				t.Fatalf("warm cache: %+v", phases)
			}
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
			var observed map[string]string
			want := "Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [Name]: [Name]"
			if !ok || json.Unmarshal([]byte(payload), &observed) != nil || observed["default"] != want || observed["user"] != want {
				t.Fatalf("insert errors: %s", payload)
			}
		})
	}
}
