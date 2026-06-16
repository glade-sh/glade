package server

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const nimbleAMSCompatSOAPPath = "/services/Soap/u/" + storage.DefaultRESTAPIVersion + "/00DLOCAL000000001"

func TestNimbleAMSRouteSetSupportsImportShape(t *testing.T) {
	org := nimbleAMSCompatOrg()
	addNimbleAMSAccount(t, &org, "acct-1", "Nimble Account")
	handler := New(&org)

	t.Run("REST query", func(t *testing.T) {
		query := url.QueryEscape("SELECT Id, Name, External_Id__c FROM Account WHERE External_Id__c = 'acct-1'")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/query?q="+query, nil))
		if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"totalSize":1`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"External_Id__c":"acct-1"`)) {
			t.Fatalf("REST query status = %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Partner SOAP describeSObjects", func(t *testing.T) {
		body := nimbleAMSSOAPEnvelope(`
			<urn:describeSObjects>
				<urn:sObjectType>Account</urn:sObjectType>
				<urn:sObjectType>Contact</urn:sObjectType>
			</urn:describeSObjects>`)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, nimbleAMSCompatSOAPPath, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("Partner SOAP describeSObjects status = %d body=%s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`Account`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`Contact`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`External_Id__c`)) {
			t.Fatalf("Partner SOAP describeSObjects missing object shape: %s", rec.Body.String())
		}
	})

	t.Run("Partner SOAP upsert", func(t *testing.T) {
		body := nimbleAMSSOAPEnvelope(`
			<urn:upsert>
				<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
				<urn:sObjects xsi:type="sf:Account">
					<sf:Name>SOAP Account</sf:Name>
					<sf:External_Id__c>soap-acct-1</sf:External_Id__c>
				</urn:sObjects>
			</urn:upsert>`)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, nimbleAMSCompatSOAPPath, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("Partner SOAP upsert status = %d body=%s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`success`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`true`)) {
			t.Fatalf("Partner SOAP upsert result = %s", rec.Body.String())
		}
	})

	t.Run("Tooling executeAnonymous", func(t *testing.T) {
		query := url.Values{"anonymousBody": {"insert new Account(Name = 'Cleaner Account', External_Id__c = 'cleaner-1');"}}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/tooling/executeAnonymous?"+query.Encode(), nil))
		if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) {
			t.Fatalf("Tooling executeAnonymous status = %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Bulk v1 insert and upsert", func(t *testing.T) {
		if !nimbleAMSCompatBulkV1Available(t, handler) {
			t.Skip("Bulk API v1 ingest is not available in this route set yet")
		}
		nimbleAMSCompatBulkV1CSV(t, handler, "insert", "Account", "Name,External_Id__c\nBulk Account,bulk-acct-1\n")
		nimbleAMSCompatBulkV1CSV(t, handler, "upsert", "Contact", "LastName,External_Id__c,Account.External_Id__c\nBulk Contact,bulk-contact-1,acct-1\n")
	})

	assertNimbleAMSAccountByExternalID(t, org, "acct-1")
	assertNimbleAMSAccountByExternalID(t, org, "cleaner-1")
}

func nimbleAMSCompatOrg() storage.OrgState {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":       {APIName: "LastName", Type: storage.FieldString, Required: true},
				"External_Id__c": {APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true},
				"AccountId":      {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
			Relations: []storage.Relationship{
				{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func addNimbleAMSAccount(t *testing.T, org *storage.OrgState, externalID, name string) {
	t.Helper()
	object := org.Objects["Account"]
	id := storage.ID(fmt.Sprintf("001%012d", len(object.Records)+1))
	object.Records[id] = storage.Record{
		ID:     id,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue(name),
			"External_Id__c": storage.StringValue(externalID),
		},
	}
	org.Objects["Account"] = object
}

func assertNimbleAMSAccountByExternalID(t *testing.T, org storage.OrgState, externalID string) {
	t.Helper()
	for _, record := range org.Objects["Account"].Records {
		if record.Fields["External_Id__c"].String == externalID {
			return
		}
	}
	t.Fatalf("Account External_Id__c %q not found in %#v", externalID, org.Objects["Account"].Records)
}

func nimbleAMSSOAPEnvelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:partner.soap.sforce.com" xmlns:sf="urn:sobject.partner.soap.sforce.com" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
	<soapenv:Body>` + body + `</soapenv:Body>
</soapenv:Envelope>`
}

func nimbleAMSCompatBulkV1Available(t *testing.T, handler http.Handler) bool {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/services/async/"+storage.DefaultRESTAPIVersion+"/job", strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
  <operation>insert</operation>
  <object>Account</object>
  <contentType>CSV</contentType>
</jobInfo>`))
	req.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotImplemented || bytes.Contains(rec.Body.Bytes(), []byte(`UNSUPPORTED_FEATURE`)) {
		return false
	}
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("Bulk API v1 job probe status = %d body=%s", rec.Code, rec.Body.String())
	}
	return true
}

func nimbleAMSCompatBulkV1CSV(t *testing.T, handler http.Handler, operation, objectName, csvBody string) {
	t.Helper()
	job := httptest.NewRecorder()
	jobReq := httptest.NewRequest(http.MethodPost, "/services/async/"+storage.DefaultRESTAPIVersion+"/job", strings.NewReader(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
  <operation>%s</operation>
  <object>%s</object>
  <contentType>CSV</contentType>
</jobInfo>`, operation, objectName)))
	jobReq.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(job, jobReq)
	if job.Code != http.StatusOK && job.Code != http.StatusCreated {
		t.Fatalf("Bulk API v1 %s %s job status = %d body=%s", operation, objectName, job.Code, job.Body.String())
	}
	jobID := nimbleAMSCompatXMLText(t, job.Body.Bytes(), "id")
	if jobID == "" {
		t.Fatalf("Bulk API v1 %s %s job id missing: %s", operation, objectName, job.Body.String())
	}

	batch := httptest.NewRecorder()
	batchReq := httptest.NewRequest(http.MethodPost, "/services/async/"+storage.DefaultRESTAPIVersion+"/job/"+jobID+"/batch", strings.NewReader(csvBody))
	batchReq.Header.Set("Content-Type", "text/csv")
	handler.ServeHTTP(batch, batchReq)
	if batch.Code != http.StatusOK && batch.Code != http.StatusCreated {
		t.Fatalf("Bulk API v1 %s %s batch status = %d body=%s", operation, objectName, batch.Code, batch.Body.String())
	}
}

func nimbleAMSCompatXMLText(t *testing.T, data []byte, local string) string {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != local {
			continue
		}
		var text string
		if err := decoder.DecodeElement(&text, &start); err != nil {
			t.Fatalf("decode XML %s: %v body=%s", local, err, string(data))
		}
		return strings.TrimSpace(text)
	}
}
