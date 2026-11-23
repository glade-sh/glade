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
	istestRows := map[string]map[string]string{}
	for _, profile := range []string{"dev", "hconly"} {
		body, err := os.ReadFile(filepath.Join(fixture, profile, "org-istest.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		istestRows[profile] = map[string]string{}
		for _, row := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 || cols[2] != "istest" {
				t.Fatalf("invalid native istest row: %s", row)
			}
			istestRows[profile][cols[1]] = nativeOutcome(cols[5])
		}
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
				probeClass, err := os.ReadFile(filepath.Join(fixture, "R5UomProbe.cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/R5UomProbe.cls"), string(probeClass))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/R5UomProbe.cls-meta.xml"), fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				index := loadTestIndex(t, root)
				if got := runAnonymousRouteProbe(t, index, api, string(apex)); !strings.Contains(got, want) {
					t.Fatalf("NATIVE standard_unit_of_measure_type/%s/org.tsv %s anonymous: want %q; Glade %q", profile, api, want, got)
				}
				// The IsTest rows (dev, hconly) report the same outcome.
				if istestWant, ok := istestRows[profile][api]; ok {
					if !strings.Contains(istestWant, "Assertion Failed: "+want) {
						t.Fatalf("native istest outcome differs from anonymous: %s", istestWant)
					}
					if got := runIsTestRouteProbe(t, index, "R5UomProbe", "uom"); !strings.Contains(got, want) {
						t.Fatalf("NATIVE standard_unit_of_measure_type/%s/org-istest.tsv %s istest: want %q; Glade %q", profile, api, want, got)
					}
				}
			})
		}
	}
}
