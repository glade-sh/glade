package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/standard_recordtype_picklist_default/org.tsv.
// A record type's default picklist value applies on insert, with an explicit
// RecordTypeId or through the user's default record type, for custom and
// standard objects, and it satisfies a required standard picklist. The same
// rows record that UnitOfMeasure.Type is defaulted by the org's feature set
// (PublicSectorAccess here). Salesforce, API 62.0 and 67.0.
func TestStandardRecordTypePicklistDefaultNativeConformance(t *testing.T) {
	runRecordTypePicklistDefaultFixture(t, "testdata/conformance/standard_recordtype_picklist_default",
		"org:\n  features: [PublicSectorAccess]\ntest:\n  permissionSets: [RTP]\n",
		map[string]string{"x_norecordtype": "native applies the user's default record type picklist default without RecordTypeId; Glade's Apex insert mode does not (uncaptured RecordTypeId result)"})
}

// NATIVE rows: testdata/conformance/standard_recordtype_profile_default/org.tsv.
// The same defaults apply when the Admin profile, not a permission set, makes
// the record types visible.
func TestStandardRecordTypeProfileDefaultNativeConformance(t *testing.T) {
	runRecordTypePicklistDefaultFixture(t, "testdata/conformance/standard_recordtype_profile_default",
		"org:\n  features: [PublicSectorAccess]\n", nil)
}

func runRecordTypePicklistDefaultFixture(t *testing.T, fixture, gladeYML string, skip map[string]string) {
	t.Helper()
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	cases := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")[1:] {
		cols := strings.Split(line, "\t")
		cases[cols[0]] = cols[2]
	}
	// Anonymous rows: one outcome per case. IsTest rows: one row per API whose
	// outcome lists every probe method.
	anonymous := map[string]map[string]string{}
	for line, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || !strings.HasPrefix(cols[8], "cleaned") {
			t.Fatalf("invalid native row %d: %s", line+2, row)
		}
		if anonymous[cols[1]] == nil {
			anonymous[cols[1]] = map[string]string{}
		}
		anonymous[cols[1]][cols[0]] = nativeOutcome(cols[5])
	}
	istest := map[string]string{}
	for line, row := range strings.Split(strings.TrimSpace(read("org-istest.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "istest" {
			t.Fatalf("invalid native istest row %d: %s", line+2, row)
		}
		istest[cols[1]] = nativeOutcome(cols[5])
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
		writeFile(t, filepath.Join(root, "glade.yml"), gladeYML)
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
		for id, want := range anonymous[api] {
			t.Run(api+"/"+id, func(t *testing.T) {
				if reason, skipped := skip[id]; skipped {
					t.Log(reason)
					return
				}
				method := strings.TrimSuffix(cases[id], ".apex")
				if _, ok := cases[id]; !ok {
					t.Fatalf("no case %s", id)
				}
				if strings.Contains(istest[api], method+"=") && !strings.Contains(istest[api], "Assertion Failed: "+want) {
					t.Fatalf("native istest outcome differs from anonymous for %s: %s", id, istest[api])
				}
				// glade.yml test.permissionSets grants field access to test
				// runs only; the anonymous route cannot read the custom field
				// P__c without it, where the native capture assigned the set
				// to the running user. The IsTest route below covers x_rt.
				if !(id == "x_rt" && strings.Contains(gladeYML, "permissionSets")) {
					if got := runAnonymousRouteProbe(t, index, api, read(cases[id])); !strings.Contains(got, want) {
						t.Fatalf("NATIVE standard_recordtype*/org.tsv %s anonymous: want %q; Glade %q", api, want, got)
					}
				}
				if strings.Contains(istest[api], method+"=") {
					if got := runIsTestRouteProbe(t, index, "R5RtProbe", method); !strings.Contains(got, want) {
						t.Fatalf("NATIVE standard_recordtype*/org-istest.tsv %s istest: want %q; Glade %q", api, want, got)
					}
				}
			})
		}
	}
}
