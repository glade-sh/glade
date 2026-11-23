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

// NATIVE rows: testdata/conformance/commerce_calculator_executor_defaults/org.tsv,
// commerce_cart_loaded_shapes/org.tsv and commerce_cart_buyer_details_address/org.tsv and
// commerce_cart_validation_outputs/org.tsv and
// commerce_cart_delivery_methods/org.tsv, API 62.0 and
// 67.0, test route. The first fixture runs project subclasses of the platform
// cart calculators against a project CartCalculateExecutorMock: calculate()
// runs the executor's default callback. The second reads a cart loaded with
// CartTestUtil.getCart (quantity scale, no selected delivery method), builds a
// delivery group method without a product, and runs a calculator built without
// an executor.
func TestCommerceCalculatorCartShapesNativeConformance(t *testing.T) {
	type fixture struct {
		dir, class string
		methods    []string
	}
	fixtures := []fixture{
		{dir: "commerce_calculator_executor_defaults", class: "R5ExecProbe", methods: []string{"viaDefault", "viaOverride"}},
		{dir: "commerce_cart_loaded_shapes", class: "R5CartProbe3", methods: []string{"loaded", "groupAddress", "superCalc"}},
		{dir: "commerce_cart_buyer_details_address", class: "R5CartProbe4", methods: []string{"details", "address"}},
		{dir: "commerce_cart_validation_outputs", class: "R5CartProbe5", methods: []string{"loadedCvo", "newCvo", "change"}},
		{dir: "commerce_cart_delivery_methods", class: "R5CartProbe6", methods: []string{"methods", "selectedAfterClear"}},
		{dir: "commerce_place_order_super_validate", class: "R5PoProbe", methods: []string{"domains", "withMock"}},
		{dir: "commerce_inventory_request_getters", class: "R5InvProbe", methods: []string{"sizes"}},
		{dir: "commerce_cart_custom_fields", class: "R5OgProbe", methods: []string{"custom"}},
	}
	outcome := regexp.MustCompile(`(?:^|; )(\w+)=(?:Fail|Pass) `)
	for _, f := range fixtures {
		f := f
		root := filepath.Join("testdata/conformance", f.dir)
		read := func(path string) string {
			t.Helper()
			body, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			return string(body)
		}
		for _, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 || cols[2] != "istest" || cols[3] != "true" {
				t.Fatalf("invalid native row in %s: %s", f.dir, row)
			}
			api := cols[1]
			text := strings.ReplaceAll(cols[5], "&#124;", "|")
			wants := map[string]string{}
			locs := outcome.FindAllStringSubmatchIndex(text, -1)
			for i, loc := range locs {
				end := len(text)
				if i+1 < len(locs) {
					end = locs[i+1][0]
				}
				wants[text[loc[2]:loc[3]]] = strings.TrimPrefix(text[loc[1]:end], "System.AssertException: Assertion Failed: ")
			}
			for _, method := range f.methods {
				want, ok := wants[method]
				if !ok {
					t.Fatalf("%s %s: native outcome lacks %s: %s", f.dir, api, method, text)
				}
				t.Run(f.dir+"/"+api+"/"+method, func(t *testing.T) {
					dir := t.TempDir()
					writeFile(t, filepath.Join(dir, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					writeFile(t, filepath.Join(dir, "config/project-scratch-def.json"), read("project-scratch-def.json"))
					if body, err := os.ReadFile(filepath.Join(root, "assign-permsets.txt")); err == nil {
						writeFile(t, filepath.Join(dir, "glade.yml"), "test:\n  permissionSets: ["+strings.Join(strings.Fields(string(body)), ", ")+"]\n")
					}
					err := filepath.WalkDir(filepath.Join(root, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						name, err := filepath.Rel(filepath.Join(root, "project"), path)
						if err != nil {
							return err
						}
						writeFile(t, filepath.Join(dir, name), strings.ReplaceAll(read(filepath.Join("project", name)), "__API__", api))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, dir)
					run := Run(index, Options{SelectedClasses: []string{f.class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("unexpected test selection: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					got := strings.TrimPrefix(problem.Message, "Assertion Failed: ")
					if problem.Type != "System.AssertException" && problem.Type != "" {
						got = problem.Type + ": " + problem.Message
					}
					if got != want {
						t.Fatalf("NATIVE %s/org.tsv %s istest %s:\nwant %q\nGlade %q", f.dir, api, method, want, got)
					}
				})
			}
		}
	}
}
