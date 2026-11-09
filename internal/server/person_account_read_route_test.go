package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Salesforce member-read observations are in person_account_read_shapes/org.tsv;
// this checks the local tooling transport and its API-version boundary.
func TestPersonAccountReadServerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, field := range []string{"FirstName", "LastName", "Name"} {
				t.Run(fmt.Sprintf("%s/enabled=%t/%s", api, enabled, field), func(t *testing.T) {
					org := testOrg()
					if enabled {
						storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					}
					source := `Account a = new Account(); String value = a.` + field + `; System.assertEquals(null, value);`
					payload, err := json.Marshal(map[string]string{"anonymousBody": source})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
					if api == "62.0" {
						assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unsupported API version v62.0")
						return
					}
					var result struct {
						Compiled       bool   `json:"compiled"`
						Success        bool   `json:"success"`
						CompileProblem string `json:"compileProblem"`
					}
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if !enabled && field != "Name" {
						if result.Compiled || result.Success || !strings.Contains(result.CompileProblem, "Variable does not exist: "+field) {
							t.Fatalf("unexpected compile result: %s", rec.Body.String())
						}
					} else if !result.Compiled || !result.Success {
						t.Fatalf("unexpected result: %s", rec.Body.String())
					}
				})
			}
		}
	}
}

// The live org, rather than a stale project feature profile, controls tooling
// anonymous analysis. The server must not mutate the shared project index.
func TestPersonAccountAnonymousUsesLiveOrgFeatures(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, field := range []string{"FirstName", "LastName", "PersonEmail"} {
			t.Run(fmt.Sprintf("enabled=%t/%s", enabled, field), func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, nil)
				if enabled {
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				}
				index := typesys.Index{}
				if !enabled {
					index.OrgShapeFeatures = []string{"PersonAccounts"}
				}
				server := New(&org)
				server.Index = &index
				payload, err := json.Marshal(map[string]string{"anonymousBody": `Account a = new Account(); String value = a.` + field + `;`})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(payload))))
				var result struct {
					Compiled       bool   `json:"compiled"`
					Success        bool   `json:"success"`
					CompileProblem string `json:"compileProblem"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if enabled && (!result.Compiled || !result.Success) {
					t.Fatalf("enabled: %s", rec.Body.String())
				}
				if !enabled && (result.Compiled || !strings.Contains(result.CompileProblem, "Variable does not exist: "+field)) {
					t.Fatalf("disabled: %s", rec.Body.String())
				}
				if enabled && len(index.OrgShapeFeatures) != 0 || !enabled && (len(index.OrgShapeFeatures) != 1 || index.OrgShapeFeatures[0] != "PersonAccounts") {
					t.Fatalf("project index mutated: %v", index.OrgShapeFeatures)
				}
			})
		}
	}
}
