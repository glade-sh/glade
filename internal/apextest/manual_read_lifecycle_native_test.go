package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a private custom object's Manual Read grant and revoke change both
// named UserRecordAccess and the reader's query: manual_read_lifecycle/org.tsv:2-3.
func TestManualReadLifecycleNativeConformance(t *testing.T) {
	const base = "testdata/conformance/manual_read_lifecycle"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default"), api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			want := ""
			for _, row := range rows {
				if row["id"] == "manual_read" && row["api"] == api && row["route"] == "istest" {
					_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
				}
			}
			if want == "" {
				t.Fatalf("missing native row for %s", api)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ShareProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected asserted observation: %+v", run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
			if got != want {
				t.Errorf("api %s: Glade=%q native=%q", api, got, want)
			}
		})
	}
}
