package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Native observations keep only the explicit describe and operation outcomes;
// org-specific identities, records, and stack traces are deliberately omitted.
func TestPermsetMutingReadOnlyNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/permset_muting_read_only"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 {
		t.Fatalf("native rows=%d, want 10", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, "testdata/conformance/permset_muting/project", api)
			mute, err := os.ReadFile(filepath.Join(fixture, "muting.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/mutingpermissionsets/M01_FieldEditMute.mutingpermissionset-meta.xml"), string(mute))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				row := row
				t.Run(row[1], func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"PermMuteObserve"}, SelectedMethod: "M01_FieldEdit_" + row[1], NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("missing test result: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing observation assertion: %+v", problem)
					}
					_, payload, ok := strings.Cut(problem.Message, "OBS|")
					if !ok {
						t.Fatalf("missing OBS payload: %v", problem)
					}
					var got, want map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					for key, expected := range want {
						observedKey := key
						if key == "removedFields" {
							observedKey = "getRemovedFields"
						}
						if !reflect.DeepEqual(got[observedKey], expected) {
							t.Errorf("%s: native=%v Glade=%v", key, expected, got[observedKey])
						}
					}
				})
			}
		})
	}
}
