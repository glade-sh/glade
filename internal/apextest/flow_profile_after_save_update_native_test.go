package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/flow_profile_after_save_update/org.tsv.
// Six record-triggered after-save Update Flows read $Profile.Name (EqualTo and
// NotEqualTo, alone and with a $Record criterion that compares to a variable).
// Salesforce ran exactly the Flows whose decision held for the running System
// Administrator, at API 62.0 and 67.0.
func TestFlowProfileAfterSaveUpdateNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_profile_after_save_update"
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
	if !strings.Contains(apex, "System.debug('R5|flow|' + s);") {
		t.Fatal("fixture apex changed")
	}
	wantByAPI := map[string]string{}
	for line, row := range rows[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[0] != "flow" || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || !strings.HasPrefix(cols[8], "cleaned") {
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
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [R5F]\n")
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
			probe := "@IsTest private class FlowProfileProbe { @IsTest static void observe() {\n" +
				strings.ReplaceAll(apex, "System.debug('R5|flow|' + s);", "System.assert(false, 'R5|flow|' + s);") + "\n} }"
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/FlowProfileProbe.cls"), probe)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"FlowProfileProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			observation := run.Suites[0].Cases[0]
			if observation.Problem == nil {
				t.Fatal("probe did not report")
			}
			got := observation.Problem.Message
			if !strings.Contains(got, want) {
				t.Fatalf("NATIVE flow_profile_after_save_update/org.tsv %s anonymous: want %q; Glade %q", api, want, got)
			}
		})
	}
}
