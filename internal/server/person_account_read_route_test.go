package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
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
						Compiled         bool   `json:"compiled"`
						Success          bool   `json:"success"`
						ExceptionMessage string `json:"exceptionMessage"`
					}
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if !result.Compiled {
						t.Fatalf("unexpected compile rejection: %s", rec.Body.String())
					}
					if !enabled && field != "Name" {
						if result.Success || !strings.Contains(result.ExceptionMessage, `unknown field "`+field+`" on Account`) {
							t.Fatalf("unexpected runtime result: %s", rec.Body.String())
						}
					} else if !result.Success {
						t.Fatalf("unexpected runtime result: %s", rec.Body.String())
					}
				})
			}
		}
	}
}
