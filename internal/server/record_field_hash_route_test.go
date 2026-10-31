package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native record/date rows: record_field_hash/org.tsv:4-7 F01/F02/F04/F07; SOAP does not serve API 62.
func TestSOAPApexRecordFieldHashNativeRoute(t *testing.T) {
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			for object, field := range map[string]string{"Contact": "LastName", "Opportunity": "Name", "Lead": "LastName"} {
				org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{field: {APIName: field, Type: storage.FieldString}}}, Records: make(map[storage.ID]storage.Record)}
			}
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>Date d=Date.newInstance(2030,1,1); Lead l=new Lead(LastName='Synthetic',Company='Example'); Opportunity o=new Opportunity(Name='Synthetic',StageName='Prospecting',CloseDate=d); System.assertEquals(-1371526565,System.hashCode(l)); System.assertEquals(822176235,System.hashCode(o)); System.assertEquals(-624577096,System.hashCode(d)); System.assertEquals(-624577065,System.hashCode(new List&lt;Date&gt;{d}));</apex:String></apex:executeAnonymous>`)
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
