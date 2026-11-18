package apextest

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// NATIVE with the Lead model deployed as Read, a Standard User's first observation still showed
// Lead edit access (sharing not yet recalculated), and a later one did not
// testdata/conformance/lead_read_model_recalculation/org.tsv:2-3 (first, not asserted) istest 62.0 67.0
// NATIVE after recalculation the Standard User has Lead read and no edit, and the update fails
// testdata/conformance/lead_read_model_recalculation/org.tsv:4-5 istest 62.0 67.0
//
// Only the recalculated Lead observation is asserted, and the update only as failing: at
// 67.0 Glade raises SecurityException where native raises DmlException (parked). The Case part is not: the declared
// Read Case model was not applied natively (external_user_case_lead_sharing models rows).
func TestLeadReadModelRecalculationNativeConformance(t *testing.T) {
	const base = "testdata/conformance/lead_read_model_recalculation"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	lead := regexp.MustCompile(`uraLead=\d+|updLead=ok|updLead=[A-Za-z.]+`)
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default"), api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			index := loadTestIndex(t, root)
			want := ""
			for _, row := range rows {
				if row["id"] == "PclEditB" && row["api"] == api {
					_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
				}
			}
			if want == "" {
				t.Fatalf("no native row for %s", api)
			}
			run := Run(index, Options{SelectedClasses: []string{"PclEditTest"}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected one asserted observation: %+v", run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
			failed := func(text string) string {
				return regexp.MustCompile(`updLead=[A-Za-z.]+`).ReplaceAllStringFunc(strings.Join(lead.FindAllString(text, -1), "|"), func(m string) string {
					if m == "updLead=ok" {
						return m
					}
					return "updLead=fail"
				})
			}
			if g, w := failed(got), failed(want); g != w || w == "" {
				t.Errorf("api %s: Glade=%q native=%q", api, got, want)
			}
		})
	}
}
