package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestGenericRecordListStoreNativeFixtureIntegrity(t *testing.T) {
	var fixture struct {
		HelperSource, HelperSourceSha256, TypedGuardSource, TypedGuardSourceSha256 string
		Captures                                                                   []struct {
			Topic, Context, SourceAPI, TransportAPI, DeploymentID, DeploymentStatus                               string
			HelperSourceSha256, ObserverSource, ObserverSourceSha256, RawResponseSha256, AnswerJSON, AnswerSha256 string
		}
	}
	raw, err := os.ReadFile("testdata/generic_record_list_store_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	digest := func(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }
	if digest(fixture.HelperSource) != fixture.HelperSourceSha256 || digest(fixture.TypedGuardSource) != fixture.TypedGuardSourceSha256 {
		t.Fatal("native helper source hash changed")
	}
	if len(fixture.Captures) != 4 {
		t.Fatalf("native captures=%d, want4", len(fixture.Captures))
	}
	seen := make(map[string]bool)
	for _, capture := range fixture.Captures {
		key := capture.Topic + "/" + capture.Context
		if seen[key] || capture.SourceAPI != "61.0" || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" || len(capture.RawResponseSha256) != 64 {
			t.Fatalf("incomplete capture %s", key)
		}
		seen[key] = true
		if digest(capture.ObserverSource) != capture.ObserverSourceSha256 || digest(capture.AnswerJSON) != capture.AnswerSha256 {
			t.Fatalf("native observer or answer hash changed: %s", key)
		}
		expectedHelper := fixture.HelperSourceSha256
		if capture.Topic == "record-list-typed-guard61" {
			expectedHelper = fixture.TypedGuardSourceSha256
		}
		if capture.HelperSourceSha256 != expectedHelper {
			t.Fatalf("wrong native helper: %s", key)
		}
		expectedTransport := "67.0"
		if capture.Context == "anonymous" {
			expectedTransport = "61.0"
		} else if capture.Context != "istest" {
			t.Fatalf("unknown native context: %s", key)
		}
		if capture.TransportAPI != expectedTransport {
			t.Fatalf("wrong transport: %s", key)
		}
	}
}

// Source61 native named/anonymous observations cover generic SObject backing,
// including Contact values written through an Account declared collection view.
// Concrete Account backing is a separate guarded boundary.
func TestGenericRecordListStoreNativeConformance(t *testing.T) {
	var fixture struct {
		Rows []struct {
			APIVersion, Context, Shape string
			Expected                   struct {
				Phase, SourceType                             string
				Matching                                      bool
				SourceSize, AssignedSize, CastSize, FinalSize int
			}
		}
	}
	raw, err := os.ReadFile("testdata/generic_record_list_store_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Rows) != 4 {
		t.Fatalf("native rows=%d, want 4", len(fixture.Rows))
	}
	for _, row := range fixture.Rows {
		t.Run(row.Context+"/"+row.Shape, func(t *testing.T) {
			if row.APIVersion != "61.0" || row.Expected.Phase != "complete" {
				t.Fatal("incomplete native observation")
			}
			element := row.Expected.SourceType
			var value string
			switch element {
			case "Account":
				value = "new Account(Name='Synthetic account')"
			case "Contact":
				value = "new Contact(LastName='Synthetic contact')"
			default:
				t.Fatalf("unobserved source element %q", element)
			}
			source := fmt.Sprintf(`
List<SObject> source=RecordListStoreProbe.rows();
System.assertEquals(%d,source.size());
System.assertEquals(%s.SObjectType,source[0].getSObjectType());
List<Account> rows=source;
System.assertEquals(%d,rows.size());
List<Account> additional=(List<Account>)RecordListStoreProbe.rows();
System.assertEquals(%d,additional.size());
rows.addAll(additional);
System.assertEquals(%d,rows.size());
`, row.Expected.SourceSize, element, row.Expected.AssignedSize, row.Expected.CastSize, row.Expected.FinalSize)
			helper, err := CompileAnonymousWithOptions("return new List<SObject>{"+value+"};", CompileOptions{APIVersion: row.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{Name: "RecordListStoreProbe", APIVersion: row.APIVersion}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "RecordListStoreProbe.rows", ClassName: "RecordListStoreProbe", ReturnType: "List<SObject>", IsStatic: true, Access: "public", APIVersion: row.APIVersion, SourceContextBound: true, Program: helper}); err != nil {
				t.Fatal(err)
			}
			if row.Context == "istest" {
				if _, err := machine.ExecuteTestInClass(program, "RecordListStoreProbe"); err != nil {
					t.Fatal(err)
				}
			} else if row.Context == "anonymous" {
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Fatalf("unobserved context %q", row.Context)
			}
		})
	}
}

func TestGenericRecordListStoreTypedGuardNativeConformance(t *testing.T) {
	var fixture struct {
		TypedGuardRows []struct {
			APIVersion, Context, Operation, MessageDisposition string
			Expected                                           struct {
				Phase, ExceptionType, ExceptionMessage string
				BeforeSize, AfterSize                  int
			}
		}
	}
	raw, err := os.ReadFile("testdata/generic_record_list_store_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.TypedGuardRows) != 4 {
		t.Fatalf("native guard rows=%d, want 4", len(fixture.TypedGuardRows))
	}
	for _, row := range fixture.TypedGuardRows {
		t.Run(row.Context+"/"+row.Operation, func(t *testing.T) {
			if row.APIVersion != "61.0" || row.Expected.Phase != row.Operation {
				t.Fatal("incomplete native guard")
			}
			mutation := "view.add(new Contact(LastName='Synthetic contact'));"
			if row.Operation == "addAll" {
				mutation = "view.addAll(new List<SObject>{new Contact(LastName='Synthetic contact')});"
			} else if row.Operation != "add" {
				t.Fatal("unobserved mutation")
			}
			source := fmt.Sprintf(`
List<Account> backing=new List<Account>{new Account(Name='Synthetic account')};
List<SObject> view=backing;
System.assertEquals(%d,view.size());
String observedType=null;
String observedMessage=null;
try { %s } catch(Exception issue) {
 observedType=issue.getTypeName();
 observedMessage=issue.getMessage();
}
System.assertEquals('%s',observedType);
System.assertEquals(%d,view.size());
`, row.Expected.BeforeSize, mutation, row.Expected.ExceptionType, row.Expected.AfterSize)
			if row.MessageDisposition == "asserted" {
				source += "System.assertEquals('" + row.Expected.ExceptionMessage + "',observedMessage);"
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			machine.SetOrg(&org)
			if row.Context == "istest" {
				if _, err := machine.ExecuteTestInClass(program, "RecordListTypedGuardProbe"); err != nil {
					t.Fatal(err)
				}
			} else if row.Context == "anonymous" {
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Fatal("unobserved context")
			}
		})
	}
}
