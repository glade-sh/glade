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

type recordTypeInsertCapture struct {
	Context, SourceAPI, TransportAPI                                                       string
	OrgIDHash                                                                              string `json:"orgIdSHA256"`
	DeploymentID, Status                                                                   string
	HelperSource, HelperMetadata, ObserverSource, ObserverMetadata                         string
	HelperSourceSha256, HelperMetadataSha256, ObserverSourceSha256, ObserverMetadataSha256 string
	RawResponseSha256, AnswerJSON, AnswerSha256                                            string
}

type recordTypeInsertFixture struct {
	Captures      []recordTypeInsertCapture
	ModeCaptures  []recordTypeInsertCapture
	MetadataFiles []struct{ Path, Source, Sha256 string }
}

type recordTypeInsertSnapshot struct {
	RecordType string
	IDPresent  bool
}

type recordTypeInsertError struct {
	Message, Status string
	Fields          []string
}

type recordTypeInsertCell struct {
	Shape                                string
	CallerBefore, CallerAfter, Persisted recordTypeInsertSnapshot
	Partial                              struct {
		Success, IDPresent bool
		Errors             []recordTypeInsertError
	}
	Throwing struct {
		Success                    bool
		Type, Message, FirstStatus string
		FirstFields                []string
		Caller                     recordTypeInsertSnapshot
	}
}

func loadRecordTypeInsertFixture(t *testing.T) recordTypeInsertFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/recordtype_insert_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture recordTypeInsertFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestRecordTypeInsertNativeFixtureIntegrity(t *testing.T) {
	fixture := loadRecordTypeInsertFixture(t)
	if len(fixture.Captures) != 6 {
		t.Fatal("native capture scope changed")
	}
	digest := func(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }
	validHash := func(hash string) bool { data, err := hex.DecodeString(hash); return err == nil && len(data) == 32 }
	seen := map[string]bool{}
	for _, capture := range append(append([]recordTypeInsertCapture{}, fixture.Captures...), fixture.ModeCaptures...) {
		key := capture.SourceAPI + "/" + capture.Context
		if strings.Contains(capture.HelperSource, "AccessLevel.SYSTEM_MODE") {
			key += "/modes"
		}
		if seen[key] {
			t.Fatalf("duplicate native capture: %s", key)
		}
		seen[key] = true
		if capture.SourceAPI != "61.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0" {
			t.Fatal("unobserved source API")
		}
		if capture.DeploymentID == "" || !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSha256) {
			t.Fatalf("incomplete provenance: %s", key)
		}
		for _, pair := range [][2]string{{capture.HelperSource, capture.HelperSourceSha256}, {capture.HelperMetadata, capture.HelperMetadataSha256}, {capture.ObserverSource, capture.ObserverSourceSha256}, {capture.ObserverMetadata, capture.ObserverMetadataSha256}, {capture.AnswerJSON, capture.AnswerSha256}} {
			if digest(pair[0]) != pair[1] {
				t.Fatalf("native source or answer digest changed: %s", key)
			}
		}
		checkPin := func(metadata string) {
			t.Helper()
			var pin struct {
				API string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(metadata), &pin); err != nil {
				t.Fatal(err)
			}
			if pin.API != capture.SourceAPI {
				t.Fatalf("source pin mismatch: %s", key)
			}
		}
		checkPin(capture.HelperMetadata)
		switch capture.Context {
		case "istest":
			if capture.TransportAPI != "67.0" {
				t.Fatal("named transport changed")
			}
			checkPin(capture.ObserverMetadata)
		case "anonymous":
			if capture.TransportAPI != capture.SourceAPI || capture.ObserverMetadata != "" {
				t.Fatal("anonymous source/transport mismatch")
			}
		default:
			t.Fatal("unobserved capture context")
		}
		if strings.Contains(capture.HelperSource, "AccessLevel.SYSTEM_MODE") {
			var cells []struct{ Mode, Shape string }
			if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
				t.Fatal(err)
			}
			if len(cells) != 4 {
				t.Fatal("native access-mode scope changed")
			}
			continue
		}
		var observed map[string]struct{ Cases []recordTypeInsertCell }
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"RTZero__c", "RTOne__c", "RTThree__c"} {
			if len(observed[object].Cases) == 0 {
				t.Fatalf("missing native object: %s", object)
			}
		}
	}
	if len(fixture.MetadataFiles) == 0 {
		t.Fatal("native custom-object metadata missing")
	}
	for _, file := range fixture.MetadataFiles {
		if file.Path == "" || digest(file.Source) != file.Sha256 {
			t.Fatal("native metadata digest changed")
		}
	}
}

// This replay admits DML outcomes and caller/query values. Describe ordering,
// availability/default mappings, JSON null-presence, and USER_MODE unavailable
// record-type denials remain separate captured observations, not assertions here.
func TestRecordTypeInsertNativeConformance(t *testing.T) {
	for _, capture := range loadRecordTypeInsertFixture(t).Captures {
		var objects map[string]struct{ Cases []recordTypeInsertCell }
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &objects); err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"RTZero__c", "RTOne__c", "RTThree__c"} {
			for _, cell := range objects[object].Cases {
				if object == "RTZero__c" && cell.Shape != "omitted" {
					continue
				}
				if capture.SourceAPI == "67.0" && strings.HasPrefix(cell.Shape, "real:") {
					continue
				}
				for _, operation := range []string{"partial", "throwing"} {
					name := capture.SourceAPI + "/" + capture.Context + "/" + object + "/" + cell.Shape + "/" + operation
					t.Run(name, func(t *testing.T) {
						executeRecordTypeInsertProbe(t, capture, recordTypeInsertBody(object, cell, operation))
					})
				}
			}
		}
	}
}

func TestRecordTypeInsertExplicitModeNativeConformance(t *testing.T) {
	fixture := loadRecordTypeInsertFixture(t)
	if len(fixture.ModeCaptures) != 2 {
		t.Fatal("native access-mode capture scope changed")
	}
	for _, capture := range fixture.ModeCaptures {
		var cells []struct {
			Mode, Shape       string
			Success           bool
			Caller, Persisted recordTypeInsertSnapshot
			Errors            []recordTypeInsertError
		}
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &cells); err != nil {
			t.Fatal(err)
		}
		for _, native := range cells {
			// USER_MODE's unavailable-real denial has a separate permission model.
			if native.Mode == "USER_MODE" && strings.HasPrefix(native.Shape, "real:") {
				continue
			}
			t.Run(capture.SourceAPI+"/"+native.Mode+"/"+native.Shape, func(t *testing.T) {
				cell := recordTypeInsertCell{Shape: native.Shape, CallerBefore: recordTypeInsertSnapshot{RecordType: "null"}, CallerAfter: native.Caller, Persisted: native.Persisted}
				if strings.HasPrefix(native.Shape, "real:") {
					cell.CallerBefore.RecordType = strings.TrimPrefix(native.Shape, "real:")
				}
				cell.Partial.Success = native.Success
				cell.Partial.IDPresent = native.Caller.IDPresent
				cell.Partial.Errors = native.Errors
				body := recordTypeInsertBody("RTOne__c", cell, "partial")
				body = strings.Replace(body, "Database.insert(row,false)", "Database.insert(row,false,AccessLevel."+native.Mode+")", 1)
				executeRecordTypeInsertProbe(t, capture, body)
			})
		}
	}
}

func recordTypeInsertBody(object string, cell recordTypeInsertCell, operation string) string {
	literal := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "\\'") + "'" }
	materializeMessage := func(message string) string { return strings.ReplaceAll(message, "<RT:Master>", "012000000000000AAA") }
	body := "SObject row=" + object + ".SObjectType.newSObject(); row.put('Name','Independent record');"
	switch {
	case cell.Shape == "null":
		body += "row.put('RecordTypeId',null);"
	case cell.Shape == "Master":
		body += "row.put('RecordTypeId'," + object + ".SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Master').getRecordTypeId());"
	case strings.HasPrefix(cell.Shape, "real:"):
		body += "row.put('RecordTypeId'," + object + ".SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get(" + literal(strings.TrimPrefix(cell.Shape, "real:")) + ").getRecordTypeId());"
	}
	body += "System.assertEquals(null,row.Id);"
	snapshot := func(variable string, expected recordTypeInsertSnapshot) string {
		assertions := fmt.Sprintf("System.assertEquals(%t,%s.Id!=null);", expected.IDPresent, variable)
		if object != "RTZero__c" {
			value := "null"
			if expected.RecordType != "null" {
				value = literal(recordTypeInsertID(object, expected.RecordType))
			}
			assertions += "System.assertEquals(" + value + "," + variable + ".get('RecordTypeId'));"
		}
		return assertions
	}
	body += snapshot("row", cell.CallerBefore)
	if operation == "partial" {
		body += "Database.SaveResult result=Database.insert(row,false);"
		body += fmt.Sprintf("System.assertEquals(%t,result.isSuccess()); System.assertEquals(%t,result.getId()!=null); System.assertEquals(%d,result.getErrors().size());", cell.Partial.Success, cell.Partial.IDPresent, len(cell.Partial.Errors))
		for index, problem := range cell.Partial.Errors {
			prefix := fmt.Sprintf("result.getErrors()[%d]", index)
			body += "System.assertEquals(" + literal(problem.Status) + ",String.valueOf(" + prefix + ".getStatusCode()));"
			body += "System.assertEquals(" + literal(materializeMessage(problem.Message)) + "," + prefix + ".getMessage());"
			body += fmt.Sprintf("System.assertEquals(%d,%s.getFields().size());", len(problem.Fields), prefix)
			for n, field := range problem.Fields {
				body += fmt.Sprintf("System.assertEquals(%s,%s.getFields()[%d]);", literal(field), prefix, n)
			}
		}
		body += snapshot("row", cell.CallerAfter)
		if cell.Partial.Success {
			fields := "Id"
			if object != "RTZero__c" {
				fields += ",RecordTypeId"
			}
			body += "Id insertedId=row.Id; SObject stored=[SELECT " + fields + " FROM " + object + " WHERE Id=:insertedId];"
			body += snapshot("stored", cell.Persisted)
		} else {
			body += "System.assertEquals(0,[SELECT COUNT() FROM " + object + "]);"
		}
	} else if cell.Throwing.Success {
		body += "insert row;" + snapshot("row", cell.Throwing.Caller)
	} else {
		body += "Boolean failed=false; try { insert row; } catch(DmlException problem) { failed=true;"
		body += "System.assertEquals(" + literal(cell.Throwing.Type) + ",problem.getTypeName());"
		body += "System.assertEquals(" + literal(materializeMessage(cell.Throwing.Message)) + ",problem.getMessage());"
		body += "System.assertEquals(" + literal(cell.Throwing.FirstStatus) + ",String.valueOf(problem.getDmlStatusCode(0)));"
		body += "} System.assert(failed); System.assertEquals(null,row.Id); System.assertEquals(0,[SELECT COUNT() FROM " + object + "]);"
	}
	// Master remains exposed regardless of whether inserting that ID succeeds.
	body += "System.assertNotEquals(null," + object + ".SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Master'));"
	return body
}

func recordTypeInsertID(object, developerName string) string {
	if developerName == "Master" {
		return "012000000000000AAA"
	}
	offset := 100
	if object == "RTThree__c" {
		offset = 200
	}
	for index, name := range []string{"Alpha", "Beta", "Gamma"} {
		if name == developerName {
			return fmt.Sprintf("012000000000%03d", offset+index+1)
		}
	}
	panic("unobserved synthetic record type")
}

func executeRecordTypeInsertProbe(t *testing.T, capture recordTypeInsertCapture, body string) {
	t.Helper()
	org := testDataOrg()
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{
			"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
		}}, Records: map[storage.ID]storage.Record{},
	}
	for index, object := range []string{"RTZero__c", "RTOne__c", "RTThree__c"} {
		fields := map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}
		count := []int{0, 1, 3}[index]
		var recordTypes []storage.RecordTypeInfo
		if count > 0 {
			fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}}
		}
		for n := 0; n < count; n++ {
			name := []string{"Alpha", "Beta", "Gamma"}[n]
			id := storage.ID(recordTypeInsertID(object, name))
			recordTypes = append(recordTypes, storage.RecordTypeInfo{ID: id, DeveloperName: name, Name: name, Active: true, Available: false, Default: false})
			recordTypeObject := org.Objects["RecordType"]
			recordTypeObject.Records[id] = storage.Record{Object: "RecordType", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
			org.Objects["RecordType"] = recordTypeObject
		}
		org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, KeyPrefix: fmt.Sprintf("a%02d", index), Fields: fields, RecordTypes: recordTypes}, Records: map[storage.ID]storage.Record{}}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if capture.Context == "istest" {
		machine.EnableTestContext()
	}
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: capture.SourceAPI})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "RecordTypeInsertProbe", APIVersion: capture.SourceAPI}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "RecordTypeInsertProbe.observe", ClassName: "RecordTypeInsertProbe", IsStatic: true, Access: "public", ReturnType: "void", APIVersion: capture.SourceAPI, SourceContextBound: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	invoke, err := CompileAnonymousWithOptions("RecordTypeInsertProbe.observe();", CompileOptions{APIVersion: capture.SourceAPI})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(invoke); err != nil {
		t.Fatal(err)
	}
}
