package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Each native observation is one complete assertion result from a named test.
// These owned fixtures cover PublishImmediately at source APIs 62, 67 and 68.
// The intentional assertion failure transports counts; every count is compared.
func TestStopTestEventDeliveryOrgConformance(t *testing.T) {
	var data struct {
		APIs  []string          `json:"apis"`
		Files map[string]string `json:"files"`
		Rows  []struct {
			API    string         `json:"api"`
			ID     string         `json:"id"`
			Counts map[string]int `json:"counts"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/stop_test_event_delivery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,67.0,68.0" || len(data.Rows) != 48 {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range data.APIs {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			expected := map[string]map[string]int{}
			for _, row := range data.Rows {
				if row.API != api {
					continue
				}
				if expected[row.ID] != nil || len(row.Counts) != 13 {
					t.Fatalf("invalid native row %s", row.ID)
				}
				expected[row.ID] = row.Counts
			}
			if len(expected) != 16 {
				t.Fatal("incomplete native API")
			}
			root := t.TempDir()
			for name, body := range data.Files {
				if strings.HasSuffix(name, "-meta.xml") {
					body = strings.ReplaceAll(body, "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", name), body+"\n")
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			seen := map[string]bool{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := result.MethodName
					t.Run(id, func(t *testing.T) {
						if seen[id] || expected[id] == nil {
							t.Fatalf("unexpected or duplicate result %q", id)
						}
						seen[id] = true
						if result.Status != testreport.StatusFail || result.Problem == nil {
							t.Fatalf("missing count transport: %+v", result)
						}
						_, counts, ok := strings.Cut(result.Problem.Message, "DELIVERY|"+id+"|")
						if !ok {
							t.Fatalf("unexpected failure: %+v", result.Problem)
						}
						var actual map[string]int
						if err := json.Unmarshal([]byte(counts), &actual); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(actual, expected[id]) {
							t.Fatalf("native %v; Glade %v", expected[id], actual)
						}
					})
				}
			}
			if len(seen) != 16 {
				t.Fatalf("observed %d of 16 named rows: %+v", len(seen), run.Summary())
			}
		})
	}
}
