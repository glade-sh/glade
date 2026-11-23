package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/standard_unit_of_measure_type/<profile>/org.tsv.
// UnitOfMeasure.Type is required. A plain Developer org gives it no values and
// rejects an insert without it. HealthCloudAddOn (Enterprise) defaults it to
// Custom and PublicSectorAccess (Enterprise) to Period. API 62.0 and 67.0.
func TestStandardUnitOfMeasureTypeNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/standard_unit_of_measure_type"
	apex, err := os.ReadFile(filepath.Join(fixture, "uom.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for profile, features := range map[string]string{"dev": "[]", "psa": "[PublicSectorAccess]", "hconly": "[HealthCloudAddOn]"} {
		body, err := os.ReadFile(filepath.Join(fixture, profile, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 || cols[0] != "uom_notype" {
				continue
			}
			api, want := cols[1], strings.ReplaceAll(cols[5], "&#124;", "|")
			if cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || strings.HasPrefix(cols[8], "tainted") && !(strings.Contains(cols[5], "&#124;EXC&#124;") && cols[8] == "tainted:unchecked-dml:1") {
				// A rejected insert rolls back; the harness cannot prove that
				// by a residue query, so it marks the row unchecked.
				t.Fatalf("invalid native row: %s", row)
			}
			t.Run(profile+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: "+features+"\n")
				source := strings.ReplaceAll(string(apex), "System.debug(", "r5out.add(")
				probe := "@IsTest private class UomProbe { @IsTest static void observe() {\nList<String> r5out = new List<String>();\n" + source + "\nSystem.assert(false, String.join(r5out, ';'));\n} }"
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/UomProbe.cls"), probe)
				index := loadTestIndex(t, root)
				run := Run(index, Options{SelectedClasses: []string{"UomProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("unexpected test selection: %+v", run.Summary())
				}
				if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, want) {
					t.Fatalf("NATIVE standard_unit_of_measure_type/%s/org.tsv %s anonymous: want %q; Glade %q", profile, api, want, got)
				}
			})
		}
	}
}
