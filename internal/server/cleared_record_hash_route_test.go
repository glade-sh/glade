package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native cleared/null Id rows: cleared_record_hash/org.tsv:4-7 C01/C05; SOAP does not serve API 62.
func TestSOAPApexClearedRecordHashNativeRoute(t *testing.T) {
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			for object, field := range map[string]string{"Contact": "LastName", "Opportunity": "Name", "Lead": "LastName"} {
				org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{field: {APIName: field, Type: storage.FieldString}}}, Records: make(map[storage.ID]storage.Record)}
			}
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>Account a=new Account(Id=null); Contact c=new Contact(LastName='Synthetic',Id=null); System.assertEquals(false,a.equals(new Account())); System.assertEquals(2363,System.hashCode(a)); System.assertEquals(2394,System.hashCode(new List&lt;Account&gt;{a})); System.assertEquals(515749503,System.hashCode(c));</apex:String></apex:executeAnonymous>`)
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
