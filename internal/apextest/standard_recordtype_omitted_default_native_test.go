package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/standard_recordtype_omitted_default/{granted,ungranted}/org.tsv.
// A custom object with one record type and no default flag: when the running
// user's permission set makes that record type visible, an insert that omits
// RecordTypeId stores it and takes its picklist default (the in-memory record
// keeps null); with no grant the record keeps no record type and no default.
// Anonymous and IsTest routes, API 62.0 and 67.0.
func TestStandardRecordTypeOmittedDefaultNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/standard_recordtype_omitted_default"
	apex, err := os.ReadFile(filepath.Join(fixture, "x_none2.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for variant, gladeYML := range map[string]string{
		"granted":   "test:\n  permissionSets: [RTP]\n",
		"ungranted": "test:\n  permissionSets: [RTP]\n",
	} {
		body, err := os.ReadFile(filepath.Join(fixture, variant, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		anonymous, istest := map[string]string{}, map[string]string{}
		for _, row := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 {
				t.Fatalf("invalid native row: %s", row)
			}
			if cols[2] == "anonymous" {
				anonymous[cols[1]] = nativeOutcome(cols[5])
			} else {
				istest[cols[1]] = nativeOutcome(cols[5])
			}
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(variant+"/"+api, func(t *testing.T) {
				want := anonymous[api]
				if !strings.HasPrefix(want, "R5|x_none2|") || !strings.Contains(istest[api], "Assertion Failed: "+want) {
					t.Fatalf("native routes disagree or are missing: %q / %q", want, istest[api])
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeFile(t, filepath.Join(root, "glade.yml"), gladeYML)
				err := filepath.WalkDir(filepath.Join(fixture, variant, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(filepath.Join(fixture, variant, "project"), path)
					if err != nil {
						return err
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(data), "__API__", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				if got := runIsTestRouteProbe(t, index, "R5XNoneProbe", "x_none2"); !strings.Contains(got, want) {
					t.Fatalf("NATIVE standard_recordtype_omitted_default/%s/org.tsv %s istest: want %q; Glade %q", variant, api, want, got)
				}
				if got := runAnonymousRouteProbe(t, index, api, string(apex)); !strings.Contains(got, want) {
					t.Fatalf("NATIVE standard_recordtype_omitted_default/%s/org.tsv %s anonymous: want %q; Glade %q", variant, api, want, got)
				}
			})
		}
	}
}
