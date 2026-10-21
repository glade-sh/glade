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
)

type readOnlyApplicationNativeFixture struct {
	Captures []struct {
		Context, SourceAPI, TransportAPI, Source, Metadata string
		SourceSha256, MetadataSha256                       string
		OrgIDHash                                          string `json:"orgIdSHA256"`
		DeploymentID, RawResponseSha256                    string
		AnswerJSON, AnswerSha256                           string
	}
}

type readOnlyApplicationNativeResult struct {
	Success, IDPresent bool
	Errors             []struct {
		Status, Message string
		Fields          []string
	}
}

type readOnlyApplicationNativeException struct {
	Type                               string
	IsDmlException, MaintenanceMessage bool
}

type readOnlyApplicationNativeObservation struct {
	InitialMode, ReadOnlyMode, ResetMode                 string
	DeleteResults                                        []readOnlyApplicationNativeResult
	Insert, Update, Upsert, Undelete                     readOnlyApplicationNativeResult
	Statement, AllOrNone                                 readOnlyApplicationNativeException
	RemainingReadOnly, RemainingWritable, NamesUnchanged int
	WritableSuccesses                                    []bool
}

func loadReadOnlyApplicationNativeFixture(t *testing.T) readOnlyApplicationNativeFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/read_only_application_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture readOnlyApplicationNativeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestReadOnlyApplicationNativeFixtureIntegrity(t *testing.T) {
	fixture := loadReadOnlyApplicationNativeFixture(t)
	if len(fixture.Captures) != 6 {
		t.Fatal("native capture scope changed")
	}
	digest := func(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }
	validHash := func(value string) bool {
		decoded, err := hex.DecodeString(value)
		return err == nil && len(decoded) == sha256.Size
	}
	seen := make(map[string]bool)
	for _, capture := range fixture.Captures {
		key := capture.SourceAPI + "/" + capture.Context
		if seen[key] {
			t.Fatalf("duplicate native context: %s", key)
		}
		seen[key] = true
		if capture.SourceAPI != "61.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0" {
			t.Fatalf("unobserved source API: %s", key)
		}
		if !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSha256) || capture.Source == "" {
			t.Fatalf("incomplete provenance: %s", key)
		}
		for _, pair := range [][2]string{
			{capture.Source, capture.SourceSha256},
			{capture.Metadata, capture.MetadataSha256},
			{capture.AnswerJSON, capture.AnswerSha256},
		} {
			if digest(pair[0]) != pair[1] {
				t.Fatalf("source or answer digest mismatch: %s", key)
			}
		}
		switch capture.Context {
		case "istest":
			if capture.TransportAPI != "67.0" || capture.DeploymentID == "" {
				t.Fatalf("named transport/deployment mismatch: %s", key)
			}
			var metadata struct {
				APIVersion string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(capture.Metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.APIVersion != capture.SourceAPI {
				t.Fatalf("source metadata API mismatch: %s", key)
			}
			var observed readOnlyApplicationNativeObservation
			if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
				t.Fatal(err)
			}
			if len(observed.DeleteResults) != 2 || len(observed.WritableSuccesses) != 2 || observed.Statement.Type == "" || observed.AllOrNone.Type == "" {
				t.Fatalf("incomplete observed shapes: %s", key)
			}
		case "anonymous":
			if capture.TransportAPI != capture.SourceAPI || capture.Metadata != "" {
				t.Fatalf("anonymous source/transport mismatch: %s", key)
			}
			var observed struct{ ExceptionMessage string }
			if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
				t.Fatal(err)
			}
			if observed.ExceptionMessage == "" {
				t.Fatalf("missing anonymous boundary answer: %s", key)
			}
		default:
			t.Fatalf("unobserved context: %s", key)
		}
	}
}

func TestReadOnlyApplicationNativeConformance(t *testing.T) {
	fixture := loadReadOnlyApplicationNativeFixture(t)
	for _, capture := range fixture.Captures {
		t.Run(capture.SourceAPI+"/"+capture.Context, func(t *testing.T) {
			switch capture.Context {
			case "istest":
				var observed readOnlyApplicationNativeObservation
				if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
					t.Fatal(err)
				}
				executeReadOnlyApplicationProbe(t, capture.SourceAPI, readOnlyApplicationNativeAssertions(observed))
			case "anonymous":
				var observed struct{ ExceptionMessage string }
				if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(capture.Source, CompileOptions{APIVersion: capture.SourceAPI})
				if err != nil {
					t.Fatal(err)
				}
				_, err = New(nil).Execute(program)
				if err == nil || err.Error() != observed.ExceptionMessage {
					t.Fatalf("anonymous boundary = %v, want %q", err, observed.ExceptionMessage)
				}
			default:
				t.Fatal("unobserved native context")
			}
		})
	}
}

func readOnlyApplicationNativeAssertions(observed readOnlyApplicationNativeObservation) string {
	quote := func(value string) string {
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "'", "\\'")
		value = strings.ReplaceAll(value, "\r", "\\r")
		value = strings.ReplaceAll(value, "\n", "\\n")
		return "'" + value + "'"
	}
	var source strings.Builder
	fmt.Fprint(&source, `
List<Account> rows=new List<Account>{new Account(Name='First'),new Account(Name='Second')};
insert rows;
Set<Id> ids=new Map<Id,Account>(rows).keySet();
Account trashed=new Account(Name='Trashed');insert trashed;delete trashed;
`)
	fmt.Fprintf(&source, "System.assertEquals(%s,String.valueOf(System.getApplicationReadWriteMode()));\n", quote(observed.InitialMode))
	fmt.Fprint(&source, "Test.startTest();Test.setReadOnlyApplicationMode(true);\n")
	fmt.Fprintf(&source, "System.assertEquals(%s,String.valueOf(System.getApplicationReadWriteMode()));\n", quote(observed.ReadOnlyMode))
	assertResult := func(expression string, result readOnlyApplicationNativeResult) {
		fmt.Fprintf(&source, "System.assertEquals(%t,%s.isSuccess());\n", result.Success, expression)
		fmt.Fprintf(&source, "System.assertEquals(%t,%s.getId()!=null);\n", result.IDPresent, expression)
		fmt.Fprintf(&source, "System.assertEquals(%d,%s.getErrors().size());\n", len(result.Errors), expression)
		for index, failure := range result.Errors {
			errorExpression := fmt.Sprintf("%s.getErrors()[%d]", expression, index)
			fmt.Fprintf(&source, "System.assertEquals(%s,String.valueOf(%s.getStatusCode()));\n", quote(failure.Status), errorExpression)
			fmt.Fprintf(&source, "System.assertEquals(%s,%s.getMessage());\n", quote(failure.Message), errorExpression)
			fmt.Fprintf(&source, "System.assertEquals(%d,%s.getFields().size());\n", len(failure.Fields), errorExpression)
			for fieldIndex, field := range failure.Fields {
				fmt.Fprintf(&source, "System.assertEquals(%s,%s.getFields()[%d]);\n", quote(field), errorExpression, fieldIndex)
			}
		}
	}
	fmt.Fprint(&source, "List<Database.DeleteResult> denied=Database.delete(rows,false);\n")
	fmt.Fprintf(&source, "System.assertEquals(%d,denied.size());\n", len(observed.DeleteResults))
	for index, result := range observed.DeleteResults {
		assertResult(fmt.Sprintf("denied[%d]", index), result)
	}
	fmt.Fprintf(&source, "System.assertEquals(%d,[SELECT COUNT() FROM Account WHERE Id IN :ids]);\n", observed.RemainingReadOnly)
	fmt.Fprint(&source, "Database.SaveResult inserted=Database.insert(new Account(Name='Blocked'),false);\n")
	assertResult("inserted", observed.Insert)
	fmt.Fprint(&source, "Database.SaveResult updated=Database.update(new Account(Id=rows[0].Id,Name='Changed'),false);\n")
	assertResult("updated", observed.Update)
	fmt.Fprint(&source, "Database.UpsertResult upserted=Database.upsert(new Account(Id=rows[1].Id,Name='Changed'),false);\n")
	assertResult("upserted", observed.Upsert)
	fmt.Fprint(&source, "Database.UndeleteResult restored=Database.undelete(trashed,false);\n")
	assertResult("restored", observed.Undelete)
	fmt.Fprintf(&source, "System.assertEquals(%d,[SELECT COUNT() FROM Account WHERE Id IN :ids AND Name!='Changed']);\n", observed.NamesUnchanged)
	assertException := func(name, statement string, exception readOnlyApplicationNativeException) {
		fmt.Fprintf(&source, "Boolean %sThrown=false;try{%s}catch(Exception e){%sThrown=true;\n", name, statement, name)
		fmt.Fprintf(&source, "System.assertEquals(%s,e.getTypeName());\n", quote(exception.Type))
		fmt.Fprintf(&source, "System.assertEquals(%t,e instanceof DmlException);\n", exception.IsDmlException)
		fmt.Fprintf(&source, "System.assertEquals(%t,e.getMessage().contains('INSERT_UPDATE_DELETE_NOT_ALLOWED_DURING_MAINTENANCE'));}\n", exception.MaintenanceMessage)
		fmt.Fprintf(&source, "System.assertEquals(true,%sThrown);\n", name)
	}
	assertException("statement", "delete rows;", observed.Statement)
	assertException("allOrNone", "Database.delete(rows,true);", observed.AllOrNone)
	fmt.Fprint(&source, "Test.setReadOnlyApplicationMode(false);\n")
	fmt.Fprintf(&source, "System.assertEquals(%s,String.valueOf(System.getApplicationReadWriteMode()));\n", quote(observed.ResetMode))
	fmt.Fprint(&source, "List<Database.DeleteResult> writable=Database.delete(rows,false);\n")
	fmt.Fprintf(&source, "System.assertEquals(%d,writable.size());\n", len(observed.WritableSuccesses))
	for index, success := range observed.WritableSuccesses {
		fmt.Fprintf(&source, "System.assertEquals(%t,writable[%d].isSuccess());\n", success, index)
	}
	fmt.Fprintf(&source, "System.assertEquals(%d,[SELECT COUNT() FROM Account WHERE Id IN :ids]);\nTest.stopTest();", observed.RemainingWritable)
	return source.String()
}
