package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// The native controls use existing IDs: case349_explicit_id/org.tsv:2-9.
// Leave unobserved identity and insertion cases on the previous DML path.
func TestCase349IdUpsertPreservesUncapturedIdentity(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}}
	deleted := storage.Record{Object: "Account", ID: "001000000000102"}
	deleted.System.IsDeleted = true
	account.Records[deleted.ID] = deleted
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, key := range []string{"", "Id", "ID"} {
		for _, id := range []storage.ID{"", "001000000000199", "001000000000102", "003000000000101"} {
			record := storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), "FirstName": storage.StringValue("Synthetic"), "PersonEmail": storage.StringValue("bad")}}
			for name, got := range map[string][]dml.Result{"name": machine.preflightBusinessAccountNameFailures([]storage.Record{record}, "upsert", key), "email": machine.preflightAccountPersonEmailFailures([]storage.Record{record}, "upsert", key)} {
				if len(got) > 0 && !got[0].Success {
					t.Errorf("%s %s %s: %#v", key, id, name, got)
				}
			}
		}
	}
}

// Mixed all-or-none Id upserts preserve identity precedence for the entire batch.
func TestCase349IdUpsertAllOrNoneIdentityPrecedence(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Records["001000000000101"] = storage.Record{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"Name": storage.StringValue("Business")}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	source := `Boolean caught=false; try { Database.upsert(new List<Account>{new Account(Id='001000000000199'),new Account(Id='001000000000101',PersonEmail='invalid')},Account.Fields.Id,true); } catch(DmlException e) { caught=true; System.assertEquals('INVALID_CROSS_REFERENCE_KEY',String.valueOf(e.getDmlStatusCode(0))); } System.assertEquals(true,caught);`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
