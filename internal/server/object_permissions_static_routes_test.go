package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native static and runtime rows: case289_followups/static-adjacent.tsv.
func TestObjectPermissionsStaticToolingRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			storage.EnsureStandardObject(&org, "ObjectPermissions")
			source := `List<ObjectPermissions> r=[SELECT Id FROM ObjectPermissions ORDER BY PermissionsViewAllFields];`
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
			if !result.Compiled || result.Success || !strings.Contains(result.ExceptionMessage, "field 'PermissionsViewAllFields' can not be sorted in a query call") {
				t.Fatalf("native compile/runtime outcome: %s", rec.Body.String())
			}
		})
	}
}
