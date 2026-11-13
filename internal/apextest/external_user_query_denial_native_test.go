package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Guest user counting Case, a Guest user counting Lead and a Community user counting
// Lead each raise SecurityException "Access to entity '<Object>' denied" under runAs
// testdata/conformance/external_user_query_denial/org.tsv:2-7 istest 62.0 67.0
func TestExternalUserQueryDenialNativeConformance(t *testing.T) {
	const base = "testdata/conformance/external_user_query_denial"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default/classes"), api)
			// The Sites feature seeds the Guest-type profile that a deployed site creates natively.
			writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites]\n")
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			index := loadTestIndex(t, root)
			for _, class := range []string{"PclMsgGuestCaseTest", "PclMsgGuestLeadTest", "PclMsgCcLeadTest"} {
				want := ""
				for _, row := range rows {
					if row["id"] == class && row["api"] == api {
						_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
					}
				}
				if want == "" {
					t.Fatalf("no native row for %s %s", class, api)
				}
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("%s: expected one asserted observation: %+v", class, run.Summary())
				}
				_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
				if got != want {
					t.Errorf("%s %s: Glade=%q native=%q", class, api, got, want)
				}
			}
		})
	}
}
