package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native trigger boundaries: testdata/conformance/case349_explicit_id/org.tsv:2-19.
func TestCase349ExplicitIDUpsertPreflight(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
	account.Records["001000000000101"] = storage.Record{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001")}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, key := range []string{"", "Id", "ID"} {
		t.Run("first_"+key, func(t *testing.T) {
			rows := []storage.Record{{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), "FirstName": storage.StringValue("Synthetic")}}}
			got := machine.preflightBusinessAccountNameFailures(rows, "upsert", key)
			if len(got) != 1 || got[0].Success || got[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
				t.Fatalf("key=%q first-name preflight: %#v", key, got)
			}
		})
		t.Run("email_"+key, func(t *testing.T) {
			rows := []storage.Record{{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"PersonEmail": storage.StringValue("bad")}}}
			got := machine.preflightAccountPersonEmailFailures(rows, "upsert", key)
			if len(got) != 1 || got[0].Success || got[0].StatusCode != "INVALID_EMAIL_ADDRESS" {
				t.Fatalf("key=%q email preflight: %#v", key, got)
			}
		})
	}
}

// TestCase349ExplicitIDUpsertNative executes captured explicit-key outcomes at
// each API, including rejection twins and the blank boundary.
func TestCase349ExplicitIDUpsertNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, assignment, status string
			success                  bool
		}{
			{"first", "a.FirstName='Synthetic';", "INVALID_FIELD_FOR_INSERT_UPDATE", false},
			{"bademail", "a.PersonEmail='bad';", "INVALID_EMAIL_ADDRESS", false},
			{"validemail", "a.PersonEmail='synthetic@example.invalid';", "INVALID_FIELD_FOR_INSERT_UPDATE", false},
			{"blankemail", "a.PersonEmail='   ';", "SUCCESS", true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := fmt.Sprintf("Account seed = new Account(Name='Business', RecordTypeId='012000000000001'); insert seed; Account a = new Account(Id=seed.Id); %s Database.UpsertResult r = Database.upsert(a, Account.Fields.Id, false); System.assertEquals(%t,r.isSuccess());", tc.assignment, tc.success)
				if !tc.success {
					source += fmt.Sprintf("System.assertEquals('%s',String.valueOf(r.getErrors()[0].getStatusCode()));", tc.status)
				}
				count := 1
				if tc.name == "first" || tc.name == "bademail" {
					count = 0
				}
				source += fmt.Sprintf("System.assertEquals(%d, ReviewRouteCounter.hits);", count)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				org.APIVersion = api
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				account := org.Objects["Account"]
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000002", DeveloperName: "PersonAccount", Active: true, Available: true, PersonAccount: true}}
				org.Objects["Account"] = account
				org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}}, Records: map[storage.ID]storage.Record{"012000000000001": {Object: "RecordType", ID: "012000000000001"}, "012000000000002": {Object: "RecordType", ID: "012000000000002"}}}
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "ReviewRouteCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
					t.Fatal(err)
				}
				trigger, err := CompileAnonymousWithOptions("ReviewRouteCounter.hits++; System.assertEquals(false, Trigger.new[0].IsPersonAccount);", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterTrigger(Trigger{Name: "ReviewRouteObserver", Object: "Account", Timing: "before", Operation: "update", APIVersion: api, Program: trigger}); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
