package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Supplemental SOAP route for empty_sobject_hash/org.tsv H02 and adjacent_empty_sobject_hash/org.tsv H01 (62/67).
// SOAP version routing does not serve 62.0; exercise its supported 65.0 instead.
// API 68 expectations: sobject_hash_version/org.tsv:6-7.
func TestSOAPApexEmptySObjectHashNativeRoute(t *testing.T) {
	for _, api := range []string{"65.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>System.assertEquals(0, System.hashCode(new Account())); System.assertEquals(0, System.hashCode(new Contact())); System.assertEquals(0, System.hashCode(new Opportunity())); System.assertEquals(0, System.hashCode(new Lead())); System.assertEquals(0, System.hashCode(new Case()));</apex:String></apex:executeAnonymous>`)
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
