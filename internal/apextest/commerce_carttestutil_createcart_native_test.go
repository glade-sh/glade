package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/commerce_carttestutil_createcart/org.tsv.
// CartTestUtil.createCart() and createCart(B2B) create an account, a store, a
// cart named My Cart (Active), one delivery group with no address and one
// product item, and return the cart loaded with them, API 62.0 and 67.0.
func TestCommerceCartTestUtilCreateCartNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/commerce_carttestutil_createcart"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	next := regexp.MustCompile(`; (?:noArg|b2b|b2ce)=Fail `)
	for _, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "istest" {
			t.Fatalf("invalid native row: %s", row)
		}
		api, outcome := cols[1], strings.ReplaceAll(cols[5], "&#124;", "|")
		wants := map[string]string{}
		for _, method := range []string{"noArg", "b2b"} {
			marker := "R5|" + method + "|"
			start := strings.Index(outcome, marker)
			if start < 0 {
				t.Fatalf("native outcome lacks %s: %s", method, outcome)
			}
			text := outcome[start:]
			if loc := next.FindStringIndex(text); loc != nil {
				text = text[:loc[0]]
			}
			wants[method] = text
		}
		for method, want := range wants {
			t.Run(api+"/"+method, func(t *testing.T) {
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
				run := Run(index, Options{SelectedClasses: []string{"R5CartProbe"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("unexpected test selection: %+v", run.Summary())
				}
				if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, want) {
					t.Fatalf("NATIVE commerce_carttestutil_createcart/org.tsv %s istest %s: want %q; Glade %q", api, method, want, got)
				}
			})
		}
	}
}
