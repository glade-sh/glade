package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Supplemental SOAP route for required_name_insert/org.tsv C137_015.
func TestSOAPApexRequiredInsertNativeRoute(t *testing.T) {
	for _, api := range []string{"65.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
			org.Objects["Account"] = account
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>String m; try { Database.insert(new List&lt;Account&gt;{new Account(Name='one'),new Account(Name='two'),new Account()},true); } catch(DmlException e) { m=e.getMessage(); } System.assertEquals('Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [Name]: [Name]',m);</apex:String></apex:executeAnonymous>`)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", api, 1), strings.NewReader(body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
			}
			result := decodeSOAPResult(t, rec.Body.String())
			if result["compiled"] != "true" || result["success"] != "true" {
				t.Fatalf("SOAP result: %#v", result)
			}
		})
	}
}
