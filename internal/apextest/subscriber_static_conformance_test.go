package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// The native named tests use PublishImmediately at source APIs 62, 67 and 68.
// Their source is retained in full; only direct publication methods are run.
// Compare static state only. Other captured fields describe delivery phases,
// publication results or transaction boundaries and are outside this contract.
func TestSubscriberStaticNativeConformance(t *testing.T) {
	var data struct {
		APIs            []string          `json:"apis"`
		Context         string            `json:"context"`
		PublishBehavior string            `json:"publishBehavior"`
		Files           map[string]string `json:"files"`
		Facets          []string          `json:"facets"`
		Rows            []struct {
			API    string         `json:"api"`
			ID     string         `json:"id"`
			Counts map[string]int `json:"counts"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/subscriber_static.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,67.0,68.0" ||
		data.Context != "named @IsTest" || data.PublishBehavior != "PublishImmediately" ||
		strings.Join(data.Facets, ",") != "triggerSeed,testHits,testSeed" || len(data.Rows) != 12 {
		t.Fatal("incomplete native static observations")
	}
	methods := []string{"direct_stop", "direct_before", "direct_after", "direct_both"}
	for _, api := range data.APIs {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			expected := map[string]map[string]int{}
			for _, row := range data.Rows {
				if row.API != api {
					continue
				}
				if expected[row.ID] != nil || len(row.Counts) != len(data.Facets) {
					t.Fatalf("invalid native row %s", row.ID)
				}
				for _, facet := range data.Facets {
					if _, ok := row.Counts[facet]; !ok {
						t.Fatalf("missing native static facet %s/%s", row.ID, facet)
					}
				}
				expected[row.ID] = row.Counts
			}
			if len(expected) != len(methods) {
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
			index := loadTestIndex(t, root)
			for _, method := range methods {
				t.Run(method, func(t *testing.T) {
					want := expected[method]
					if want == nil {
						t.Fatal("missing native method")
					}
					run := Run(index, Options{SelectedClasses: []string{"DeliveryProbeTest"}, SelectedMethod: method, NoDiskCache: true})
					seen := 0
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							seen++
							if result.MethodName != method || result.Status != testreport.StatusFail || result.Problem == nil {
								t.Fatalf("missing native observation transport: %+v", result)
							}
							_, payload, ok := strings.Cut(result.Problem.Message, "DELIVERY|"+method+"|")
							if !ok {
								t.Fatalf("unexpected failure: %+v", result.Problem)
							}
							var actual map[string]int
							if err := json.Unmarshal([]byte(payload), &actual); err != nil {
								t.Fatal(err)
							}
							for _, facet := range data.Facets {
								got, found := actual[facet]
								if !found || got != want[facet] {
									t.Errorf("%s: native %d; Glade %d (present %t)", facet, want[facet], got, found)
								}
							}
						}
					}
					if seen != 1 {
						t.Fatalf("observed %d named methods; summary %+v", seen, run.Summary())
					}
				})
			}
		})
	}
}
