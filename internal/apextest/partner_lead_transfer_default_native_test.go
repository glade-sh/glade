package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE a Partner Community user with Lead read observes no access to another
// user's Lead when only internal ReadWriteTransfer is declared:
// testdata/conformance/partner_lead_transfer_default/org.tsv:2-3 istest 62.0 67.0.
func TestPartnerLeadTransferDefaultNativeConformance(t *testing.T) {
	const base = "testdata/conformance/partner_lead_transfer_default"
	rows := readNativeTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default"), api)
			copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
			// At API 67 Glade applies user mode to the setup update, unlike the
			// native setup. Explicit system mode preserves the intended Account
			// state while leaving the captured Lead observations unchanged.
			classPath := filepath.Join(root, "force-app/main/default/classes/PartnerLeadDefaultTest.cls")
			source, err := os.ReadFile(classPath)
			if err != nil {
				t.Fatal(err)
			}
			updated := strings.Replace(string(source), "a.IsPartner = true; update a;", "a.IsPartner = true; update as system a;", 1)
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
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PartnerLeadDefaultTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
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
