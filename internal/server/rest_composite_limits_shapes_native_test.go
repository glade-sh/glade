package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
)

// These are local composition twins of the captured direct limits response,
// not a claim that native Composite Graph accepts the same subrequests.
func TestRESTCompositeLimitsNativeShapes(t *testing.T) {
	file, err := os.Open("testdata/rest_resource_limits_shapes/org.tsv")
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
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if row[1] != "limits" {
			continue
		}
		if seen[row[0]] {
			t.Fatalf("duplicate native limits row for %s", row[0])
		}
		seen[row[0]] = true
		var native restRouteAnswer
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatal(err)
		}
		if native.Status != http.StatusOK || len(native.Keys) != 40 || !slices.IsSorted(native.Keys) {
			t.Fatal("native row must contain the captured 200 and first 40 sorted keys")
		}
		for _, route := range []string{"composite", "composite/batch", "composite/graph"} {
			t.Run(row[0]+"/"+route, func(t *testing.T) {
				org := testOrg()
				handler := New(&org)
				base := "/services/data/v" + row[0]
				// Repeat on the same handler, including both path spellings.
				for range 2 {
					for _, suffix := range []string{"", "/"} {
						subrequests := []map[string]any{{"method": "GET", "url": base + "/limits" + suffix, "referenceId": "limits"}}
						request := map[string]any{"compositeRequest": subrequests}
						switch route {
						case "composite/batch":
							request = map[string]any{"batchRequests": subrequests}
						case "composite/graph":
							request = map[string]any{"graphs": []any{map[string]any{"graphId": "limitsGraph", "compositeRequest": subrequests}}}
						}
						payload, err := json.Marshal(request)
						if err != nil {
							t.Fatal(err)
						}
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, base+"/"+route+suffix, bytes.NewReader(payload)))
						if rec.Code != http.StatusOK {
							t.Fatalf("outer status=%d body=%s", rec.Code, rec.Body.String())
						}
						var response struct {
							HasErrors         bool                        `json:"hasErrors"`
							CompositeResponse []compositeSubresponse      `json:"compositeResponse"`
							Results           []compositeBatchSubresponse `json:"results"`
							Graphs            []struct {
								GraphID       string `json:"graphId"`
								IsSuccessful  bool   `json:"isSuccessful"`
								GraphResponse struct {
									CompositeResponse []compositeSubresponse `json:"compositeResponse"`
								} `json:"graphResponse"`
							} `json:"graphs"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
							t.Fatal(err)
						}
						var body any
						if route == "composite/batch" {
							if response.HasErrors || len(response.Results) != 1 || response.Results[0].StatusCode != native.Status {
								t.Fatalf("limits batch response=%s", rec.Body.String())
							}
							body = response.Results[0].Result
						} else {
							responses := response.CompositeResponse
							if route == "composite/graph" {
								if len(response.Graphs) != 1 || response.Graphs[0].GraphID != "limitsGraph" || !response.Graphs[0].IsSuccessful {
									t.Fatalf("limits graph response=%s", rec.Body.String())
								}
								responses = response.Graphs[0].GraphResponse.CompositeResponse
							}
							if len(responses) != 1 || responses[0].ReferenceID != "limits" || responses[0].HTTPStatusCode != native.Status {
								t.Fatalf("limits composite response=%s", rec.Body.String())
							}
							body = responses[0].Body
						}
						limits, ok := body.(map[string]any)
						if !ok || len(limits) == 0 {
							t.Fatalf("expected nonempty limits object, got %#v", body)
						}
						for key := range limits {
							if key <= native.Keys[len(native.Keys)-1] && !slices.Contains(native.Keys, key) {
								t.Errorf("nonnative limits key %q; captured native keys %v", key, native.Keys)
							}
						}
					}
				}
			})
		}
	}
	if len(seen) != 4 || !seen["60.0"] || !seen["65.0"] || !seen["67.0"] || !seen["68.0"] {
		t.Fatalf("incomplete native limits version matrix: %v", seen)
	}
}
