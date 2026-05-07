package dml

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/open-aer/oaer/internal/soql"
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
	if missing[0].StatusCode != "REQUIRED_FIELD_MISSING" || len(missing[0].Fields) != 1 || missing[0].Fields[0] != "Name" {
		t.Fatalf("missing required detail = %#v", missing[0])
	}
	if len(missing[0].Errors) != 1 || missing[0].Errors[0].StatusCode != "REQUIRED_FIELD_MISSING" || missing[0].Errors[0].Fields[0] != "Name" {
		t.Fatalf("missing required errors = %#v", missing[0].Errors)
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

func TestInsertPersonAccountCreatesSyntheticPersonContact(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"FirstName":                storage.StringValue("Ada"),
			"LastName":                 storage.StringValue("Lovelace"),
			"PersonEmail":              storage.StringValue("ada@example.invalid"),
			"PersonTitle":              storage.StringValue("Countess"),
			"PersonMailingStateCode":   storage.StringValue("CA"),
			"PersonMailingCountryCode": storage.StringValue("US"),
			"PersonOtherStreet":        storage.StringValue("1 Other Way"),
			"PersonBirthdate":          storage.DateValue("1815-12-10"),
			"PersonDoNotCall":          storage.BooleanValue(true),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	account := org.Objects["Account"].Records[insert[0].ID]
	if got := account.Fields["Name"].String; got != "Ada Lovelace" {
		t.Fatalf("person account name = %q", got)
	}
	contactID := account.Fields["PersonContactId"].ID
	if contactID == "" {
		t.Fatalf("PersonContactId was not populated: %#v", account.Fields)
	}
	contact := org.Objects["Contact"].Records[contactID]
	if got := contact.Fields["AccountId"].ID; got != insert[0].ID {
		t.Fatalf("person contact AccountId = %q", got)
	}
	if got := contact.Fields["Email"].String; got != "ada@example.invalid" {
		t.Fatalf("person contact email = %q", got)
	}
	if got := contact.Fields["Title"].String; got != "Countess" {
		t.Fatalf("person contact title = %q", got)
	}
	if got := contact.Fields["MailingCountryCode"].String; got != "US" {
		t.Fatalf("person contact mailing country code = %q", got)
	}
	if got := contact.Fields["OtherStreet"].String; got != "1 Other Way" {
		t.Fatalf("person contact other street = %q", got)
	}
	if got := contact.Fields["Birthdate"].String; got != "1815-12-10" {
		t.Fatalf("person contact birthdate = %q", got)
	}
	if !contact.Fields["DoNotCall"].Boolean {
		t.Fatalf("person contact DoNotCall was not mirrored: %#v", contact.Fields["DoNotCall"])
	}
	update := engine.Update([]storage.Record{{
		Object: "Account",
		ID:     insert[0].ID,
		Fields: map[string]storage.Value{
			"PersonMobilePhone": storage.StringValue("555-0101"),
		},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update[0])
	}
	contact = org.Objects["Contact"].Records[contactID]
	if got := contact.Fields["MobilePhone"].String; got != "555-0101" {
		t.Fatalf("person contact mobile after update = %q", got)
	}
}

func TestDatabaseErrorDetailsForRequiredDuplicateAndValidationFailures(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Code__c"] = storage.Field{APIName: "Code__c", Type: storage.FieldString, Unique: true}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "BlockBadName",
		Active:                true,
		ErrorConditionFormula: `Name = "Blocked"`,
		ErrorMessage:          "blocked by validation rule",
		ErrorDisplayField:     "Name",
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	existing := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Acme"),
			"Code__c": storage.StringValue("A"),
		},
	}})
	if !existing[0].Success {
		t.Fatalf("existing insert = %#v", existing)
	}
	duplicateValue := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Other"),
			"Code__c": storage.StringValue("a"),
		},
	}})
	assertDMLErrorDetail(t, duplicateValue[0], "DUPLICATE_VALUE", "Code__c")

	duplicateID := engine.Insert([]storage.Record{{
		ID:     existing[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Same Id"),
		},
	}})
	assertDMLErrorDetail(t, duplicateID[0], "DUPLICATE_VALUE", "Id")

	blocked := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Blocked"),
		},
	}})
	assertDMLErrorDetail(t, blocked[0], "FIELD_CUSTOM_VALIDATION_EXCEPTION", "Name")
}

func assertDMLErrorDetail(t *testing.T, result Result, statusCode, field string) {
	t.Helper()
	if result.Success || result.StatusCode != statusCode {
		t.Fatalf("result = %#v, want status %s", result, statusCode)
	}
	if len(result.Fields) != 1 || result.Fields[0] != field {
		t.Fatalf("result fields = %#v, want %s", result.Fields, field)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %#v, want one", result.Errors)
	}
	err := result.Errors[0]
	if err.StatusCode != statusCode || len(err.Fields) != 1 || err.Fields[0] != field || err.Message == "" {
		t.Fatalf("error detail = %#v, want %s on %s", err, statusCode, field)
	}
}

func TestDMLRejectsCalculatedFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Score__c": storage.DecimalValue("42"),
		},
	}})
	if insert[0].Success || insert[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" || len(insert[0].Fields) != 1 || insert[0].Fields[0] != "Score__c" {
		t.Fatalf("calculated insert = %#v", insert)
	}

	created := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !created[0].Success {
		t.Fatalf("insert = %#v", created)
	}
	update := engine.Update([]storage.Record{{
		ID:            created[0].ID,
		Object:        "Account",
		ExplicitNulls: map[string]bool{"Score__c": true},
	}})
	if update[0].Success || update[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" || len(update[0].Errors) != 1 || update[0].Errors[0].Fields[0] != "Score__c" {
		t.Fatalf("calculated update = %#v", update)
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

func TestEmptyRecycleBinRemovesDeletedRecords(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	notDeleted := engine.EmptyRecycleBin([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if notDeleted[0].Success || notDeleted[0].StatusCode != "ENTITY_IS_NOT_IN_RECYCLE_BIN" {
		t.Fatalf("not deleted emptyRecycleBin = %#v", notDeleted)
	}
	if len(notDeleted[0].Errors) != 1 || notDeleted[0].Errors[0].StatusCode != "ENTITY_IS_NOT_IN_RECYCLE_BIN" {
		t.Fatalf("not deleted emptyRecycleBin errors = %#v", notDeleted[0].Errors)
	}
	deleted := engine.Delete([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if !deleted[0].Success {
		t.Fatalf("delete = %#v", deleted)
	}
	emptied := engine.EmptyRecycleBin([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if !emptied[0].Success || emptied[0].ID != insert[0].ID {
		t.Fatalf("emptyRecycleBin = %#v", emptied)
	}
	if _, ok := org.Objects["Account"].Records[insert[0].ID]; ok {
		t.Fatalf("record remained after emptyRecycleBin: %#v", org.Objects["Account"].Records[insert[0].ID])
	}
}

func TestUndeleteRejectsActiveRecords(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	active := engine.Undelete([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if active[0].Success || active[0].StatusCode != "ENTITY_IS_NOT_DELETED" {
		t.Fatalf("active undelete = %#v", active)
	}
	if len(active[0].Errors) != 1 || active[0].Errors[0].StatusCode != "ENTITY_IS_NOT_DELETED" {
		t.Fatalf("active undelete errors = %#v", active[0].Errors)
	}
	if org.Objects["Account"].Records[insert[0].ID].System.IsDeleted {
		t.Fatalf("active record changed after failed undelete")
	}
}

func TestUndeleteMixedRowsKeepResultAlignment(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{
		{
			Object: "Account",
			Fields: map[string]storage.Value{"Name": storage.StringValue("Deleted")},
		},
		{
			Object: "Account",
			Fields: map[string]storage.Value{"Name": storage.StringValue("Active")},
		},
	})
	if len(insert) != 2 || !insert[0].Success || !insert[1].Success {
		t.Fatalf("insert = %#v", insert)
	}
	deletedID := insert[0].ID
	activeID := insert[1].ID
	if deleted := engine.Delete([]storage.Record{{ID: deletedID, Object: "Account"}}); !deleted[0].Success {
		t.Fatalf("delete = %#v", deleted)
	}

	results := engine.Undelete([]storage.Record{
		{ID: deletedID, Object: "Account"},
		{ID: activeID, Object: "Account"},
		{ID: "001999999999999", Object: "Account"},
		{ID: "003000000000001", Object: "Account"},
	})
	if len(results) != 4 {
		t.Fatalf("results len = %d, want 4: %#v", len(results), results)
	}
	if !results[0].Success || results[0].ID != deletedID {
		t.Fatalf("deleted row result = %#v", results[0])
	}
	if results[1].Success || results[1].ID != activeID || results[1].StatusCode != "ENTITY_IS_NOT_DELETED" {
		t.Fatalf("active row result = %#v", results[1])
	}
	if results[2].Success || results[2].ID != "001999999999999" || results[2].StatusCode != "ENTITY_IS_DELETED" {
		t.Fatalf("missing row result = %#v", results[2])
	}
	if results[3].Success || results[3].ID != "003000000000001" || results[3].StatusCode != "INVALID_FIELD" {
		t.Fatalf("mismatched id result = %#v", results[3])
	}
	if org.Objects["Account"].Records[deletedID].System.IsDeleted {
		t.Fatalf("deleted row did not undelete")
	}
	if org.Objects["Account"].Records[activeID].System.IsDeleted {
		t.Fatalf("active row changed")
	}
}

func TestLockAndUnlockToggleSystemLock(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	locked := engine.Lock([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if !locked[0].Success || locked[0].ID != insert[0].ID {
		t.Fatalf("lock = %#v", locked)
	}
	if !org.Objects["Account"].Records[insert[0].ID].System.Locked {
		t.Fatalf("record was not locked: %#v", org.Objects["Account"].Records[insert[0].ID])
	}
	unlocked := engine.Unlock([]storage.Record{{ID: insert[0].ID, Object: "Account"}})
	if !unlocked[0].Success || unlocked[0].ID != insert[0].ID {
		t.Fatalf("unlock = %#v", unlocked)
	}
	if org.Objects["Account"].Records[insert[0].ID].System.Locked {
		t.Fatalf("record remained locked: %#v", org.Objects["Account"].Records[insert[0].ID])
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

func TestValidationRules(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "BlockBadName",
		Active:                true,
		ErrorConditionFormula: `Name = "Blocked"`,
		ErrorMessage:          "blocked by validation rule",
		ErrorDisplayField:     "Name",
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	blockedInsert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Blocked")},
	}})
	if blockedInsert[0].Success || blockedInsert[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" || blockedInsert[0].Fields[0] != "Name" {
		t.Fatalf("blocked insert = %#v", blockedInsert)
	}
	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Allowed")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	blockedUpdate := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Blocked")},
	}})
	if blockedUpdate[0].Success || blockedUpdate[0].Error != "blocked by validation rule" {
		t.Fatalf("blocked update = %#v", blockedUpdate)
	}
}

func TestValidationRulesObservedFormulaFunctions(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["BillingState"] = storage.Field{APIName: "BillingState", Type: storage.FieldString}
	account.Definition.Fields["BillingCountry"] = storage.Field{APIName: "BillingCountry", Type: storage.FieldString}
	account.Definition.Fields["BillingPostalCode"] = storage.Field{APIName: "BillingPostalCode", Type: storage.FieldString}
	account.Definition.ValidationRules = []storage.ValidationRule{
		{
			Name:                  "ValidUSState",
			Active:                true,
			ErrorConditionFormula: `AND(ISBLANK(BillingState) = FALSE, OR(BillingCountry = "US", BillingCountry = "USA", ISBLANK(BillingCountry)), NOT(CONTAINS("CA:NY:WA", BillingState)))`,
			ErrorMessage:          "invalid state",
			ErrorDisplayField:     "BillingState",
		},
		{
			Name:                  "USZip",
			Active:                true,
			ErrorConditionFormula: `AND(ISBLANK(BillingPostalCode) = FALSE, OR(BillingCountry = "US", BillingCountry = "USA"), NOT(REGEX(BillingPostalCode, "\d{5}(-\d{4})?")))`,
			ErrorMessage:          "invalid postal code",
			ErrorDisplayField:     "BillingPostalCode",
		},
		{
			Name:                  "BothAlternates",
			Active:                true,
			ErrorConditionFormula: `NOT(ISBLANK(BindingObject__c)) && NOT(ISBLANK(BindingObjectAlternate__c))`,
			ErrorMessage:          "choose one binding object",
			ErrorDisplayField:     "BindingObject__c",
		},
	}
	account.Definition.Fields["BindingObject__c"] = storage.Field{APIName: "BindingObject__c", Type: storage.FieldString}
	account.Definition.Fields["BindingObjectAlternate__c"] = storage.Field{APIName: "BindingObjectAlternate__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	allowed := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":              storage.StringValue("Allowed"),
			"BillingCountry":    storage.StringValue("US"),
			"BillingState":      storage.StringValue("CA"),
			"BillingPostalCode": storage.StringValue("94105"),
		},
	}})
	if !allowed[0].Success {
		t.Fatalf("allowed insert = %#v", allowed)
	}
	badState := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Bad State"),
			"BillingCountry": storage.StringValue("US"),
			"BillingState":   storage.StringValue("ZZ"),
		},
	}})
	if badState[0].Success || badState[0].Error != "invalid state" || badState[0].Fields[0] != "BillingState" {
		t.Fatalf("bad state = %#v", badState)
	}
	badPostalCode := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":              storage.StringValue("Bad Zip"),
			"BillingCountry":    storage.StringValue("USA"),
			"BillingPostalCode": storage.StringValue("ABCDE"),
		},
	}})
	if badPostalCode[0].Success || badPostalCode[0].Error != "invalid postal code" || badPostalCode[0].Fields[0] != "BillingPostalCode" {
		t.Fatalf("bad postal code = %#v", badPostalCode)
	}
	bothBindings := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":                      storage.StringValue("Both"),
			"BindingObject__c":          storage.StringValue("Account"),
			"BindingObjectAlternate__c": storage.StringValue("Contact"),
		},
	}})
	if bothBindings[0].Success || bothBindings[0].Error != "choose one binding object" || bothBindings[0].Fields[0] != "BindingObject__c" {
		t.Fatalf("both bindings = %#v", bothBindings)
	}
}

func TestWorkflowFieldUpdateCriteriaTrueFalseAndVisibleAfterDML(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "MarkActive",
		Active: true,
		Criteria: []storage.WorkflowCriteriaItem{{
			Field:     "Name",
			Operation: "equals",
			Value:     "Acme",
		}},
		FieldUpdates: []storage.WorkflowFieldUpdate{{
			Name:         "SetStatus",
			Field:        "Status__c",
			LiteralValue: "Active",
		}},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	miss := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Other")},
	}})
	if !miss[0].Success {
		t.Fatalf("miss insert = %#v", miss)
	}
	if _, ok := org.Objects["Account"].Records[miss[0].ID].Fields["Status__c"]; ok {
		t.Fatalf("workflow should not update false criteria record: %#v", org.Objects["Account"].Records[miss[0].ID])
	}

	hit := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !hit[0].Success {
		t.Fatalf("hit insert = %#v", hit)
	}
	if got := org.Objects["Account"].Records[hit[0].ID].Fields["Status__c"].String; got != "Active" {
		t.Fatalf("workflow status after insert = %q", got)
	}

	update := engine.Update([]storage.Record{{
		ID:     miss[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update)
	}
	if got := org.Objects["Account"].Records[miss[0].ID].Fields["Status__c"].String; got != "Active" {
		t.Fatalf("workflow status after update = %q", got)
	}
}

func TestWorkflowFieldUpdateResolvesNamespacedCriteriaAndSourceFields(t *testing.T) {
	org := testOrg()
	org.Namespace = "pkg"
	account := org.Objects["Account"]
	account.Definition.Fields["Source__c"] = storage.Field{APIName: "Source__c", Type: storage.FieldString}
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.Fields["FormulaCopy__c"] = storage.Field{APIName: "FormulaCopy__c", Type: storage.FieldString}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "CopyNamespacedField",
		Active: true,
		Criteria: []storage.WorkflowCriteriaItem{{
			Field:     "pkg__Source__c",
			Operation: "equals",
			Value:     "Ready",
		}},
		FieldUpdates: []storage.WorkflowFieldUpdate{
			{Name: "CopySource", Field: "Status__c", SourceField: "pkg__Source__c"},
			{Name: "CopyFormulaField", Field: "FormulaCopy__c", Formula: "pkg__Source__c"},
		},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Acme"),
			"Source__c": storage.StringValue("Ready"),
		},
	}})
	if !result[0].Success {
		t.Fatalf("insert = %#v", result)
	}
	record := org.Objects["Account"].Records[result[0].ID]
	if got := record.Fields["Status__c"].String; got != "Ready" {
		t.Fatalf("source field copy = %q", got)
	}
	if got := record.Fields["FormulaCopy__c"].String; got != "Ready" {
		t.Fatalf("formula field copy = %q", got)
	}
}

func TestFlowRuleFormulaAndFormulaFieldUpdates(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Type: storage.FieldBoolean}
	account.Definition.Fields["ScoreCopy__c"] = storage.Field{APIName: "ScoreCopy__c", Type: storage.FieldInteger}
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:    "ProcessBuilderStyle",
		Active:  true,
		Formula: `Name = "Acme" && Score__c >= 10`,
		FieldUpdates: []storage.WorkflowFieldUpdate{
			{Name: "SetStatus", Field: "Status__c", Formula: `"Process-" & Name`},
			{Name: "SetActive", Field: "Active__c", LiteralValue: "true"},
			{Name: "CopyScore", Field: "ScoreCopy__c", SourceField: "Score__c"},
		},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	miss := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Other"), "Score__c": storage.IntegerValue(15)},
	}})
	if !miss[0].Success {
		t.Fatalf("miss insert = %#v", miss)
	}
	if _, ok := org.Objects["Account"].Records[miss[0].ID].Fields["Status__c"]; ok {
		t.Fatalf("flow should not update false formula record: %#v", org.Objects["Account"].Records[miss[0].ID])
	}

	hit := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Score__c": storage.IntegerValue(12)},
	}})
	if !hit[0].Success {
		t.Fatalf("hit insert = %#v", hit)
	}
	record := org.Objects["Account"].Records[hit[0].ID]
	if got := record.Fields["Status__c"].String; got != "Process-Acme" {
		t.Fatalf("formula status = %q", got)
	}
	if got := record.Fields["Active__c"].Boolean; !got {
		t.Fatalf("active = %v", got)
	}
	if got := record.Fields["ScoreCopy__c"].Integer; got != 12 {
		t.Fatalf("score copy = %d", got)
	}
}

func TestFlowRecordCreateRunsAndLookupSuppressesDuplicate(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:   "CreateActionRequest",
		Active: true,
		RecordLookups: []storage.FlowRecordLookup{{
			Name:               "ExistingRequest",
			ObjectName:         "ActionRequest__c",
			GetFirstRecordOnly: true,
			Criteria: []storage.WorkflowCriteriaItem{
				{Field: "SourceRecordId__c", Operation: "equals", SourceField: "Id"},
				{Field: "ActionName__c", Operation: "equals", Value: "Notify"},
			},
		}},
		RecordCreates: []storage.FlowRecordCreate{{
			Name:       "CreateRequest",
			ObjectName: "ActionRequest__c",
			InputAssignments: []storage.WorkflowFieldUpdate{
				{Name: "ActionName__c", Field: "ActionName__c", LiteralValue: "Notify"},
				{Name: "SourceRecordId__c", Field: "SourceRecordId__c", SourceField: "Id"},
				{Name: "Payload__c", Field: "Payload__c", SourceField: "Name"},
			},
		}},
	}}
	org.Objects["Account"] = account
	org.Objects["ActionRequest__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ActionRequest__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"ActionName__c":     {APIName: "ActionName__c", Type: storage.FieldString, Required: true},
				"SourceRecordId__c": {APIName: "SourceRecordId__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, Required: true},
				"Payload__c":        {APIName: "Payload__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	var events []string
	engine.AutomationTracer = func(name string, args map[string]any) {
		events = append(events, name)
	}

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	requests := org.Objects["ActionRequest__c"].Records
	if len(requests) != 1 {
		t.Fatalf("requests after insert = %#v", requests)
	}
	var request storage.Record
	for _, candidate := range requests {
		request = candidate
	}
	if got := request.Fields["ActionName__c"].String; got != "Notify" {
		t.Fatalf("action name = %q", got)
	}
	if got := request.Fields["SourceRecordId__c"].ID; got != insert[0].ID {
		t.Fatalf("source record = %q", got)
	}
	if got := request.Fields["Payload__c"].String; got != "Acme" {
		t.Fatalf("payload = %q", got)
	}

	update := engine.Update([]storage.Record{{
		Object: "Account",
		ID:     insert[0].ID,
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme Updated")},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update)
	}
	if got := len(org.Objects["ActionRequest__c"].Records); got != 1 {
		t.Fatalf("lookup should suppress duplicate record create, got %d records", got)
	}
	for _, name := range []string{
		"apex.flow.rule",
		"apex.flow.record_lookup",
		"apex.flow.record_create",
		"apex.flow.record_create_suppressed",
	} {
		if !stringSliceContains(events, name) {
			t.Fatalf("trace missing %s in %#v", name, events)
		}
	}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func TestWorkflowFieldUpdateRejectsInvalidSourceFieldAndRollsBack(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "CopyMissing",
		Active: true,
		Criteria: []storage.WorkflowCriteriaItem{{
			Field:     "Name",
			Operation: "equals",
			Value:     "Acme",
		}},
		FieldUpdates: []storage.WorkflowFieldUpdate{{
			Name:        "BadSource",
			Field:       "Status__c",
			SourceField: "Missing__c",
		}},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
		t.Fatalf("workflow source failure = %#v", result)
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("workflow source failure did not roll back insert: %#v", org.Objects["Account"].Records)
	}
}

func TestWorkflowFieldUpdateRollsBackOnFailure(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:    "BadUpdate",
		Active:  true,
		Formula: `Name = "Acme"`,
		FieldUpdates: []storage.WorkflowFieldUpdate{{
			Name:         "SetMissing",
			Field:        "Missing__c",
			LiteralValue: "bad",
		}},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
		t.Fatalf("workflow failure = %#v", result)
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("workflow failure did not roll back insert: %#v", org.Objects["Account"].Records)
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
	if len(merge[0].UpdatedRelatedIDs) != 1 || merge[0].UpdatedRelatedIDs[0] != child[0].ID {
		t.Fatalf("merge updated related ids = %#v, want %s", merge[0].UpdatedRelatedIDs, child[0].ID)
	}
	if len(merge[0].MergedRecordIDs) != 1 || merge[0].MergedRecordIDs[0] != duplicate[0].ID {
		t.Fatalf("merge merged record ids = %#v, want %s", merge[0].MergedRecordIDs, duplicate[0].ID)
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

func TestCustomMetadataDMLIsReadOnly(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Feature__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Feature__mdt",
			KeyPrefix: "a00",
			Metadata:  map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Feature__mdt", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Default")}},
		},
	}
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{Object: "Feature__mdt", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Other")}}})
	if insert[0].Success || insert[0].StatusCode != "INVALID_TYPE" || !strings.Contains(insert[0].Error, "read-only") {
		t.Fatalf("insert = %#v", insert[0])
	}
	update := engine.Update([]storage.Record{{ID: "a00000000000001", Object: "Feature__mdt", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Changed")}}})
	if update[0].Success || update[0].StatusCode != "INVALID_TYPE" || !strings.Contains(update[0].Error, "read-only") {
		t.Fatalf("update = %#v", update[0])
	}
	deleteResult := engine.Delete([]storage.Record{{ID: "a00000000000001", Object: "Feature__mdt"}})
	if deleteResult[0].Success || deleteResult[0].StatusCode != "INVALID_TYPE" || !strings.Contains(deleteResult[0].Error, "read-only") {
		t.Fatalf("delete = %#v", deleteResult[0])
	}
}

func TestAttachmentBodyDMLAndSOQLRoundTrip(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account insert = %#v", account)
	}
	attachment := engine.Insert([]storage.Record{{
		Object: "Attachment",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("note.txt"),
			"ParentId": storage.IDValue(account[0].ID),
			"Body":     storage.BlobValue("hello bytes"),
		},
	}})
	if !attachment[0].Success || attachment[0].ID != "00P000000000001" {
		t.Fatalf("attachment insert = %#v", attachment)
	}
	query, err := soql.Parse("SELECT Id, Name, ParentId, Body FROM Attachment WHERE Id = '" + string(attachment[0].ID) + "'")
	if err != nil {
		t.Fatal(err)
	}
	result, err := soql.Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("attachment query rows = %d", len(result.Records))
	}
	row := result.Records[0]
	if row.Fields["ParentId"].ID != account[0].ID || row.Fields["Body"].Kind != storage.ValueBlob || row.Fields["Body"].String != "hello bytes" {
		t.Fatalf("attachment row = %#v", row)
	}
}

func TestDocumentBodyDMLSOQLAndDelete(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	document := engine.Insert([]storage.Record{{
		Object: "Document",
		Fields: map[string]storage.Value{
			"Name":          storage.StringValue("Terms.pdf"),
			"DeveloperName": storage.StringValue("Terms"),
			"Body":          storage.BlobValue("document bytes"),
			"ContentType":   storage.StringValue("application/pdf"),
			"Type":          storage.StringValue("pdf"),
			"IsPublic":      storage.BooleanValue(true),
		},
	}})
	if !document[0].Success || document[0].ID != "015000000000001" {
		t.Fatalf("document insert = %#v", document)
	}
	query, err := soql.Parse("SELECT Id, Name, DeveloperName, Body, ContentType, Type, IsPublic FROM Document WHERE Id = '" + string(document[0].ID) + "'")
	if err != nil {
		t.Fatal(err)
	}
	result, err := soql.Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("document query rows = %d", len(result.Records))
	}
	row := result.Records[0]
	if row.Fields["Body"].Kind != storage.ValueBlob || row.Fields["Body"].String != "document bytes" || row.Fields["ContentType"].String != "application/pdf" || !row.Fields["IsPublic"].Boolean {
		t.Fatalf("document row = %#v", row)
	}
	deleted := engine.Delete([]storage.Record{{Object: "Document", ID: document[0].ID}})
	if !deleted[0].Success {
		t.Fatalf("document delete = %#v", deleted)
	}
	result, err = soql.Execute(org, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 0 {
		t.Fatalf("deleted document query rows = %#v", result.Records)
	}
}

func TestContentVersionCreatesDocumentAndLinks(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account insert = %#v", account)
	}
	first := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":                  storage.StringValue("Spec"),
			"PathOnClient":           storage.StringValue("docs/spec.pdf"),
			"VersionData":            storage.BlobValue("pdf bytes"),
			"FirstPublishLocationId": storage.IDValue(account[0].ID),
		},
	}})
	if !first[0].Success || first[0].ID != "068000000000001" {
		t.Fatalf("content version insert = %#v", first)
	}
	version := org.Objects["ContentVersion"].Records[first[0].ID]
	documentID := version.Fields["ContentDocumentId"].ID
	if documentID != "069000000000001" {
		t.Fatalf("content document id = %s", documentID)
	}
	document := org.Objects["ContentDocument"].Records[documentID]
	if document.Fields["LatestPublishedVersionId"].ID != first[0].ID || document.Fields["Title"].String != "Spec" || document.Fields["FileExtension"].String != "pdf" {
		t.Fatalf("content document = %#v", document)
	}
	if got := len(org.Objects["ContentDocumentLink"].Records); got != 1 {
		t.Fatalf("content document links = %d", got)
	}
	var autoLink storage.Record
	for _, link := range org.Objects["ContentDocumentLink"].Records {
		autoLink = link
	}
	if autoLink.Fields["ContentDocumentId"].ID != documentID || autoLink.Fields["LinkedEntityId"].ID != account[0].ID || autoLink.Fields["ShareType"].String != "V" {
		t.Fatalf("auto link = %#v", autoLink)
	}

	second := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":             storage.StringValue("Spec v2"),
			"PathOnClient":      storage.StringValue("docs/spec-v2.pdf"),
			"VersionData":       storage.BlobValue("second bytes"),
			"ContentDocumentId": storage.IDValue(documentID),
		},
	}})
	if !second[0].Success || second[0].ID != "068000000000002" {
		t.Fatalf("second content version insert = %#v", second)
	}
	if got := len(org.Objects["ContentDocument"].Records); got != 1 {
		t.Fatalf("content documents = %d", got)
	}
	document = org.Objects["ContentDocument"].Records[documentID]
	if document.Fields["LatestPublishedVersionId"].ID != second[0].ID || document.Fields["Title"].String != "Spec v2" || document.Fields["FileExtension"].String != "pdf" {
		t.Fatalf("updated content document = %#v", document)
	}
	firstVersion := org.Objects["ContentVersion"].Records[first[0].ID]
	secondVersion := org.Objects["ContentVersion"].Records[second[0].ID]
	if firstVersion.Fields["IsLatest"].Boolean || !secondVersion.Fields["IsLatest"].Boolean {
		t.Fatalf("content version latest flags: first=%#v second=%#v", firstVersion.Fields["IsLatest"], secondVersion.Fields["IsLatest"])
	}
	explicitLink := engine.Insert([]storage.Record{{
		Object: "ContentDocumentLink",
		Fields: map[string]storage.Value{
			"ContentDocumentId": storage.IDValue(documentID),
			"LinkedEntityId":    storage.IDValue(account[0].ID),
			"ShareType":         storage.StringValue("C"),
			"Visibility":        storage.StringValue("InternalUsers"),
		},
	}})
	if !explicitLink[0].Success || explicitLink[0].ID != "06A000000000002" {
		t.Fatalf("explicit link insert = %#v", explicitLink)
	}
}

func TestContentVersionTransactionRollbackRemovesDocumentAndLink(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account insert = %#v", account)
	}
	err := engine.WithTransaction(func(tx *Engine) error {
		insert := tx.Insert([]storage.Record{{
			Object: "ContentVersion",
			Fields: map[string]storage.Value{
				"Title":                  storage.StringValue("Spec"),
				"PathOnClient":           storage.StringValue("docs/spec.txt"),
				"VersionData":            storage.BlobValue("version bytes"),
				"FirstPublishLocationId": storage.IDValue(account[0].ID),
			},
		}})
		if !insert[0].Success {
			t.Fatalf("content version insert = %#v", insert)
		}
		return errors.New("rollback")
	})
	if err == nil {
		t.Fatal("expected transaction rollback")
	}
	if got := len(org.Objects["ContentVersion"].Records); got != 0 {
		t.Fatalf("content versions after rollback = %d", got)
	}
	if got := len(org.Objects["ContentDocument"].Records); got != 0 {
		t.Fatalf("content documents after rollback = %d", got)
	}
	if got := len(org.Objects["ContentDocumentLink"].Records); got != 0 {
		t.Fatalf("content document links after rollback = %d", got)
	}
	if got := len(org.Objects["Account"].Records); got != 1 {
		t.Fatalf("accounts after rollback = %d", got)
	}
}

func TestContentVersionRejectsMissingContentDocument(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":             storage.StringValue("Spec"),
			"VersionData":       storage.BlobValue("bytes"),
			"ContentDocumentId": storage.IDValue("069000000000999"),
		},
	}})
	if result[0].Success || result[0].StatusCode != "FIELD_INTEGRITY_EXCEPTION" {
		t.Fatalf("content version insert = %#v", result)
	}
	if len(org.Objects["ContentVersion"].Records) != 0 {
		t.Fatalf("content version was stored after failed insert: %#v", org.Objects["ContentVersion"].Records)
	}
	if _, exists := org.Objects["ContentDocument"].Records["069000000000999"]; exists {
		t.Fatalf("missing content document was created")
	}
}

func fileTestOrg() storage.OrgState {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	return org
}
