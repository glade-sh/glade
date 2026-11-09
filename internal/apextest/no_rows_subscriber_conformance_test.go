package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNoRowsSubscriberNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "no_rows_subscriber", "SubscriberNoRowsProbeTest", "62.0", "67.0")
}

func TestNoRowsSubscriberDeliveryNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/no_rows_subscriber"
	f, err := os.Open(filepath.Join(dir, "delivery", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("invalid observations: %v", err)
	}
	for _, row := range rows[1:] {
		if len(row) != 3 || row[1] != "observeDelivery" {
			t.Fatal("invalid row")
		}
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"SubscriberNoRowsProbeTest"}, SelectedMethod: "observeDelivery", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "DELIVERY|")
			var got, want map[string]any
			if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[2], payload)
			}
		})
	}
}

func TestNoRowsSubscriberCacheTransitions(t *testing.T) {
	const fixture = "testdata/conformance/no_rows_subscriber/project"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			meta := filepath.Join(root, "force-app/main/default/triggers/SyntheticEventTrigger.trigger-meta.xml")
			for _, step := range []struct {
				status string
				count  float64
			}{{"Active", 1}, {"Inactive", 0}, {"Active", 1}} {
				writeFile(t, meta, `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>`+step.status+`</status></ApexTrigger>`)
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"SubscriberNoRowsProbeTest"}, SelectedMethod: "observe", Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
				var got map[string]any
				if !ok || json.Unmarshal([]byte(payload), &got) != nil || got["existing"] != step.count || got["unsubscribed"] != float64(0) || got["disabled"] != float64(0) {
					t.Fatalf("status %s: %s", step.status, payload)
				}
			}
		})
	}
}
