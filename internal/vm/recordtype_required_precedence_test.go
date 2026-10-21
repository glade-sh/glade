package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type recordTypeRequiredPrecedenceCell struct {
	Shape, Mode      string
	RequiredSupplied bool
	Partial          struct {
		Success, IDPresent bool
		Errors             []recordTypeInsertError
	}
	Throwing struct {
		Success, IDPresent                       bool
		Type, Message, FirstStatus, FirstMessage string
		FirstFieldsNull                          bool
		FirstFields, FirstFieldNames             []string
	}
}

type recordTypeRequiredPrecedenceFixture struct {
	Captures      []recordTypeInsertCapture
	MetadataFiles []struct{ Path, Source, Sha256 string }
}

func loadRecordTypeRequiredPrecedenceFixture(t *testing.T) recordTypeRequiredPrecedenceFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/recordtype_required_precedence_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture recordTypeRequiredPrecedenceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestRecordTypeRequiredPrecedenceNativeFixtureIntegrity(t *testing.T) {
	fixture := loadRecordTypeRequiredPrecedenceFixture(t)
	if len(fixture.Captures) != 5 {
		t.Fatal("native capture scope changed")
	}
	digest := func(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }
	validHash := func(value string) bool {
		decoded, err := hex.DecodeString(value)
		return err == nil && len(decoded) == 32
	}
	seen := map[string]bool{}
	for _, capture := range fixture.Captures {
		key := capture.Context + "/" + capture.SourceAPI
		if (capture.Context == "anonymous" && capture.SourceAPI == "61.0") || (capture.Context != "istest" && capture.Context != "anonymous") || (capture.SourceAPI != "61.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0") || seen[key] {
			t.Fatal("unobserved or duplicate capture scope")
		}
		seen[key] = true
		if (capture.Context == "istest" && capture.TransportAPI != "67.0") || (capture.Context == "anonymous" && (capture.TransportAPI != capture.SourceAPI || capture.ObserverMetadata != "")) {
			t.Fatal("native transport/context changed")
		}
		if capture.DeploymentID == "" || !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSha256) {
			t.Fatal("incomplete native provenance")
		}
		for _, pair := range [][2]string{{capture.HelperSource, capture.HelperSourceSha256}, {capture.HelperMetadata, capture.HelperMetadataSha256}, {capture.ObserverSource, capture.ObserverSourceSha256}, {capture.ObserverMetadata, capture.ObserverMetadataSha256}, {capture.AnswerJSON, capture.AnswerSha256}} {
			if digest(pair[0]) != pair[1] {
				t.Fatal("native source or answer digest changed")
			}
		}
		pins := []string{capture.HelperMetadata}
		if capture.Context == "istest" {
			pins = append(pins, capture.ObserverMetadata)
		}
		for _, source := range pins {
			var metadata struct {
				API string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(source), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.API != capture.SourceAPI {
				t.Fatal("native source pin changed")
			}
		}
		var cells []recordTypeRequiredPrecedenceCell
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
			t.Fatal(err)
		}
		expected := 4
		if capture.SourceAPI == "67.0" {
			expected = 5
		}
		if capture.Context == "anonymous" {
			expected = 2
			if capture.SourceAPI == "67.0" {
				expected = 3
			}
		}
		if len(cells) != expected {
			t.Fatal("native cell scope changed")
		}
		for _, cell := range cells {
			if cell.Partial.Success != cell.Partial.IDPresent || cell.Partial.Success != cell.Throwing.Success || cell.Throwing.Success != cell.Throwing.IDPresent || (cell.Partial.Success && len(cell.Partial.Errors) != 0) || (!cell.Partial.Success && len(cell.Partial.Errors) != 1) {
				t.Fatal("native outcome observation changed")
			}
			// Full throwing-message headers and nullable getDmlFields remain in the
			// packet. Neither header formatting nor exception field-token/null
			// materialization is expanded by this precedence fix.
			if !cell.Throwing.Success && (!cell.Throwing.FirstFieldsNull || len(cell.Throwing.FirstFields) != 0) {
				t.Fatal("native nullable exception-field carry changed")
			}
		}
	}
	if len(fixture.MetadataFiles) == 0 {
		t.Fatal("native metadata missing")
	}
	for _, file := range fixture.MetadataFiles {
		if file.Path == "" || digest(file.Source) != file.Sha256 {
			t.Fatal("native metadata digest changed")
		}
	}
}

func TestRecordTypeRequiredPrecedenceNativeConformance(t *testing.T) {
	for _, capture := range loadRecordTypeRequiredPrecedenceFixture(t).Captures {
		var cells []recordTypeRequiredPrecedenceCell
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
			t.Fatal(err)
		}
		for _, cell := range cells {
			if capture.SourceAPI == "67.0" && cell.Mode == "default" && cell.Shape == "real" && cell.Throwing.FirstStatus == "INVALID_CROSS_REFERENCE_KEY" {
				// The original packet independently observes unavailable real record-type
				// denial in default USER_MODE. Actor record-type visibility is not changed
				// by the explicit-null/required-field precedence fix.
				t.Logf("carried native USER_MODE visibility cell: %s: %s", cell.Throwing.FirstStatus, cell.Throwing.FirstMessage)
				continue
			}
			for _, operation := range []string{"partial", "throwing"} {
				name := fmt.Sprintf("%s/%s/%s/%s/required=%t/%s", capture.SourceAPI, capture.Context, cell.Mode, cell.Shape, cell.RequiredSupplied, operation)
				t.Run(name, func(t *testing.T) {
					executeRecordTypeRequiredPrecedenceProbe(t, capture, recordTypeRequiredPrecedenceBody(cell, operation))
				})
			}
		}
	}
}

func recordTypeRequiredPrecedenceBody(cell recordTypeRequiredPrecedenceCell, operation string) string {
	literal := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "\\'") + "'" }
	body := "SObject row=RequiredChoice__c.SObjectType.newSObject();"
	if cell.Shape == "null" {
		body += "row.put('RecordTypeId',null);"
	}
	if cell.Shape == "real" {
		body += "row.put('RecordTypeId',RequiredChoice__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Alpha').getRecordTypeId());"
	}
	if cell.RequiredSupplied {
		body += "row.put('RequiredNote__c','Supplied');"
	}
	mode := ""
	if cell.Mode != "default" {
		mode = ",AccessLevel." + cell.Mode
	}
	if operation == "partial" {
		body += "Database.SaveResult result=Database.insert(new List<SObject>{row},false" + mode + ")[0];"
		body += fmt.Sprintf("System.assertEquals(%t,result.isSuccess());System.assertEquals(%t,result.getId()!=null);System.assertEquals(%d,result.getErrors().size());", cell.Partial.Success, cell.Partial.IDPresent, len(cell.Partial.Errors))
		for index, problem := range cell.Partial.Errors {
			prefix := fmt.Sprintf("result.getErrors()[%d]", index)
			body += "System.assertEquals(" + literal(problem.Status) + ",String.valueOf(" + prefix + ".getStatusCode()));System.assertEquals(" + literal(problem.Message) + "," + prefix + ".getMessage());"
			body += fmt.Sprintf("System.assertEquals(%d,%s.getFields().size());", len(problem.Fields), prefix)
			for n, field := range problem.Fields {
				body += fmt.Sprintf("System.assertEquals(%s,%s.getFields()[%d]);", literal(field), prefix, n)
			}
		}
	} else if cell.Throwing.Success {
		body += "Database.insert(new List<SObject>{row},true" + mode + ");"
	} else {
		body += "Boolean caught=false;try{Database.insert(new List<SObject>{row},true" + mode + ");}catch(DmlException problem){caught=true;"
		for _, assertion := range [][2]string{{cell.Throwing.Type, "problem.getTypeName()"}, {cell.Throwing.FirstStatus, "String.valueOf(problem.getDmlStatusCode(0))"}, {cell.Throwing.FirstMessage, "problem.getDmlMessage(0)"}} {
			body += "System.assertEquals(" + literal(assertion[0]) + "," + assertion[1] + ");"
		}
		body += fmt.Sprintf("System.assertEquals(%d,problem.getDmlFieldNames(0).size());", len(cell.Throwing.FirstFieldNames))
		for n, field := range cell.Throwing.FirstFieldNames {
			body += fmt.Sprintf("System.assertEquals(%s,problem.getDmlFieldNames(0)[%d]);", literal(field), n)
		}
		body += "}System.assert(caught);"
	}
	success := cell.Partial.Success
	if operation == "throwing" {
		success = cell.Throwing.IDPresent
	}
	count := 0
	if success {
		count = 1
	}
	return body + fmt.Sprintf("System.assertEquals(%t,row.Id!=null);System.assertEquals(%d,[SELECT COUNT() FROM RequiredChoice__c]);", success, count)
}

func executeRecordTypeRequiredPrecedenceProbe(t *testing.T, capture recordTypeInsertCapture, body string) {
	t.Helper()
	org := testDataOrg()
	org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
	var types []storage.RecordTypeInfo
	for index, name := range []string{"Alpha", "Beta", "Gamma"} {
		id := storage.ID(fmt.Sprintf("012000000000%03d", index+101))
		types = append(types, storage.RecordTypeInfo{ID: id, Name: name, DeveloperName: name, Active: true, Available: false})
		object := org.Objects["RecordType"]
		object.Records[id] = storage.Record{Object: "RecordType", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
		org.Objects["RecordType"] = object
	}
	org.Objects["RequiredChoice__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RequiredChoice__c", KeyPrefix: "a01", RecordTypes: types, Fields: map[string]storage.Field{
		"Name":            {APIName: "Name", Type: storage.FieldString, Required: true, AutoNumber: true, DisplayFormat: "ROW-{0000}"},
		"RecordTypeId":    {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
		"RequiredNote__c": {APIName: "RequiredNote__c", Type: storage.FieldString, Required: true, Length: 20},
	}}, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.SetOrg(&org)
	if capture.Context == "istest" {
		machine.EnableTestContext()
	}
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: capture.SourceAPI})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "RequiredPrecedenceProbe", APIVersion: capture.SourceAPI}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "RequiredPrecedenceProbe.observe", ClassName: "RequiredPrecedenceProbe", IsStatic: true, Access: "public", ReturnType: "void", APIVersion: capture.SourceAPI, SourceContextBound: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	invoke, err := CompileAnonymousWithOptions("RequiredPrecedenceProbe.observe();", CompileOptions{APIVersion: capture.SourceAPI})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(invoke); err != nil {
		t.Fatal(err)
	}
}
