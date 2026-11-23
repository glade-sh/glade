package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/commerce_cartitem_missing_product/org.tsv.
// A CartItem whose Product2Id names no Product2 is rejected with
// INVALID_CROSS_REFERENCE_KEY, API 62.0 and 67.0 (istest route).
func TestCommerceCartItemMissingProductNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/commerce_cartitem_missing_product"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	want := "R5|fakeProduct|System.DmlException|Insert failed. First exception on row 0; first error: INVALID_CROSS_REFERENCE_KEY, invalid cross reference id: []"
	for _, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "istest" || !strings.Contains(strings.ReplaceAll(cols[5], "&#124;", "|"), want) {
			t.Fatalf("invalid native row: %s", row)
		}
		api := cols[1]
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), read("project-scratch-def.json"))
			err := filepath.WalkDir(filepath.Join(fixture, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(read(filepath.Join("project", name)), "__API__", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"R5CartProbe"}, SelectedMethod: "fakeProduct", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, want) {
				t.Fatalf("NATIVE commerce_cartitem_missing_product/org.tsv %s istest: want %q; Glade %q", api, want, got)
			}
			// Anonymous route (org-anonymous.tsv): the same error.
			anonymousWant := ""
			for _, row := range strings.Split(strings.TrimSpace(read("org-anonymous.tsv")), "\n")[1:] {
				cols := strings.Split(row, "\t")
				if len(cols) >= 9 && cols[0] == "fake" && cols[1] == api && cols[2] == "anonymous" && cols[3] == "True" {
					anonymousWant = strings.ReplaceAll(cols[5], "&#124;", "|")
				}
			}
			if !strings.Contains(anonymousWant, "INVALID_CROSS_REFERENCE_KEY, invalid cross reference id: []") {
				t.Fatalf("no native anonymous row for %s", api)
			}
			if got := runAnonymousRouteProbe(t, index, api, read("fake.apex")); !strings.Contains(got, anonymousWant) {
				t.Fatalf("NATIVE commerce_cartitem_missing_product/org-anonymous.tsv %s anonymous: want %q; Glade %q", api, anonymousWant, got)
			}
		})
	}
}
