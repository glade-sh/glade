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

// NATIVE rows: testdata/conformance/standard_recordtype_picklist_default/org.tsv.
// A record type's default picklist value applies on insert, with an explicit
// RecordTypeId or through the user's default record type, for custom and
// standard objects, and it satisfies a required standard picklist. The same
// rows record that UnitOfMeasure.Type is defaulted by the org's feature set
// (PublicSectorAccess here). Salesforce, API 62.0 and 67.0.
func TestStandardRecordTypePicklistDefaultNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/standard_recordtype_picklist_default"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	rows := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	cases := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")[1:] {
		cols := strings.Split(line, "\t")
		cases[cols[0]] = cols[2]
	}
	debug := regexp.MustCompile(`System\.debug\(`)
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
		writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PublicSectorAccess]\ntest:\n  permissionSets: [RTP]\n")
		err := filepath.WalkDir(filepath.Join(fixture, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(root, name), read(filepath.Join("project", name)))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for line, row := range rows[1:] {
			cols := strings.Split(row, "\t")
			if len(cols) < 9 || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || !strings.HasPrefix(cols[8], "cleaned") {
				t.Fatalf("invalid native row %d: %s", line+2, row)
			}
			if cols[1] != api {
				continue
			}
			order = append(order, cols[0])
			body := "List<String> r5out = new List<String>();\n" + debug.ReplaceAllString(read(cases[cols[0]]), "r5out.add(") + "\nSystem.assert(false, String.join(r5out, ';'));"
			class := "Probe" + strings.ReplaceAll(cols[0], "_", "")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), "@IsTest private class "+class+" { @IsTest static void observe() {\n"+body+"\n} }")
		}
		index := loadTestIndex(t, root)
		for _, row := range rows[1:] {
			cols := strings.Split(row, "\t")
			if cols[1] != api {
				continue
			}
			id, want := cols[0], strings.ReplaceAll(cols[5], "&#124;", "|")
			t.Run(api+"/"+id, func(t *testing.T) {
				class := "Probe" + strings.ReplaceAll(id, "_", "")
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected test selection: %+v", run.Summary())
				}
				observation := run.Suites[0].Cases[0]
				if observation.Problem == nil {
					t.Fatal("probe did not report")
				}
				if got := observation.Problem.Message; !strings.Contains(got, want) {
					t.Fatalf("NATIVE standard_recordtype_picklist_default/org.tsv %s anonymous: want %q; Glade %q", api, want, got)
				}
			})
		}
		_ = order
	}
}
