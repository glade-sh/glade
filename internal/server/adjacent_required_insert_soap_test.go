package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Supplemental SOAP route for insert_event_child/org.tsv:2-5 D1/D5.
func TestSOAPApexAdjacentRequiredInsertNativeRoute(t *testing.T) {
	for _, api := range []string{"65.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
			org.Objects["Account"] = account
			for object, field := range map[string]string{"Contact": "LastName", "Opportunity": "Name"} {
				org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{field: {APIName: field, Type: storage.FieldString, Required: true}}}, Records: make(map[storage.ID]storage.Record)}
			}
			opportunity := org.Objects["Opportunity"]
			opportunity.Definition.Fields["StageName"] = storage.Field{APIName: "StageName", Type: storage.FieldString}
			opportunity.Definition.Fields["CloseDate"] = storage.Field{APIName: "CloseDate", Type: storage.FieldDate}
			org.Objects["Opportunity"] = opportunity
			handler := New(&org)
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>String a; String b; try { Database.insert(new List&lt;Contact&gt;{new Contact(LastName='One'),new Contact(LastName='Two'),new Contact()},true); } catch(DmlException e) { a=e.getMessage(); } try { Database.insert(new List&lt;Opportunity&gt;{new Opportunity(Name='One',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(Name='Two',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(StageName='Prospecting',CloseDate=Date.today())},true,AccessLevel.USER_MODE); } catch(DmlException e) { b=e.getMessage(); } System.assertEquals('Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [LastName]: [LastName]',a); System.assertEquals('Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [Name]: [Name]',b);</apex:String></apex:executeAnonymous>`)
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
