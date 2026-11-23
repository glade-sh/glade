package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE sort results: testdata/conformance/stdlib_comparable_interface/org.tsv:2-13.
func TestComparableSortUIHostRoutes(t *testing.T) {
	const fixture = "testdata/conformance/stdlib_comparable_interface"
	cases := readNativeTSV(t, filepath.Join(fixture, "cases.tsv"))
	rows := readNativeTSV(t, filepath.Join(fixture, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			sourceDir := filepath.Join(fixture, "force-app/main/default/classes")
			entries, err := os.ReadDir(sourceDir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				body, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", entry.Name()), strings.ReplaceAll(string(body), "__API__", api))
			}
			var wrapper strings.Builder
			wrapper.WriteString("public class SortHost {\n")
			for _, c := range cases {
				if c["route"] != "anonymous" {
					continue
				}
				id := c["id"]
				fmt.Fprintf(&wrapper, "@AuraEnabled public static void lwc_%s() { SortProbe.%s(); }\n", id, id)
				fmt.Fprintf(&wrapper, "public PageReference vf_%s() { SortProbe.%s(); return null; }\n", id, id)
			}
			wrapper.WriteString("}\n")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/SortHost.cls"), wrapper.String())
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/SortHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			seen := 0
			for _, row := range rows {
				if row["api"] != api || row["route"] != "anonymous" {
					continue
				}
				seen++
				for _, route := range []string{"lwc", "visualforce"} {
					row, route := row, route
					t.Run(row["id"]+"/"+route, func(t *testing.T) {
						machine := runner.newMachine()
						var got vm.UIInvocationResult
						var err error
						if route == "lwc" {
							got, err = machine.InvokeLWCMethod("SortHost", "lwc_"+row["id"], nil)
						} else {
							got, err = machine.InvokeVisualforceAction("SortHost", "vf_"+row["id"], "/apex/SortHost", nil)
						}
						if err != nil {
							t.Fatal(err)
						}
						if row["success"] == "True" {
							if !got.Success {
								t.Fatalf("native passed; host result %+v", got)
							}
							return
						}
						if got.Success || got.Error == nil || got.Error.ExceptionTypeName()+": "+got.Error.Message != row["outcome"] {
							t.Fatalf("native %q; host result %+v", row["outcome"], got)
						}
					})
				}
			}
			if seen != 3 {
				t.Fatalf("selected %d native rows, want 3", seen)
			}
		})
	}
}
