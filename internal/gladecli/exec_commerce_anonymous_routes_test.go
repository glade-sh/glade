package gladecli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows, route anonymous, API 62.0 and 67.0:
// ../apextest/testdata/conformance/commerce_inventory_request_getters/org.tsv
// (the four commerce_inventory request classes cannot be built outside a test;
// responses and CartDeliveryGroupMethod can; CartTestUtil.createCart is a test
// method), commerce_cartutil_anonymous/org.tsv (getCart and every createCart
// overload throw) and commerce_cart_custom_fields/org.tsv (getCart is a test method, so
// the cart custom-field members are unreachable from anonymous Apex). The
// IsTest rows for the same fixtures run in internal/apextest.
func TestExecCommerceAnonymousRoutesMatchNativeRows(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, fixture := range []string{"commerce_inventory_request_getters", "commerce_cart_custom_fields", "commerce_cartutil_anonymous"} {
		base := filepath.Join("../apextest/testdata/conformance", fixture)
		table, err := os.ReadFile(filepath.Join(base, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		apex, err := os.ReadFile(filepath.Join(base, "run.apex"))
		if err != nil {
			t.Fatal(err)
		}
		scratch, err := os.ReadFile(filepath.Join(base, "project-scratch-def.json"))
		if err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, row := range strings.Split(strings.TrimSpace(string(table)), "\n")[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 || cols[2] != "anonymous" {
				continue
			}
			api, want := cols[1], strings.ReplaceAll(cols[5], "&#124;", "|")
			t.Run(fixture+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(scratch))
				if fixture == "commerce_cart_custom_fields" {
					writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [R5OG]\n")
				}
				err := filepath.WalkDir(filepath.Join(base, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					rel, _ := filepath.Rel(filepath.Join(base, "project"), path)
					writeTestFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "__API__", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				ok, output := caseFLSExec(root, string(apex))
				if !ok || !strings.Contains(output, want) {
					t.Fatalf("NATIVE %s/org.tsv %s anonymous: want %q; Glade ok=%v %s", fixture, api, want, ok, output)
				}
			})
			checked++
		}
		if checked != 2 {
			t.Fatalf("%s: %d anonymous rows", fixture, checked)
		}
	}
}
