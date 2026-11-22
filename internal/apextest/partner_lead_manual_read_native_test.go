package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Partner Community user sees neither unshared Lead, then sees the
// one with a manual Read share, through direct access rows, COUNT and SELECT:
// testdata/conformance/partner_lead_manual_read/org.tsv:2-3 istest 62.0 67.0.
func TestPartnerLeadManualReadNativeConformance(t *testing.T) {
	const base = "testdata/conformance/partner_lead_manual_read"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default"), api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			// Match the native Account setup state despite the local secure-default gap.
			classPath := filepath.Join(root, "force-app/main/default/classes/PartnerLeadSharingTest.cls")
			source, err := os.ReadFile(classPath)
			if err != nil {
				t.Fatal(err)
			}
			updated := strings.Replace(string(source), "a.IsPartner=true; update a;", "a.IsPartner=true; update as system a;", 1)
			if updated == string(source) {
				t.Fatal("missing setup update")
			}
			writeFile(t, classPath, updated)
			want := ""
			for _, row := range rows {
				if row["api"] == api {
					_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
				}
			}
			if want == "" {
				t.Fatalf("no native row for %s", api)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PartnerLeadSharingTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
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
