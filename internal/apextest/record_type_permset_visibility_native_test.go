package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/record_type_permset_visibility/org.tsv. A
// record type the System Administrator profile marks not visible is available
// to the running user when an assigned permission set grants it, and stays
// unavailable when the granting set is not assigned, at API 62.0 and 67.0.
func TestRecordTypePermsetVisibilityNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/record_type_permset_visibility"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	for _, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "istest" || cols[3] != "true" {
			t.Fatalf("invalid native row: %s", row)
		}
		api := cols[1]
		want := strings.TrimPrefix(strings.ReplaceAll(cols[5], "&#124;", "|"), "avail=Fail System.AssertException: Assertion Failed: ")
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [P1]\n")
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
			run := Run(index, Options{SelectedClasses: []string{"R5RtProbe"}, SelectedMethod: "avail", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			got := strings.TrimPrefix(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
			if got != want {
				t.Fatalf("NATIVE record_type_permset_visibility/org.tsv %s istest avail:\nwant %q\nGlade %q", api, want, got)
			}
		})
	}
}
