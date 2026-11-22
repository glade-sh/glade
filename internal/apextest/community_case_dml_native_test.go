package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE Customer Community user Case describe and user-mode DML:
// testdata/conformance/community_case_dml/org.tsv:2-3 istest 62.0 67.0.
func TestCommunityCaseDMLNativeConformance(t *testing.T) {
	const base = "testdata/conformance/community_case_dml"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			copyTree(t, filepath.Join("testdata/conformance/external_user_case_lead_sharing/project/force-app/main/default/classes", "PclSupport.cls"), filepath.Join(classes, "PclSupport.cls"), api)
			copyTree(t, filepath.Join("testdata/conformance/external_user_case_lead_sharing/project/force-app/main/default/classes", "PclSupport.cls-meta.xml"), filepath.Join(classes, "PclSupport.cls-meta.xml"), api)
			copyTree(t, filepath.Join(base, "project/classes"), classes, api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			want := ""
			for _, row := range rows {
				if row["api"] == api {
					_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
				}
			}
			if want == "" {
				t.Fatalf("no native row for %s", api)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"CommunityCaseDmlTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
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
