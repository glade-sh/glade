package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCB117DuplicateAndActiveUndeleteContracts(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External__c"] = storage.Field{APIName: "External__c", Type: storage.FieldString, Unique: true}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	first := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("First"),
			"External__c": storage.StringValue("external-1"),
		},
	}})
	if !first[0].Success {
		t.Fatalf("first insert = %#v", first[0])
	}
	duplicate := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Duplicate"),
			"External__c": storage.StringValue("EXTERNAL-1"),
		},
	}})
	if duplicate[0].Success || duplicate[0].StatusCode != "DUPLICATE_VALUE" {
		t.Fatalf("duplicate = %#v", duplicate[0])
	}
	if len(duplicate[0].Fields) != 0 || len(duplicate[0].Errors) != 1 || len(duplicate[0].Errors[0].Fields) != 0 {
		t.Fatalf("duplicate fields = %#v", duplicate[0])
	}
	wantMessage := "duplicate value found: External__c duplicates value on record with id: " + string(first[0].ID)
	if duplicate[0].Error != wantMessage || duplicate[0].Errors[0].Message != wantMessage {
		t.Fatalf("duplicate message = %#v, want %q", duplicate[0], wantMessage)
	}

	active := engine.Undelete([]storage.Record{{ID: first[0].ID, Object: "Account"}})
	if active[0].Success || active[0].StatusCode != "UNDELETE_FAILED" || len(active[0].Fields) != 0 {
		t.Fatalf("active undelete = %#v", active[0])
	}
	if len(active[0].Errors) != 1 || active[0].Errors[0].StatusCode != "UNDELETE_FAILED" || active[0].Errors[0].Message != "Entity is not in the recycle bin" || !strings.Contains(active[0].Error, "recycle bin") {
		t.Fatalf("active undelete errors = %#v", active[0].Errors)
	}
}
