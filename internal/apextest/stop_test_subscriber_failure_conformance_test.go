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

// Native APIs 62/67/68 pair an ordinary test outcome with a separate complete
// count observation. Assertion transport cannot stand in for whether a test fails.
func TestStopTestSubscriberFailureOrgConformance(t *testing.T) {
	var data struct {
		APIs  []string          `json:"apis"`
		Files map[string]string `json:"files"`
		Rows  []struct {
			API    string         `json:"api"`
			ID     string         `json:"id"`
			Counts map[string]int `json:"counts"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/stop_test_subscriber_failure.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,67.0,68.0" || len(data.Rows) != 36 {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range data.APIs {
		t.Run(api, func(t *testing.T) {
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
			if len(expected) != 12 {
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
					method := result.MethodName
					t.Run(method, func(t *testing.T) {
						split := strings.LastIndex(method, "_")
						if split < 0 {
							t.Fatalf("unexpected result %q", method)
						}
						id, kind := method[:split], method[split+1:]
						want := expected[id]
						if seen[method] || want == nil || (kind != "plain" && kind != "observe") {
							t.Fatalf("unexpected or duplicate result %q", method)
						}
						seen[method] = true
						if kind == "plain" {
							status := testreport.StatusPass
							if want["testFails"] == 1 {
								status = testreport.StatusFail
							}
							if result.Status != status {
								t.Fatalf("native testFails=%d; Glade %s: %+v", want["testFails"], result.Status, result.Problem)
							}
							return
						}
						if result.Status != testreport.StatusFail || result.Problem == nil {
							t.Fatalf("missing count transport: %+v", result)
						}
						_, counts, ok := strings.Cut(result.Problem.Message, "FAILURE|"+id+"|")
						if !ok {
							t.Fatalf("unexpected failure: %+v", result.Problem)
						}
						var actual map[string]int
						if err := json.Unmarshal([]byte(counts), &actual); err != nil {
							t.Fatal(err)
						}
						observedWant := map[string]int{}
						for field, value := range want {
							if field != "testFails" {
								observedWant[field] = value
							}
						}
						if !reflect.DeepEqual(actual, observedWant) {
							t.Fatalf("native %v; Glade %v", observedWant, actual)
						}
					})
				}
			}
			if len(seen) != 24 {
				t.Fatalf("observed %d of 24 named methods: %+v", len(seen), run.Summary())
			}
		})
	}
}
