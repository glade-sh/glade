package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/flow_record_lookup_parent_ref/org.tsv.
// Three record-triggered after-save Flows each run Get Records with a filter
// whose value is $Record.<Lookup>__r.<Field>: a Contact lookup on the parent
// compared with User.ContactId and with a custom Contact lookup, and a lookup
// field of another type. A null parent value matches the null field. At API
// 62.0 and 67.0 Salesforce found no User for a Contact, found the custom row,
// and found a User when the parent value was null.
func TestFlowRecordLookupParentRefNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_record_lookup_parent_ref"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	rows := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	apex := read("run.apex")
	if !strings.Contains(apex, "System.debug('R5|lookup|' + String.join(rows, ','));") {
		t.Fatal("fixture apex changed")
	}
	wantByAPI := map[string]string{}
	for line, row := range rows[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[0] != "lookup" || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || !strings.HasPrefix(cols[8], "cleaned") {
			t.Fatalf("invalid native row %d: %s", line+2, row)
		}
		wantByAPI[cols[1]] = strings.ReplaceAll(cols[5], "&#124;", "|")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want, ok := wantByAPI[api]
			if !ok {
				t.Fatalf("no native row for %s", api)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [R5L]\n")
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
			probe := "@IsTest private class FlowLookupProbe { @IsTest static void observe() {\n" +
				strings.ReplaceAll(apex, "System.debug('R5|lookup|' + String.join(rows, ','));", "System.assert(false, 'R5|lookup|' + String.join(rows, ','));") + "\n} }"
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FlowLookupProbe.cls"), probe)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"FlowLookupProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			observation := run.Suites[0].Cases[0]
			if observation.Problem == nil {
				t.Fatal("probe did not report")
			}
			got := observation.Problem.Message
			if !strings.Contains(got, want) {
				t.Fatalf("NATIVE flow_record_lookup_parent_ref/org.tsv %s anonymous: want %q; Glade %q", api, want, got)
			}
		})
	}
}
