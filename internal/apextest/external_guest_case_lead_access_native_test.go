package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Guest-type user has no Case or Lead access: UserRecordAccess read and edit false,
// both count queries raise, isAccessible and isCreateable false for both objects
// testdata/conformance/external_guest_case_lead_access/org.tsv:2-3 istest 62.0 67.0
func TestExternalGuestCaseLeadAccessNativeConformance(t *testing.T) {
	const base = "testdata/conformance/external_guest_case_lead_access"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app"), api)
			// The Sites feature seeds the Guest-type profile that a deployed site creates natively.
			writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites]\n")
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			index := loadTestIndex(t, root)
			want := ""
			for _, row := range rows {
				if row["id"] == "PclGuestTest" && row["api"] == api {
					_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
				}
			}
			if want == "" {
				t.Fatalf("no native row for %s", api)
			}
			run := Run(index, Options{SelectedClasses: []string{"PclGuestTest"}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected one asserted observation: %+v", run.Summary())
			}
			if got := run.Suites[0].Cases[0].Problem.Message; !strings.HasSuffix(got, want) {
				t.Errorf("api %s: Glade=%q native=%q", api, got, want)
			}
		})
	}
}
