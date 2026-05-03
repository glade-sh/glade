package dml

import (
	"errors"
	"testing"
	"time"

	"github.com/open-aer/oaer/internal/storage"
)

func TestInsertUpdateDelete(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	engine.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}})
	if !insert[0].Success || insert[0].ID != "001000000000001" {
		t.Fatalf("insert = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if stored.System.CreatedDate != "2026-05-02T12:00:00Z" || stored.System.SystemModstamp == "" || stored.System.OwnerID == "" {
		t.Fatalf("system fields after insert = %#v", stored.System)
	}

	engine.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 5, 0, 0, time.UTC) }
	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Changed"),
		},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Changed" {
		t.Fatalf("updated name = %q", got)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].System.LastModifiedDate; got != "2026-05-02T12:05:00Z" {
		t.Fatalf("last modified after update = %q", got)
	}

	engine.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 10, 0, 0, time.UTC) }
	deleteResult := engine.Delete([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if !deleteResult[0].Success {
		t.Fatalf("delete = %#v", deleteResult)
	}
	if !org.Objects["Account"].Records[insert[0].ID].System.IsDeleted {
		t.Fatalf("record was not soft deleted: %#v", org.Objects["Account"].Records[insert[0].ID])
	}
	if got := org.Objects["Account"].Records[insert[0].ID].System.SystemModstamp; got != "2026-05-02T12:10:00Z" {
		t.Fatalf("system modstamp after delete = %q", got)
	}
}

func TestInsertValidatesRequiredAndUnknownFields(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)

	missing := engine.Insert([]storage.Record{{Object: "Account"}})
	if missing[0].Success || missing[0].Error == "" {
		t.Fatalf("missing required result = %#v", missing)
	}
	unknown := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Bogus__c": storage.StringValue("bad"),
		},
	}})
	if unknown[0].Success || unknown[0].Error == "" {
		t.Fatalf("unknown field result = %#v", unknown)
	}
}

func TestWithTransactionRollsBackOnError(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	err := engine.WithTransaction(func(tx *Engine) error {
		result := tx.Insert([]storage.Record{{
			Object: "Account",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue("Acme"),
			},
		}})
		if !result[0].Success {
			t.Fatalf("insert = %#v", result)
		}
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected transaction error")
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("transaction did not roll back: %#v", org.Objects["Account"].Records)
	}
}

func TestUpsertByExternalIDAndUniqueValidation(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Code__c"] = storage.Field{APIName: "Code__c", Type: storage.FieldString, Unique: true}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Upsert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue("Acme"),
			"External_Key__c": storage.StringValue("ext-1"),
			"Code__c":         storage.StringValue("A"),
		},
	}})
	if !insert[0].Success || !insert[0].Created {
		t.Fatalf("external insert = %#v", insert)
	}
	update := engine.Upsert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"External_Key__c": storage.StringValue("EXT-1"),
			"Name":            storage.StringValue("Changed"),
		},
	}})
	if !update[0].Success || update[0].Created || update[0].ID != insert[0].ID {
		t.Fatalf("external update = %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Changed" {
		t.Fatalf("updated name = %q", got)
	}

	duplicate := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Other"),
			"Code__c": storage.StringValue("a"),
		},
	}})
	if duplicate[0].Success || duplicate[0].StatusCode != "DUPLICATE_VALUE" {
		t.Fatalf("duplicate = %#v", duplicate)
	}
}

func TestUpsertWithExplicitExternalID(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Other_Key__c"] = storage.Field{APIName: "Other_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue("Acme"),
			"External_Key__c": storage.StringValue("ext-1"),
			"Other_Key__c":    storage.StringValue("other-1"),
		},
	}}, "Other_Key__c")
	if !insert[0].Success || !insert[0].Created {
		t.Fatalf("explicit insert = %#v", insert)
	}
	update := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Changed"),
			"Other_Key__c": storage.StringValue("OTHER-1"),
		},
	}}, "Other_Key__c")
	if !update[0].Success || update[0].Created || update[0].ID != insert[0].ID {
		t.Fatalf("explicit update = %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Changed" {
		t.Fatalf("updated name = %q", got)
	}
	missing := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Missing")},
	}}, "Other_Key__c")
	if missing[0].Success || missing[0].StatusCode != "MISSING_ARGUMENT" {
		t.Fatalf("missing external id = %#v", missing)
	}
}

func TestReferenceValidationRestrictedDeleteAndUndelete(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:            "AccountId",
				ParentObjects:    []string{"Account"},
				RestrictedDelete: true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account = %#v", account)
	}
	missingParent := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Smith"),
			"AccountId": storage.IDValue("001999999999999"),
		},
	}})
	if missingParent[0].Success || missingParent[0].StatusCode != "FIELD_INTEGRITY_EXCEPTION" {
		t.Fatalf("missing parent = %#v", missingParent)
	}
	contact := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Smith"),
			"AccountId": storage.IDValue(account[0].ID),
		},
	}})
	if !contact[0].Success {
		t.Fatalf("contact = %#v", contact)
	}
	blockedDelete := engine.Delete([]storage.Record{{Object: "Account", ID: account[0].ID}})
	if blockedDelete[0].Success || blockedDelete[0].StatusCode != "DELETE_FAILED" {
		t.Fatalf("blocked delete = %#v", blockedDelete)
	}
	deleteContact := engine.Delete([]storage.Record{{Object: "Contact", ID: contact[0].ID}})
	if !deleteContact[0].Success {
		t.Fatalf("delete contact = %#v", deleteContact)
	}
	undeleteContact := engine.Undelete([]storage.Record{{Object: "Contact", ID: contact[0].ID}})
	if !undeleteContact[0].Success || org.Objects["Contact"].Records[contact[0].ID].System.IsDeleted {
		t.Fatalf("undelete contact = %#v", undeleteContact)
	}
}

func TestMergeSoftDeletesDuplicateAndReparentsChildren(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:         "AccountId",
				ParentObjects: []string{"Account"},
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	master := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Master")},
	}})
	duplicate := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Duplicate")},
	}})
	child := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Smith"),
			"AccountId": storage.IDValue(duplicate[0].ID),
		},
	}})
	if !master[0].Success || !duplicate[0].Success || !child[0].Success {
		t.Fatalf("setup = %#v %#v %#v", master, duplicate, child)
	}

	merge := engine.Merge(storage.Record{Object: "Account", ID: master[0].ID}, []storage.Record{{Object: "Account", ID: duplicate[0].ID}})
	if len(merge) != 1 || !merge[0].Success || merge[0].ID != master[0].ID {
		t.Fatalf("merge = %#v", merge)
	}
	if !org.Objects["Account"].Records[duplicate[0].ID].System.IsDeleted {
		t.Fatalf("duplicate was not soft deleted")
	}
	if got := org.Objects["Contact"].Records[child[0].ID].Fields["AccountId"].ID; got != master[0].ID {
		t.Fatalf("child account id = %s", got)
	}
}

func TestDeleteCascadesThroughRelationshipMetadata(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:         "AccountId",
				ParentObjects: []string{"Account"},
				CascadeDelete: true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account = %#v", account)
	}
	contact := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Smith"),
			"AccountId": storage.IDValue(account[0].ID),
		},
	}})
	if !contact[0].Success {
		t.Fatalf("contact = %#v", contact)
	}
	deleteAccount := engine.Delete([]storage.Record{{Object: "Account", ID: account[0].ID}})
	if !deleteAccount[0].Success {
		t.Fatalf("delete account = %#v", deleteAccount)
	}
	if !org.Objects["Account"].Records[account[0].ID].System.IsDeleted {
		t.Fatalf("account was not deleted")
	}
	if !org.Objects["Contact"].Records[contact[0].ID].System.IsDeleted {
		t.Fatalf("contact was not cascade deleted")
	}
}

func testOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}
