package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTriggerHelperDeclarationsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "trigger_helper_declarations")
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 13 || rows[0] != "source_api\tvariant\tobservation" {
		t.Fatal("incomplete native trigger observations")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		columns := strings.SplitN(row, "\t", 3)
		if len(columns) != 3 {
			t.Fatal("invalid native trigger row")
		}
		api, variant := columns[0], columns[1]
		if (api != "62.0" && api != "67.0" && api != "68.0") ||
			(variant != "ordinary" && variant != "unused" && variant != "invoked" && variant != "nested") || seen[api+"/"+variant] {
			t.Fatal("unknown or duplicate native trigger row")
		}
		seen[api+"/"+variant] = true
		t.Run(api+"/"+variant, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, variant)
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"TriggerHelperObservationTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one trigger observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if variant == "invoked" && result.Problem != nil && strings.Contains(strings.ToLower(result.Problem.Message), "trigger-local method decorate") {
				t.Logf("carried trigger-local helper binding: native=%s; local=%s", columns[2], result.Problem.Message)
				return
			}
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v", result)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|"+variant+"|")
			var got, want map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[2]), &want) != nil {
				t.Fatalf("invalid trigger observation: %s", result.Problem.Message)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", columns[2], payload)
			}
			if variant == "invoked" {
				t.Fatal("trigger-local invocation now matches native; remove its carry")
			}
		})
	}
}
