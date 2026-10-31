package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native list rows: sobject_list_hash/org.tsv:4-7, L01; SOAP does not serve API 62.
func TestSOAPApexSObjectListHashNativeRoute(t *testing.T) {
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			for object, field := range map[string]string{"Contact": "LastName", "Opportunity": "Name", "Lead": "LastName"} {
				org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{field: {APIName: field, Type: storage.FieldString}}}, Records: make(map[storage.ID]storage.Record)}
			}
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>System.assertEquals(31, System.hashCode(new List&lt;Account&gt;{new Account()})); System.assertEquals(31, System.hashCode(new List&lt;Contact&gt;{new Contact()})); System.assertEquals(31, System.hashCode(new List&lt;Opportunity&gt;{new Opportunity()})); System.assertEquals(31, System.hashCode(new List&lt;Lead&gt;{new Lead()}));</apex:String></apex:executeAnonymous>`)
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
