package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The DML engine regression preserves the earlier direct-route behavior; the
// related Apex controls are in person_account_followups/review2/org.tsv:2-3,86-87.
func TestFlowDirectInsertRecordTypeProvenanceServer(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"default", `{"LastName":null}`, "REQUIRED_FIELD_MISSING"},
		{"explicit", `{"RecordTypeId":"012000000000101","LastName":null}`, "INVALID_FIELD_FOR_INSERT_UPDATE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
			org.Objects["Account"] = account
			req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/sobjects/Account", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			New(&org).ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s want %s", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
}
