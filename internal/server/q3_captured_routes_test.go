package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// q3semijoin/org.tsv:2-5 and q3share/org.tsv:2-5 contain native source results.
func TestQ3CapturedServerRoutes(t *testing.T) {
	for _, tc := range []struct{ name, path string }{{"child", "../apextest/testdata/conformance/q3semijoin/seeded.apex"}, {"share", "../apextest/testdata/conformance/q3share/standard.apex"}} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			for _, name := range []string{"Account", "Contact", "Opportunity", "OpportunityContactRole", "AccountShare", "OpportunityShare"} {
				storage.EnsureStandardObject(&org, name)
			}
			handler := New(&org)
			for _, api := range []string{"62.0", "67.0"} {
				for attempt := 0; attempt < 2; attempt++ {
					payload, err := json.Marshal(map[string]string{"anonymousBody": string(body)})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
					if api == "62.0" {
						assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unsupported API version v62.0")
						continue
					}
					var result struct {
						Compiled bool `json:"compiled"`
						Success  bool `json:"success"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || !result.Compiled || !result.Success {
						t.Fatalf("%s run %d status=%d body=%s", api, attempt, rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
}

func TestQ3SharingReasonRejectionsOnServer(t *testing.T) {
	for _, name := range []string{"reject.apex", "variable.apex"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../apextest/testdata/conformance/q3share/" + name)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "AccountShare")
			payload, err := json.Marshal(map[string]string{"anonymousBody": string(source)})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(payload))))
			var result struct {
				Compiled bool `json:"compiled"`
				Success  bool `json:"success"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || result.Compiled || result.Success {
				t.Fatalf("native compile rejection was not matched: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
