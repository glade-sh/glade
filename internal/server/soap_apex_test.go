package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const soapApexTestPath = "/services/Soap/s/65.0/00DLOCAL000000001"

func TestSOAPApexExecuteAnonymousInsertsIntoLocalOrg(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, soapApexTestPath, strings.NewReader(soapEnvelope(`
		<apex:executeAnonymous>
			<apex:String>insert new Account(Name = 'SOAP Apex');</apex:String>
		</apex:executeAnonymous>`))))

	if rec.Code != http.StatusOK {
		t.Fatalf("SOAP executeAnonymous status = %d body=%s", rec.Code, rec.Body.String())
	}
	result := decodeSOAPResult(t, rec.Body.String())
	if result["compiled"] != "true" || result["success"] != "true" || result["line"] != "-1" || result["column"] != "-1" {
		t.Fatalf("SOAP executeAnonymous result = %#v body=%s", result, rec.Body.String())
	}
	if result["compileProblem"] != "" || result["exceptionMessage"] != "" || result["exceptionStackTrace"] != "" {
		t.Fatalf("SOAP executeAnonymous failure fields = %#v", result)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("records after SOAP executeAnonymous = %#v", org.Objects["Account"].Records)
	}
}

func TestSOAPApexExecuteAnonymousAcceptsSalesforceCLIShape(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, soapApexTestPath, strings.NewReader(soapEnvelope(`
		<executeAnonymous xmlns="http://soap.sforce.com/2006/08/apex">
			<apexcode>insert new Account(Name = 'SF CLI Apex');</apexcode>
		</executeAnonymous>`))))

	if rec.Code != http.StatusOK {
		t.Fatalf("SOAP executeAnonymous CLI shape status = %d body=%s", rec.Code, rec.Body.String())
	}
	result := decodeSOAPResult(t, rec.Body.String())
	if result["compiled"] != "true" || result["success"] != "true" {
		t.Fatalf("SOAP executeAnonymous CLI shape result = %#v body=%s", result, rec.Body.String())
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("records after SOAP executeAnonymous CLI shape = %#v", org.Objects["Account"].Records)
	}
}

func TestSOAPApexExecuteAnonymousCompileFailureReturnsSOAPResult(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, soapApexTestPath, strings.NewReader(soapEnvelope(`
		<apex:executeAnonymous>
			<apex:String>Account a = ; insert new Account(Name = 'No Commit');</apex:String>
		</apex:executeAnonymous>`))))

	if rec.Code != http.StatusOK {
		t.Fatalf("SOAP executeAnonymous compile status = %d body=%s", rec.Code, rec.Body.String())
	}
	result := decodeSOAPResult(t, rec.Body.String())
	if result["compiled"] != "false" || result["success"] != "false" || result["line"] != "1" || result["column"] != "1" {
		t.Fatalf("SOAP compile failure result = %#v body=%s", result, rec.Body.String())
	}
	if result["compileProblem"] == "" {
		t.Fatalf("SOAP compileProblem missing: %#v body=%s", result, rec.Body.String())
	}
	if result["exceptionMessage"] != "" || result["exceptionStackTrace"] != "" {
		t.Fatalf("SOAP compile failure exception fields = %#v", result)
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("compile failure committed records = %#v", org.Objects["Account"].Records)
	}
}

func TestSOAPApexExecuteAnonymousVersion60IsExplicitlyUnsupported(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/60.0/00DLOCAL000000001", strings.NewReader(soapEnvelope(`
		<apex:executeAnonymous><apex:String>System.debug('legacy');</apex:String></apex:executeAnonymous>`))))
	result := decodeSOAPResult(t, rec.Body.String())
	if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "unsupported source API version") {
		t.Fatalf("status = %d result=%#v body=%s", rec.Code, result, rec.Body.String())
	}
}

func TestSOAPApexExecuteAnonymousPreservesURIVersion(t *testing.T) {
	org := testOrg()
	org.APIVersion = "65.0"
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/67.0/00DLOCAL000000001", strings.NewReader(soapEnvelope(`
		<apex:executeAnonymous><apex:String>List&lt;Account&gt; rows = [SELECT Id FROM Account WITH SECURITY_ENFORCED];</apex:String></apex:executeAnonymous>`))))
	result := decodeSOAPResult(t, rec.Body.String())
	if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "WITH SECURITY_ENFORCED") {
		t.Fatalf("status = %d result=%#v body=%s", rec.Code, result, rec.Body.String())
	}
}

func TestSOAPApexRejectsUnknownMethod(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, soapApexTestPath, strings.NewReader(soapEnvelope(`
		<apex:notExecuteAnonymous>
			<apex:String>System.debug('nope');</apex:String>
		</apex:notExecuteAnonymous>`))))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unknown SOAP method status = %d body=%s", rec.Code, rec.Body.String())
	}
	fault := decodeSOAPFault(t, rec.Body.String())
	if fault["faultcode"] == "" || !strings.Contains(fault["faultstring"], "unknown SOAP method") {
		t.Fatalf("unknown SOAP method fault = %#v body=%s", fault, rec.Body.String())
	}
}

func soapEnvelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:apex="urn:partner.soap.sforce.com">
	<soapenv:Body>` + body + `</soapenv:Body>
</soapenv:Envelope>`
}

func decodeSOAPResult(t *testing.T, body string) map[string]string {
	t.Helper()
	fields := map[string]string{}
	decoder := xml.NewDecoder(strings.NewReader(body))
	var inResult bool
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "result" {
				inResult = true
				continue
			}
			if !inResult {
				continue
			}
			var value string
			if err := decoder.DecodeElement(&value, &token); err != nil {
				t.Fatal(err)
			}
			fields[token.Name.Local] = value
		case xml.EndElement:
			if token.Name.Local == "result" {
				inResult = false
			}
		}
	}
	if len(fields) == 0 {
		t.Fatalf("SOAP result missing: %s", body)
	}
	return fields
}

func decodeSOAPFault(t *testing.T, body string) map[string]string {
	t.Helper()
	fields := map[string]string{}
	decoder := xml.NewDecoder(strings.NewReader(body))
	var inFault bool
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "Fault" {
				inFault = true
				continue
			}
			if !inFault {
				continue
			}
			var value string
			if err := decoder.DecodeElement(&value, &token); err != nil {
				t.Fatal(err)
			}
			fields[token.Name.Local] = value
		case xml.EndElement:
			if token.Name.Local == "Fault" {
				inFault = false
			}
		}
	}
	if len(fields) == 0 {
		t.Fatalf("SOAP fault missing: %s", body)
	}
	return fields
}
