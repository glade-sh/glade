package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Supplemental SOAP route for empty_sobject_hash/org.tsv H03/H04 and adjacent_empty_sobject_hash/org.tsv H02 (62/67).
// SOAP version routing does not serve 62.0; exercise its supported 65.0 instead.
// API 68 expectations: sobject_hash_version/org.tsv:6-7.
func TestSOAPApexPopulatedSObjectHashNativeRoute(t *testing.T) {
	for _, api := range []string{"65.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			for object, field := range map[string]string{"Contact": "LastName", "Opportunity": "Name", "Lead": "LastName"} {
				org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{field: {APIName: field, Type: storage.FieldString}}}, Records: make(map[storage.ID]storage.Record)}
			}
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>System.assertEquals(-1304172146, System.hashCode(new Account(Name='Synthetic'))); System.assertEquals(515747140, System.hashCode(new Contact(LastName='Synthetic'))); System.assertEquals(2420395, System.hashCode(new Opportunity(Name=null))); System.assertEquals(-1394955679, System.hashCode(new Lead(LastName=null)));</apex:String></apex:executeAnonymous>`)
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
