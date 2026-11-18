package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Standard User reader of a Private Case with no access gets an explicit
// UserRecordAccess row with HasReadAccess=false, from the Case owner's after-insert
// trigger (zero rule CaseShare rows) and from the test user after save; the
// reader's own Case queries return zero rows and the escalation fields hold.
// testdata/conformance/case_phase/observations.tsv:4-5 istest 62.0 67.0 (org.tsv:5-6 holds the same rows with the helper-shortened message)
//
// The anonymous rows (observations.tsv:2-3) need criteria-based sharing at project load
// and are not replayed here.
func TestCasePhaseNativeConformance(t *testing.T) {
	const base = "testdata/conformance/case_phase"
	rows := readNativeTSV(t, filepath.Join(base, "observations.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project", "force-app"), filepath.Join(root, "force-app"), api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			index := loadTestIndex(t, root)
			class := "CasePhaseTest" + strings.TrimSuffix(api, ".0")
			want := ""
			for _, row := range rows {
				if row["route"] == "istest" && row["api"] == api {
					want = strings.TrimPrefix(row["observation"], "OBS|")
				}
			}
			if want == "" {
				t.Fatalf("no native row for %s", api)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("%s: expected one asserted observation: %+v", class, run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Actual: ")
			if got != want {
				t.Errorf("%s: Glade=%q native=%q", class, got, want)
			}
		})
	}
}
