package dml

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
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
	if !insert[0].Success || insert[0].ID == "" || insert[0].ID == "001000000000001" {
		t.Fatalf("insert = %#v", insert)
	}
	if org.IDSequences["Account"] != 1 {
		t.Fatalf("account id sequence = %d, want 1", org.IDSequences["Account"])
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

func TestInsertProbeTestObjectDoesNotCopyCustomNameToStandardName(t *testing.T) {
	org := testOrg()
	org.Objects["ProbeTestObject__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ProbeTestObject__c",
			KeyPrefix: "a0p",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString, Required: true},
				"Name__c": {APIName: "Name__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "ProbeTestObject__c",
		Fields: map[string]storage.Value{
			"Name__c": storage.StringValue("Probe"),
		},
	}})

	if result[0].Success {
		t.Fatalf("insert succeeded: %#v", result[0])
	}
	if !strings.Contains(result[0].Error, "Required fields are missing: [Name]") {
		t.Fatalf("insert error = %q", result[0].Error)
	}
}

func TestInsertCustomRelationshipFieldRequiresMetadata(t *testing.T) {
	org := testOrg()
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Line__c",
			KeyPrefix: "a0l",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Parent__r": storage.NullValue(),
		},
	}})

	if result[0].Success {
		t.Fatalf("insert succeeded: %#v", result[0])
	}
	if !strings.Contains(result[0].Error, "unknown field Line__c.Parent__r") {
		t.Fatalf("insert error = %q", result[0].Error)
	}
}

func TestInsertAdvancesSystemTimestampsWhenClockIsFixed(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	engine.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }

	first := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("First")},
	}})
	second := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Second")},
	}})
	if !first[0].Success || !second[0].Success {
		t.Fatalf("insert results = %#v %#v", first, second)
	}

	firstStamp := org.Objects["Account"].Records[first[0].ID].System.CreatedDate
	secondStamp := org.Objects["Account"].Records[second[0].ID].System.CreatedDate
	if firstStamp != "2026-05-02T12:00:00Z" {
		t.Fatalf("first CreatedDate = %q", firstStamp)
	}
	if secondStamp != "2026-05-02T12:00:01Z" {
		t.Fatalf("second CreatedDate = %q", secondStamp)
	}
}

func TestBulkInsertUsesOneSystemTimestampWhenClockIsFixed(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	engine.Now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }

	results := engine.Insert([]storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("First")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Second")}},
	})
	if !results[0].Success || !results[1].Success {
		t.Fatalf("insert results = %#v", results)
	}

	first := org.Objects["Account"].Records[results[0].ID]
	second := org.Objects["Account"].Records[results[1].ID]
	if first.System.CreatedDate != "2026-05-02T12:00:00Z" || second.System.CreatedDate != first.System.CreatedDate {
		t.Fatalf("bulk insert created dates = %q, %q", first.System.CreatedDate, second.System.CreatedDate)
	}
	if first.System.LastModifiedDate != first.System.CreatedDate || second.System.LastModifiedDate != second.System.CreatedDate {
		t.Fatalf("bulk insert modified dates = %#v, %#v", first.System, second.System)
	}

	next := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Third")},
	}})
	if !next[0].Success {
		t.Fatalf("next insert = %#v", next)
	}
	if got := org.Objects["Account"].Records[next[0].ID].System.CreatedDate; got != "2026-05-02T12:00:01Z" {
		t.Fatalf("next CreatedDate = %q", got)
	}
}

func TestDMLRejectsInvalidSystemOwnerIDPrefix(t *testing.T) {
	org := testOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{
		Object: "Widget__c",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Bad owner")},
		System: storage.SystemFields{OwnerID: "003000000000001"},
	}})
	if result[0].Success || result[0].StatusCode != "FIELD_INTEGRITY_EXCEPTION" || len(result[0].Fields) != 1 || result[0].Fields[0] != "OwnerId" {
		t.Fatalf("insert = %#v", result[0])
	}
}

func TestValidationFormulaResolvesForeignNamespacedRelationshipFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "namz"
	org.Objects["pkg__Entity__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "pkg__Entity__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"a01000000000001EAA": {ID: "a01000000000001EAA", Object: "pkg__Entity__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Primary")}},
		},
	}
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "pkg__Product__c", KeyPrefix: "a02", Fields: map[string]storage.Field{
			"pkg__Entity__c": {APIName: "pkg__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Entity__c"}, RelationshipName: "pkg__Entity__r"},
		}},
		Records: map[storage.ID]storage.Record{
			"a02000000000001EAA": {ID: "a02000000000001EAA", Object: "pkg__Product__c", Fields: map[string]storage.Value{"pkg__Entity__c": storage.IDValue("a01000000000001EAA")}},
		},
	}
	org.Objects["pkg__CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "pkg__CartItem__c", KeyPrefix: "a03", Fields: map[string]storage.Field{
			"pkg__Entity__c": {APIName: "pkg__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Entity__c"}, RelationshipName: "pkg__Entity__r"},
		}},
		Records: map[storage.ID]storage.Record{
			"a03000000000001EAA": {ID: "a03000000000001EAA", Object: "pkg__CartItem__c", Fields: map[string]storage.Value{"pkg__Entity__c": storage.IDValue("a01000000000001EAA")}},
		},
	}
	org.Objects["pkg__CartItemLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__CartItemLine__c",
			KeyPrefix: "a04",
			Fields: map[string]storage.Field{
				"pkg__Product2__c": {APIName: "pkg__Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Product__c"}, RelationshipName: "pkg__Product2__r"},
				"pkg__CartItem__c": {APIName: "pkg__CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__CartItem__c"}, RelationshipName: "pkg__CartItem__r"},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "EntityMustMatch",
				Active:                true,
				ErrorConditionFormula: "Product2__r.Entity__c <> CartItem__r.Entity__c",
				ErrorMessage:          "entity mismatch",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}

	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{
		Object: "pkg__CartItemLine__c",
		Fields: map[string]storage.Value{
			"pkg__Product2__c": storage.IDValue("a02000000000001EAA"),
			"pkg__CartItem__c": storage.IDValue("a03000000000001EAA"),
		},
	}})
	if !result[0].Success {
		t.Fatalf("insert = %#v", result)
	}
}

func TestValidationFormulaUsesObjectNamespaceBeforeOrgNamespace(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "namz"
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Line__c",
			KeyPrefix: "a04",
			Fields: map[string]storage.Field{
				"pkg__Entity__c":  {APIName: "pkg__Entity__c", Type: storage.FieldString},
				"namz__Entity__c": {APIName: "namz__Entity__c", Type: storage.FieldString},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "EntityMustMatch",
				Active:                true,
				ErrorConditionFormula: `Entity__c = "wrong"`,
				ErrorMessage:          "entity mismatch",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}

	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{
		Object: "pkg__Line__c",
		Fields: map[string]storage.Value{
			"pkg__Entity__c":  storage.StringValue("right"),
			"namz__Entity__c": storage.StringValue("wrong"),
		},
	}})
	if !result[0].Success {
		t.Fatalf("insert = %#v", result)
	}
}

func TestUpdateRequiredFieldToNullFails(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	var organizationRecordTypeID storage.ID
	for _, recordType := range org.Objects["Account"].Definition.RecordTypes {
		if recordType.DeveloperName == "Organization" {
			organizationRecordTypeID = recordType.ID
			break
		}
	}
	if organizationRecordTypeID == "" {
		t.Fatalf("missing organization record type: %#v", org.Objects["Account"].Definition.RecordTypes)
	}

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	update := engine.Update([]storage.Record{{
		ID:            insert[0].ID,
		Object:        "Account",
		ExplicitNulls: map[string]bool{"Name": true},
	}})
	if update[0].Success || update[0].Error == "" {
		t.Fatalf("expected required field failure, got %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Acme" {
		t.Fatalf("stored name after failed update = %q", got)
	}
}

func TestUpdateCustomObjectNameFieldToNullSucceeds(t *testing.T) {
	org := testOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Widget__c",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Widget")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	update := engine.Update([]storage.Record{{
		ID:            insert[0].ID,
		Object:        "Widget__c",
		ExplicitNulls: map[string]bool{"Name": true},
	}})
	if !update[0].Success {
		t.Fatalf("custom object name null update = %#v", update)
	}
	if stored := org.Objects["Widget__c"].Records[insert[0].ID]; !stored.HasExplicitNull("Name") {
		t.Fatalf("stored explicit nulls = %#v fields = %#v", stored.ExplicitNulls, stored.Fields)
	}
}

func TestUpdateStandardNameFieldToBlankFails(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("")},
	}})
	if update[0].Success || update[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("blank standard name update = %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Acme" {
		t.Fatalf("stored name after failed update = %q", got)
	}
}

func TestUpdateSparseExistingRecordDoesNotRequireAllRequiredFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{},
	}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	update := engine.Update([]storage.Record{{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{"Score__c": storage.IntegerValue(42)},
	}})
	if !update[0].Success {
		t.Fatalf("sparse update = %#v", update[0])
	}
	if got := org.Objects["Account"].Records["001000000000001"].Fields["Score__c"].Integer; got != 42 {
		t.Fatalf("score = %d", got)
	}
}

func TestInsertStoresBlankOptionalTextAsNull(t *testing.T) {
	org := testOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
				"Description__c": {APIName: "Description__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Widget__c",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Widget"),
			"Description__c": storage.StringValue(""),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	stored := org.Objects["Widget__c"].Records[insert[0].ID]
	if _, ok := stored.Fields["Description__c"]; ok {
		t.Fatalf("Description__c stored as field: %#v", stored.Fields["Description__c"])
	}
	if !stored.HasExplicitNull("Description__c") {
		t.Fatalf("Description__c explicit null missing: %#v", stored.ExplicitNulls)
	}
}

func TestUpdateExplicitNullAfterRuntimeSnapshot(t *testing.T) {
	org := testOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
				"Description__c": {APIName: "Description__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Widget__c",
				Fields: map[string]storage.Value{"Name": storage.StringValue("Widget")},
			},
		},
	}
	engine := NewEngine(&org)

	_ = storage.SnapshotRuntimeOrg(&org)
	update := engine.Update([]storage.Record{{
		ID:            "a00000000000001",
		Object:        "Widget__c",
		ExplicitNulls: map[string]bool{"Description__c": true},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update[0])
	}
	stored := org.Objects["Widget__c"].Records["a00000000000001"]
	if !stored.HasExplicitNull("Description__c") {
		t.Fatalf("Description__c explicit null missing: %#v", stored.ExplicitNulls)
	}
}

func TestInsertRejectsOverlongSingleLineText(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Short__c"] = storage.Field{APIName: "Short__c", Type: storage.FieldString, Length: 3}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Short__c": storage.StringValue("abcd"),
		},
	}})
	if insert[0].Success || insert[0].StatusCode != "STRING_TOO_LONG" || insert[0].ID != "" {
		t.Fatalf("overlong insert = %#v", insert[0])
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("record persisted after overlong insert: %#v", org.Objects["Account"].Records)
	}
}

func TestInsertTruncatesOverlongSingleLineTextWhenAllowed(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Short__c"] = storage.Field{APIName: "Short__c", Type: storage.FieldString, Length: 3}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	engine.Options.AllowFieldTruncation = true

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Short__c": storage.StringValue("abcd"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("truncating insert = %#v", insert[0])
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Short__c"].String; got != "abc" {
		t.Fatalf("Short__c = %q", got)
	}
}

func TestUpdateRequiredFieldCaseVariantCanonicalizesAlias(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{"name": storage.StringValue("Renamed")},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if got := stored.Fields["Name"].String; got != "Renamed" {
		t.Fatalf("stored name = %q", got)
	}
}

func TestUpdateCanonicalAliasReplacesStoredNamespacedField(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["ExternalPaymentProfile__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ExternalPaymentProfile__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"FirstName__c": {APIName: "FirstName__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a10000000000001AAA": {
				Object: "ExternalPaymentProfile__c",
				ID:     "a10000000000001AAA",
				Fields: map[string]storage.Value{
					"NU__FirstName__c": storage.StringValue("Eric"),
				},
			},
		},
	}
	engine := NewEngine(&org)

	update := engine.Update([]storage.Record{{
		ID:     "a10000000000001AAA",
		Object: "ExternalPaymentProfile__c",
		Fields: map[string]storage.Value{"FirstName__c": storage.StringValue("Eddy")},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update[0])
	}
	stored := org.Objects["ExternalPaymentProfile__c"].Records["a10000000000001AAA"]
	if _, ok := stored.Fields["NU__FirstName__c"]; ok {
		t.Fatalf("stored namespaced alias remained: %#v", stored.Fields)
	}
	if got := stored.Fields["FirstName__c"].String; got != "Eddy" {
		t.Fatalf("stored first name = %q", got)
	}
}

func TestUpdateMatchesStoredRecordByFifteenCharacterID(t *testing.T) {
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

	id18 := storage.ID(string(insert[0].ID) + "AAA")
	update := engine.Update([]storage.Record{{
		ID:     id18,
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
}

func TestInsertAppliesAutoNumberName(t *testing.T) {
	org := testOrg()
	org.Objects["Order__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Order__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true, AutoNumber: true, DisplayFormat: "Order {0000000}"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{Object: "Order__c"}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	if got := org.Objects["Order__c"].Records[insert[0].ID].Fields["Name"].String; got != "Order 0000001" {
		t.Fatalf("auto number name = %q", got)
	}
}

func TestContactUpdateRecomputesNameFromFirstAndLastName(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"FirstName": {APIName: "FirstName", Type: storage.FieldString},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Old"),
			"LastName":  storage.StringValue("Name"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Contact",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Old Name"),
			"FirstName": storage.StringValue("New"),
			"LastName":  storage.StringValue("Person"),
		},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update)
	}
	if got := org.Objects["Contact"].Records[insert[0].ID].Fields["Name"].String; got != "New Person" {
		t.Fatalf("updated contact name = %q", got)
	}
}

func TestDMLCollapsesNewlinesForSingleLineTextFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["SingleLine__c"] = storage.Field{APIName: "SingleLine__c", Type: storage.FieldString}
	account.Definition.Fields["LongText__c"] = storage.Field{APIName: "LongText__c", Type: storage.FieldString, DisplayType: "TEXTAREA"}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":          storage.StringValue("Acme"),
			"SingleLine__c": storage.StringValue("English\nFrench\r\nSpanish\rCreole"),
			"LongText__c":   storage.StringValue("English\nFrench"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if got := stored.Fields["SingleLine__c"].String; got != "English French Spanish Creole" {
		t.Fatalf("single-line text = %q", got)
	}
	if got := stored.Fields["LongText__c"].String; got != "English\nFrench" {
		t.Fatalf("textarea text = %q", got)
	}
}

func TestNormalizeStoredFieldValuePlainTextDoesNotAllocate(t *testing.T) {
	field := storage.Field{APIName: "Name", Type: storage.FieldString}
	value := storage.StringValue("Acme")
	if got := normalizeStoredFieldValue(field, value); got.String != "Acme" {
		t.Fatalf("normalized = %#v", got)
	}
	allocs := testing.AllocsPerRun(1000, func() {
		normalizeStoredFieldValue(field, value)
	})
	if allocs != 0 {
		t.Fatalf("allocs = %.2f, want 0", allocs)
	}
}

func TestDMLRecalculatesSummaryFields(t *testing.T) {
	org := testOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString},
				"MaxDate__c":      {APIName: "MaxDate__c", Type: storage.FieldSummary, SummarizedField: "Child__c.AdjustedOn__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "max"},
				"MaxTerm__c":      {APIName: "MaxTerm__c", Type: storage.FieldSummary, SummarizedField: "Child__c.Term__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "max"},
				"Count__c":        {APIName: "Count__c", Type: storage.FieldSummary, SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "count"},
				"Paid__c":         {APIName: "Paid__c", Type: storage.FieldDecimal},
				"FormulaTotal__c": {APIName: "FormulaTotal__c", Type: storage.FieldSummary, SummarizedField: "Child__c.FormulaAmount__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "sum"},
				"Total__c":        {APIName: "Total__c", Type: storage.FieldSummary, SummarizedField: "Child__c.Amount__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "sum"},
				"Balance__c":      {APIName: "Balance__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "Total__c - Paid__c"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Parent__c":        {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
				"AdjustedOn__c":    {APIName: "AdjustedOn__c", Type: storage.FieldDate},
				"Term__c":          {APIName: "Term__c", Type: storage.FieldDecimal},
				"Amount__c":        {APIName: "Amount__c", Type: storage.FieldDecimal},
				"FormulaAmount__c": {APIName: "FormulaAmount__c", Type: storage.FieldDecimal, Formula: "Amount__c * 2"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Parent__c"}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent[0])
	}
	children := engine.Insert([]storage.Record{
		{Object: "Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parent[0].ID), "AdjustedOn__c": storage.DateValue("2026-01-01"), "Term__c": storage.DecimalValue("1"), "Amount__c": storage.DecimalValue("5")}},
		{Object: "Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parent[0].ID + "AAA"), "AdjustedOn__c": storage.DateValue("2026-01-02"), "Term__c": storage.DecimalValue("12"), "Amount__c": storage.DecimalValue("7")}},
	})
	if !children[0].Success || !children[1].Success {
		t.Fatalf("child insert = %#v", children)
	}
	stored := org.Objects["Parent__c"].Records[parent[0].ID]
	if got := stored.Fields["MaxDate__c"].String; got != "2026-01-02" {
		t.Fatalf("date max summary after insert = %q", got)
	}
	if got := stored.Fields["MaxTerm__c"].Decimal; got != "12" {
		t.Fatalf("max summary after insert = %q", got)
	}
	if got := stored.Fields["Total__c"].Decimal; got != "12" {
		t.Fatalf("sum summary after insert = %q", got)
	}
	if got := stored.Fields["FormulaTotal__c"].Decimal; got != "24" {
		t.Fatalf("formula sum summary after insert = %q", got)
	}
	if got := stored.Fields["Count__c"].Integer; got != 2 {
		t.Fatalf("count summary after insert = %d", got)
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(org.Objects["Parent__c"].Definition.Fields["Balance__c"].Formula, org.Objects["Parent__c"].Definition.Fields["Balance__c"], &org, org.Objects["Parent__c"].Definition, storage.Record{
		Object: "Parent__c",
		ID:     parent[0].ID,
		Fields: map[string]storage.Value{
			"Paid__c": storage.DecimalValue("2"),
		},
	})
	if !ok || value.Kind != storage.ValueDecimal || value.Decimal != "10" {
		t.Fatalf("formula over summary = %#v, ok=%v; want decimal 10", value, ok)
	}
	parentUpdate := engine.Update([]storage.Record{{ID: parent[0].ID, Object: "Parent__c", Fields: map[string]storage.Value{
		"Name":       storage.StringValue("Updated"),
		"MaxTerm__c": stored.Fields["MaxTerm__c"],
		"Total__c":   stored.Fields["Total__c"],
	}}})
	if !parentUpdate[0].Success {
		t.Fatalf("parent update with unchanged summaries = %#v", parentUpdate[0])
	}
	formulaBackedUpdate := engine.Update([]storage.Record{{ID: children[0].ID, Object: "Child__c", Fields: map[string]storage.Value{
		"Amount__c":        storage.DecimalValue("6"),
		"FormulaAmount__c": storage.DecimalValue("0"),
	}}})
	if !formulaBackedUpdate[0].Success {
		t.Fatalf("child update with formula-backed field = %#v", formulaBackedUpdate[0])
	}
	stored = org.Objects["Parent__c"].Records[parent[0].ID]
	if got := stored.Fields["FormulaTotal__c"].Decimal; got != "26" {
		t.Fatalf("formula sum summary after formula-backed update = %q", got)
	}
	update := engine.Update([]storage.Record{{ID: children[1].ID, Object: "Child__c", Fields: map[string]storage.Value{"AdjustedOn__c": storage.DateValue("2025-12-31"), "Term__c": storage.DecimalValue("2"), "Amount__c": storage.DecimalValue("3")}}})
	if !update[0].Success {
		t.Fatalf("child update = %#v", update[0])
	}
	stored = org.Objects["Parent__c"].Records[parent[0].ID]
	if got := stored.Fields["MaxDate__c"].String; got != "2026-01-01" {
		t.Fatalf("date max summary after update = %q", got)
	}
	if got := stored.Fields["MaxTerm__c"].Decimal; got != "2" {
		t.Fatalf("max summary after update = %q", got)
	}
	if got := stored.Fields["Total__c"].Decimal; got != "9" {
		t.Fatalf("sum summary after update = %q", got)
	}
	if got := stored.Fields["FormulaTotal__c"].Decimal; got != "18" {
		t.Fatalf("formula sum summary after update = %q", got)
	}
	if got := stored.Fields["Count__c"].Integer; got != 2 {
		t.Fatalf("count summary after update = %d", got)
	}
	deleteResult := engine.Delete([]storage.Record{{ID: children[1].ID, Object: "Child__c"}})
	if !deleteResult[0].Success {
		t.Fatalf("child delete = %#v", deleteResult[0])
	}
	stored = org.Objects["Parent__c"].Records[parent[0].ID]
	if got := stored.Fields["MaxDate__c"].String; got != "2026-01-01" {
		t.Fatalf("date max summary after delete = %q", got)
	}
	if got := stored.Fields["MaxTerm__c"].Decimal; got != "1" {
		t.Fatalf("max summary after delete = %q", got)
	}
	if got := stored.Fields["Total__c"].Decimal; got != "6" {
		t.Fatalf("sum summary after delete = %q", got)
	}
	if got := stored.Fields["Count__c"].Integer; got != 1 {
		t.Fatalf("count summary after delete = %d", got)
	}
}

func TestDMLRecalculatesSummaryFieldsWithNestedFormulaAndDefaultFilter(t *testing.T) {
	org := testOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"SubTotal__c": {
					APIName:           "SubTotal__c",
					Type:              storage.FieldSummary,
					DisplayType:       "Currency",
					SummarizedField:   "Child__c.Total__c",
					SummaryForeignKey: "Child__c.Parent__c",
					SummaryOperation:  "sum",
					SummaryFilterItems: []storage.SummaryFilterItem{{
						Field:     "Child__c.IsCoupon__c",
						Operation: "equals",
						Value:     "False",
					}},
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Parent__c":    {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
				"Quantity__c":  {APIName: "Quantity__c", Type: storage.FieldInteger},
				"UnitPrice__c": {APIName: "UnitPrice__c", Type: storage.FieldDecimal},
				"SubTotal__c":  {APIName: "SubTotal__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "Quantity__c * UnitPrice__c"},
				"Total__c":     {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "SubTotal__c"},
				"IsCoupon__c":  {APIName: "IsCoupon__c", Type: storage.FieldBoolean, DefaultValue: "false"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Parent__c"}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent[0])
	}
	child := engine.Insert([]storage.Record{{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Parent__c":    storage.IDValue(parent[0].ID),
			"Quantity__c":  storage.IntegerValue(1),
			"UnitPrice__c": storage.DecimalValue("38.50"),
		},
	}})
	if !child[0].Success {
		t.Fatalf("child insert = %#v", child[0])
	}
	stored := org.Objects["Parent__c"].Records[parent[0].ID]
	if got := stored.Fields["SubTotal__c"].Decimal; got != "38.5" && got != "38.50" {
		t.Fatalf("nested formula summary = %q, want 38.50", got)
	}
}

func TestDMLCascadesSummaryFieldRecalculation(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Grandparent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Grandparent__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Total__c": {APIName: "Total__c", Type: storage.FieldSummary, SummarizedField: "Parent__c.Total__c", SummaryForeignKey: "Parent__c.Grandparent__c", SummaryOperation: "sum"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Grandparent__c": {APIName: "Grandparent__c", Type: storage.FieldReference, ReferenceTo: []string{"Grandparent__c"}},
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"Total__c":       {APIName: "Total__c", Type: storage.FieldSummary, SummarizedField: "Child__c.Amount__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "sum"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	grandparent := engine.Insert([]storage.Record{{Object: "Grandparent__c"}})
	if !grandparent[0].Success {
		t.Fatalf("grandparent insert = %#v", grandparent[0])
	}
	parent := engine.Insert([]storage.Record{{
		Object: "Parent__c",
		Fields: map[string]storage.Value{"Grandparent__c": storage.IDValue(grandparent[0].ID)},
	}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent[0])
	}
	child := engine.Insert([]storage.Record{{
		Object: "Child__c",
		Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parent[0].ID), "Amount__c": storage.DecimalValue("42")},
	}})
	if !child[0].Success {
		t.Fatalf("child insert = %#v", child[0])
	}
	if got := org.Objects["Parent__c"].Records[parent[0].ID].Fields["Total__c"].Decimal; got != "42" {
		t.Fatalf("parent total = %q", got)
	}
	if got := org.Objects["Grandparent__c"].Records[grandparent[0].ID].Fields["Total__c"].Decimal; got != "42" {
		t.Fatalf("grandparent total = %q", got)
	}
	parentUpdate := engine.Update([]storage.Record{{ID: parent[0].ID, Object: "Parent__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Updated")}}})
	if !parentUpdate[0].Success {
		t.Fatalf("parent update = %#v", parentUpdate[0])
	}
	if got := org.Objects["Parent__c"].Records[parent[0].ID].Fields["Total__c"].Decimal; got != "42" {
		t.Fatalf("parent total after unrelated update = %q", got)
	}
	if got := org.Objects["Grandparent__c"].Records[grandparent[0].ID].Fields["Total__c"].Decimal; got != "42" {
		t.Fatalf("grandparent total after unrelated parent update = %q", got)
	}
}

func TestInsertAppliesRecordTypeFormulaDefaults(t *testing.T) {
	org := testOrg()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
				"RecordTypeId":   {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
				"QuantityMax__c": {APIName: "QuantityMax__c", Type: storage.FieldDecimal, Required: true, DefaultValue: "IF($RecordType.Name == 'Merchandise', 999, 1)"},
				"TypeName__c":    {APIName: "TypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000001AAA", DeveloperName: "Merchandise", Name: "Merchandise"},
				{ID: "012000000000002AAA", DeveloperName: "Subscription", Name: "Subscription"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000001AAA": {Object: "RecordType", ID: "012000000000001AAA"},
			"012000000000002AAA": {Object: "RecordType", ID: "012000000000002AAA"},
		},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Product__c",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Subscription"),
			"RecordTypeId": storage.IDValue("012000000000002AAA"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	got := org.Objects["Product__c"].Records[insert[0].ID].Fields["QuantityMax__c"]
	if got.Kind != storage.ValueDecimal || got.Decimal != "1" {
		t.Fatalf("QuantityMax__c = %#v; want decimal 1", got)
	}
	typeName := org.Objects["Product__c"].Records[insert[0].ID].Fields["TypeName__c"]
	if typeName.Kind != storage.ValueString || typeName.String != "Subscription" {
		t.Fatalf("TypeName__c = %#v; want Subscription", typeName)
	}
}

func TestInsertDefaultsRecordTypeIDAndRecordTypePicklistDefaults(t *testing.T) {
	org := testOrg()
	org.Objects["FacilityCredentialingEvent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "FacilityCredentialingEvent__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString, Required: true},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
				"Type__c":      {APIName: "Type__c", Type: storage.FieldPicklist},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{
					ID:               "012000000000011AAA",
					DeveloperName:    "Internal",
					Name:             "Internal",
					Active:           true,
					Available:        true,
					Default:          true,
					PicklistDefaults: map[string]string{"Type__c": "Initial"},
				},
				{
					ID:            "012000000000012AAA",
					DeveloperName: "CVO",
					Name:          "CVO",
					Active:        true,
					Available:     true,
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
				"SobjectType":   {APIName: "SobjectType", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000011AAA": {Object: "RecordType", ID: "012000000000011AAA"},
			"012000000000012AAA": {Object: "RecordType", ID: "012000000000012AAA"},
		},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "FacilityCredentialingEvent__c",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Internal"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	stored := org.Objects["FacilityCredentialingEvent__c"].Records[insert[0].ID]
	if got := stored.Fields["RecordTypeId"]; got.Kind != storage.ValueID || got.ID != "012000000000011AAA" {
		t.Fatalf("RecordTypeId = %#v", got)
	}
	if got := stored.Fields["Type__c"]; got.Kind != storage.ValueString || got.String != "Initial" {
		t.Fatalf("Type__c = %#v", got)
	}
}

func TestInsertUpdateCanonicalizesPicklistValueCasing(t *testing.T) {
	org := testOrg()
	org.Objects["FacilitySE__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "FacilitySE__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
				"Type__c": {
					APIName: "Type__c",
					Type:    storage.FieldPicklist,
					PicklistValues: []storage.PicklistValue{
						{Value: "OigExclusions", Active: true},
						{Value: "Sam", Active: true},
					},
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "FacilitySE__c",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Facility SE"),
			"Type__c": storage.StringValue("OIGExclusions"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	if got := org.Objects["FacilitySE__c"].Records[insert[0].ID].Fields["Type__c"]; got.Kind != storage.ValueString || got.String != "OigExclusions" {
		t.Fatalf("insert Type__c = %#v; want OigExclusions", got)
	}

	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "FacilitySE__c",
		Fields: map[string]storage.Value{
			"Type__c": storage.StringValue("sam"),
		},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update[0])
	}
	if got := org.Objects["FacilitySE__c"].Records[insert[0].ID].Fields["Type__c"]; got.Kind != storage.ValueString || got.String != "Sam" {
		t.Fatalf("update Type__c = %#v; want Sam", got)
	}
}

func TestInsertRejectsInvalidRestrictedPicklistValue(t *testing.T) {
	org := testOrg()
	org.Objects["FacilitySE__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "FacilitySE__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
				"Type__c": {
					APIName:            "Type__c",
					Type:               storage.FieldPicklist,
					RestrictedPicklist: true,
					PicklistValues: []storage.PicklistValue{
						{Value: "OigExclusions", Active: true},
						{Value: "Sam", Active: true},
					},
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "FacilitySE__c",
		Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Facility SE"),
			"Type__c": storage.StringValue("Bogus"),
		},
	}})

	if insert[0].Success || insert[0].StatusCode != "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST" || len(insert[0].Fields) != 1 || insert[0].Fields[0] != "Type__c" {
		t.Fatalf("restricted picklist insert = %#v", insert[0])
	}
	if len(org.Objects["FacilitySE__c"].Records) != 0 {
		t.Fatalf("record persisted after restricted picklist failure: %#v", org.Objects["FacilitySE__c"].Records)
	}
}

func TestInsertDoesNotInferAmbiguousRecordTypeFromPicklistDefaults(t *testing.T) {
	org := testOrg()
	org.Objects["Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Event__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString, Required: true},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
				"Type__c":      {APIName: "Type__c", Type: storage.FieldPicklist},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000021AAA", DeveloperName: "External", Name: "External", Active: true, Available: true},
				{ID: "012000000000022AAA", DeveloperName: "Internal", Name: "Internal", Active: true, Available: true, PicklistDefaults: map[string]string{"Type__c": "Initial"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{}},
		Records: map[storage.ID]storage.Record{
			"012000000000021AAA": {Object: "RecordType", ID: "012000000000021AAA"},
			"012000000000022AAA": {Object: "RecordType", ID: "012000000000022AAA"},
		},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Event__c",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Event")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	stored := org.Objects["Event__c"].Records[insert[0].ID]
	if got, ok := stored.Fields["RecordTypeId"]; ok {
		t.Fatalf("RecordTypeId = %#v", got)
	}
	if got, ok := stored.Fields["Type__c"]; ok {
		t.Fatalf("Type__c = %#v", got)
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
	blank := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue(""),
		},
	}})
	if blank[0].Success || blank[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("blank required result = %#v", blank)
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

func TestInsertSynthesizesMissingCustomObject(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "pkg__PriceClass__c",
		Fields: map[string]storage.Value{
			"Name":               storage.StringValue("Default"),
			"pkg__ExternalID__c": storage.StringValue("default"),
		},
	}})
	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	if _, ok := org.Objects["pkg__PriceClass__c"]; !ok {
		t.Fatalf("synthetic object was not added: %#v", org.Objects)
	}
}

func TestInsertPersonAccountCreatesSyntheticPersonContact(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts", "StateAndCountryPicklist"})
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
	clearAddress := engine.Update([]storage.Record{{
		Object: "Account",
		ID:     insert[0].ID,
		ExplicitNulls: map[string]bool{
			"PersonMailingCountryCode": true,
			"PersonOtherStreet":        true,
		},
	}})
	if !clearAddress[0].Success {
		t.Fatalf("clear address update = %#v", clearAddress[0])
	}
	contact = org.Objects["Contact"].Records[contactID]
	if _, ok := contact.Fields["MailingCountryCode"]; ok {
		t.Fatalf("person contact mailing country code was not cleared: %#v", contact.Fields["MailingCountryCode"])
	}
	if _, ok := contact.Fields["OtherStreet"]; ok {
		t.Fatalf("person contact other street was not cleared: %#v", contact.Fields["OtherStreet"])
	}
}

func TestInsertPersonAccountRequiresLastName(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"RecordTypeId": storage.IDValue(personAccountRecordTypeID(t, org)),
			"LastName":     storage.NullValue(),
			"PersonEmail":  storage.StringValue("ada@example.invalid"),
		},
	}})

	if len(insert) != 1 || insert[0].Success || insert[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("insert result = %#v", insert)
	}
	if insert[0].Error != "Required fields are missing: [LastName]" {
		t.Fatalf("insert error = %q", insert[0].Error)
	}
}

func TestUpdatePersonAccountClearingLastNameFails(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"RecordTypeId": storage.IDValue(personAccountRecordTypeID(t, org)),
			"LastName":     storage.StringValue("Lovelace"),
			"PersonEmail":  storage.StringValue("ada@example.invalid"),
		},
	}})
	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	update := engine.Update([]storage.Record{{
		ID:            insert[0].ID,
		Object:        "Account",
		ExplicitNulls: map[string]bool{"LastName": true},
	}})
	if len(update) != 1 || update[0].Success || update[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("update result = %#v", update)
	}
	if update[0].Error != "Required fields are missing: [LastName]" {
		t.Fatalf("update error = %q", update[0].Error)
	}
}

func TestInsertPersonAccountWithoutRecordTypeUsesPersonRecordType(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"FirstName":   storage.StringValue("Eric"),
			"LastName":    storage.StringValue("Clapton"),
			"PersonEmail": storage.StringValue("eric@example.invalid"),
		},
	}})

	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if got, want := idFromStorageValue(stored.Fields["RecordTypeId"]), personAccountRecordTypeID(t, org); got != want {
		t.Fatalf("record type = %s, want %s; fields=%#v", got, want, stored.Fields)
	}
	if !stored.Fields["IsPersonAccount"].Boolean {
		t.Fatalf("person account flag was not set: %#v", stored.Fields)
	}
	if idFromStorageValue(stored.Fields["PersonContactId"]) == "" {
		t.Fatalf("person contact was not created: %#v", stored.Fields)
	}
}

func TestInsertOrganizationAccountInPersonAccountOrgRequiresName(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"OwnerId": storage.IDValue("005000000000001"),
		},
	}})

	if len(insert) != 1 || insert[0].Success || insert[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("insert result = %#v", insert)
	}
	if insert[0].Error != "Required fields are missing: [Name]" {
		t.Fatalf("insert error = %q", insert[0].Error)
	}
}

func TestInsertOrganizationAccountIgnoresFalseStringPersonDefaults(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{
		ID:            "012000000000ORG",
		DeveloperName: "Organization",
		Name:          "Organization",
		Active:        true,
		Available:     true,
	})
	org.Objects["Account"] = account
	storage.EnsureDeterministicPlatformData(&org)
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"RecordTypeId":             storage.IDValue("012000000000ORG"),
			"Name":                     storage.StringValue("Test Org"),
			"PersonDoNotCall":          storage.StringValue("false"),
			"PersonHasOptedOutOfEmail": storage.BooleanValue(false),
		},
	}})

	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if got := idFromStorageValue(stored.Fields["RecordTypeId"]); got != "012000000000ORG" {
		t.Fatalf("record type = %s; fields=%#v", got, stored.Fields)
	}
	if stored.Fields["IsPersonAccount"].Boolean {
		t.Fatalf("person account flag was set: %#v", stored.Fields)
	}
}

func TestInsertBusinessAccountPrefersBusinessRecordTypeWithoutPersonFields(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{
		ID:            "012000000000ORG",
		DeveloperName: "Organization",
		Name:          "Organization",
		Active:        true,
		Available:     true,
	})
	org.Objects["Account"] = account
	storage.EnsureDeterministicPlatformData(&org)
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Test Org"),
		},
	}})

	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	recordType := recordTypeByID(org.Objects["Account"].Definition.RecordTypes, idFromStorageValue(stored.Fields["RecordTypeId"]))
	if recordTypeLooksPersonAccount(recordType) {
		t.Fatalf("record type = %#v; fields=%#v", recordType, stored.Fields)
	}
	if stored.Fields["IsPersonAccount"].Boolean {
		t.Fatalf("person account flag was set: %#v", stored.Fields)
	}
}

func recordTypeByID(recordTypes []storage.RecordTypeInfo, id storage.ID) storage.RecordTypeInfo {
	for _, recordType := range recordTypes {
		if recordType.ID == id {
			return recordType
		}
	}
	return storage.RecordTypeInfo{ID: id}
}

func TestInsertOrganizationAccountRecordTypeWithPersonNameFieldsUsesPersonAccount(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{
		ID:            "012000000000ORG",
		DeveloperName: "Organization",
		Name:          "Organization",
		Active:        true,
		Available:     true,
	})
	org.Objects["Account"] = account
	storage.EnsureDeterministicPlatformData(&org)
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"RecordTypeId": storage.IDValue("012000000000ORG"),
			"Name":         storage.StringValue("Test Org"),
			"FirstName":    storage.StringValue("Eric"),
			"LastName":     storage.StringValue("Clapton"),
			"PersonEmail":  storage.StringValue("eric@example.invalid"),
		},
	}})

	if len(insert) != 1 || !insert[0].Success {
		t.Fatalf("insert result = %#v", insert)
	}
	stored := org.Objects["Account"].Records[insert[0].ID]
	if got := stored.Fields["Name"].String; got != "Eric Clapton" {
		t.Fatalf("account name = %q", got)
	}
	if got, want := idFromStorageValue(stored.Fields["RecordTypeId"]), personAccountRecordTypeID(t, org); got != want {
		t.Fatalf("record type = %s, want %s; fields=%#v", got, want, stored.Fields)
	}
	if !stored.Fields["IsPersonAccount"].Boolean {
		t.Fatalf("person account flag was not set: %#v", stored.Fields)
	}
	if idFromStorageValue(stored.Fields["PersonContactId"]) == "" {
		t.Fatalf("person contact was not created: %#v", stored.Fields)
	}
}

func personAccountRecordTypeID(t *testing.T, org storage.OrgState) storage.ID {
	t.Helper()
	for _, recordType := range org.Objects["Account"].Definition.RecordTypes {
		if recordType.DeveloperName == "PersonAccount" {
			return recordType.ID
		}
	}
	t.Fatalf("missing PersonAccount record type: %#v", org.Objects["Account"].Definition.RecordTypes)
	return ""
}

func TestInsertPersonAccountDoesNotPopulateCustomPersonContactAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["pkg__PersonContact__c"] = storage.Field{
		APIName:          "pkg__PersonContact__c",
		Type:             storage.FieldReference,
		ReferenceTo:      []string{"Contact"},
		RelationshipName: "pkg__PersonContact__r",
	}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Ada"),
			"LastName":  storage.StringValue("Lovelace"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	accountRecord := org.Objects["Account"].Records[insert[0].ID]
	contactID := accountRecord.Fields["PersonContactId"].ID
	if contactID == "" {
		t.Fatalf("PersonContactId was not populated: %#v", accountRecord.Fields)
	}
	if got := accountRecord.Fields["pkg__PersonContact__c"]; got.Kind != "" {
		t.Fatalf("pkg__PersonContact__c = %#v, want unset; fields=%#v", got, accountRecord.Fields)
	}
}

func TestInsertPersonAccountDoesNotMirrorNamespacedPersonEmailAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["pkg__PersonEmail__c"] = storage.Field{APIName: "pkg__PersonEmail__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"FirstName":   storage.StringValue("Ada"),
			"LastName":    storage.StringValue("Lovelace"),
			"PersonEmail": storage.StringValue("ada@example.invalid"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	accountRecord := org.Objects["Account"].Records[insert[0].ID]
	if got := accountRecord.Fields["pkg__PersonEmail__c"]; got.Kind != "" {
		t.Fatalf("pkg__PersonEmail__c = %#v, want unset; fields=%#v", got, accountRecord.Fields)
	}
}

func TestInsertEmailMessageCreatesToRelations(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "EmailMessage")
	storage.EnsureStandardObject(&org, "EmailMessageRelation")
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "EmailMessage",
		Fields: map[string]storage.Value{
			"Subject":         storage.StringValue("Test Email"),
			"FromAddress":     storage.StringValue("sender@example.invalid"),
			"ToAddress":       storage.StringValue("system@example.invalid"),
			"ToIds":           storage.ListValue(storage.IDValue("005000000000001AAA")),
			"Incoming":        storage.BooleanValue(true),
			"Status":          storage.StringValue("3"),
			"IsClientManaged": storage.BooleanValue(true),
			"MessageDate":     storage.DateTimeValue("2026-05-02T12:00:00Z"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	if got := len(org.Objects["EmailMessage"].Records); got != 1 {
		t.Fatalf("email messages = %d, want 1", got)
	}
	relations := org.Objects["EmailMessageRelation"].Records
	if got := len(relations); got != 1 {
		t.Fatalf("email message relations = %d, want 1", got)
	}
	for _, relation := range relations {
		if got := relation.Fields["EmailMessageId"].ID; got != insert[0].ID {
			t.Fatalf("EmailMessageId = %q, want %q", got, insert[0].ID)
		}
		if got := relation.Fields["RelationId"].ID; got != "005000000000001AAA" {
			t.Fatalf("RelationId = %q", got)
		}
		if got := relation.Fields["RelationType"].String; got != "ToAddress" {
			t.Fatalf("RelationType = %q", got)
		}
	}
}

func TestInsertContactPopulatesCompoundName(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Query"),
			"LastName":  storage.StringValue("Factory"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	contact := org.Objects["Contact"].Records[insert[0].ID]
	if got := contact.Fields["Name"].String; got != "Query Factory" {
		t.Fatalf("contact name = %q", got)
	}
}

func TestInsertPersonAccountRollbackRemovesAccountWhenSyntheticContactFails(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	contact := org.Objects["Contact"]
	contact.Definition.ValidationRules = []storage.ValidationRule{{
		Active:                true,
		ErrorConditionFormula: `LastName = "Blocked"`,
		ErrorMessage:          "blocked contact",
	}}
	org.Objects["Contact"] = contact
	beforeAccounts := len(org.Objects["Account"].Records)
	beforeContacts := len(org.Objects["Contact"].Records)
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Ada"),
			"LastName":  storage.StringValue("Blocked"),
		},
	}})
	if len(insert) != 1 || insert[0].Success {
		t.Fatalf("insert = %#v, want failed synthetic contact", insert)
	}
	if got := len(org.Objects["Account"].Records); got != beforeAccounts {
		t.Fatalf("accounts after rollback = %d, want %d", got, beforeAccounts)
	}
	if got := len(org.Objects["Contact"].Records); got != beforeContacts {
		t.Fatalf("contacts after rollback = %d, want %d", got, beforeContacts)
	}
}

func TestInsertAccountWithDefaultPersonFlagDoesNotCreatePersonContact(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	account := org.Objects["Account"]
	field := account.Definition.Fields["IsPersonAccount"]
	field.DefaultValue = "true"
	account.Definition.Fields["IsPersonAccount"] = field
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Business Account"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	if got := len(org.Objects["Contact"].Records); got != 0 {
		t.Fatalf("synthetic contacts = %d", got)
	}
}

func TestInsertEvaluatesDateFormulaDefaultsWithOrgClock(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC) }
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate, DefaultValue: "TODAY()"}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	record := org.Objects["Account"].Records[insert[0].ID]
	if got := record.Fields["RenewalDate__c"]; got.Kind != storage.ValueDate || got.String != "2026-05-15" {
		t.Fatalf("RenewalDate__c = %#v", got)
	}
}

func TestInsertEvaluatesTextFormulaDefaultsWithOrgClock(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC) }
	account := org.Objects["Account"]
	account.Definition.Fields["DefaultedLabel__c"] = storage.Field{APIName: "DefaultedLabel__c", Type: storage.FieldString, DefaultValue: "TEXT(TODAY())"}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	record := org.Objects["Account"].Records[insert[0].ID]
	if got := record.Fields["DefaultedLabel__c"]; got.Kind != storage.ValueString || got.String != "2026-05-15" {
		t.Fatalf("DefaultedLabel__c = %#v", got)
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
		ErrorMessage:          `blocked by "validation" rule`,
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

func TestDMLRejectsNonCreateableAndNonUpdateableFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["CreatedDate"] = storage.Field{APIName: "CreatedDate", Type: storage.FieldDateTime, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	account.Definition.Fields["External_Code__c"] = storage.Field{APIName: "External_Code__c", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Acme"),
			"CreatedDate": storage.StringValue("2026-05-14T00:00:00Z"),
		},
	}})
	assertDMLErrorDetail(t, insert[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "CreatedDate")

	created := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Acme"),
			"External_Code__c": storage.StringValue("original"),
		},
	}})
	if !created[0].Success {
		t.Fatalf("insert = %#v", created)
	}

	update := engine.Update([]storage.Record{{
		ID:     created[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"External_Code__c": storage.StringValue("changed"),
		},
	}})
	assertDMLErrorDetail(t, update[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "External_Code__c")
}

func TestDMLAllowsCampaignMemberStatusCampaignIDOnInsert(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "Campaign")
	storage.EnsureStandardObject(&org, "CampaignMemberStatus")
	engine := NewEngine(&org)
	campaign := engine.Insert([]storage.Record{{
		Object: "Campaign",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Spring Appeal"),
		},
	}})
	if !campaign[0].Success {
		t.Fatalf("campaign insert = %#v", campaign)
	}

	insert := engine.Insert([]storage.Record{{
		Object: "CampaignMemberStatus",
		Fields: map[string]storage.Value{
			"CampaignId":   storage.IDValue(campaign[0].ID),
			"Label":        storage.StringValue("Sent"),
			"SortOrder":    storage.IntegerValue(1),
			"IsDefault":    storage.BooleanValue(false),
			"HasResponded": storage.BooleanValue(false),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
}

func TestDMLAllowsLocalCreateRelationshipIdentityFieldsForJunctionObjects(t *testing.T) {
	org := testOrg()
	org.Objects["Junction__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Junction__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"ParentId":      {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent", Required: true, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
				"OtherParentId": {APIName: "OtherParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "OtherParent", Required: true, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent)
	}
	junction := engine.Insert([]storage.Record{{
		Object: "Junction__c",
		Fields: map[string]storage.Value{
			"Name":          storage.StringValue("Child"),
			"ParentId":      storage.IDValue(parent[0].ID),
			"OtherParentId": storage.IDValue(parent[0].ID),
		},
	}})
	if !junction[0].Success {
		t.Fatalf("junction insert = %#v", junction)
	}
}

func TestDMLStripsUnchangedNonUpdateableFieldsBeforeUpdateValidation(t *testing.T) {
	org := testOrg()
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString, Updateable: storage.BoolFlag(true)},
				"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent", Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}})
	otherParent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Other")}}})
	child := engine.Insert([]storage.Record{{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Child"),
			"ParentId": storage.IDValue(parent[0].ID),
		},
	}})
	if !parent[0].Success || !otherParent[0].Success || !child[0].Success {
		t.Fatalf("setup inserts parent=%#v other=%#v child=%#v", parent, otherParent, child)
	}

	unchanged := engine.Update([]storage.Record{{
		ID:     child[0].ID,
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Updated"),
			"ParentId": storage.IDValue(parent[0].ID),
		},
	}})
	if !unchanged[0].Success {
		t.Fatalf("unchanged parent update = %#v", unchanged)
	}

	org.Objects["Child__c"].Definition.Fields["ReadonlyText__c"] = storage.Field{
		APIName:    "ReadonlyText__c",
		Type:       storage.FieldString,
		Updateable: storage.BoolFlag(false),
	}
	org.Objects["Child__c"].Definition.Fields["FormulaText__c"] = storage.Field{
		APIName: "FormulaText__c",
		Type:    storage.FieldString,
		Formula: `"selected"`,
	}
	nullAbsent := engine.Update([]storage.Record{{
		ID:     child[0].ID,
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue("Updated Again"),
			"ReadonlyText__c": storage.NullValue(),
		},
		ExplicitNulls: map[string]bool{"FormulaText__c": true},
	}})
	if !nullAbsent[0].Success {
		t.Fatalf("null absent readonly update = %#v", nullAbsent)
	}

	changed := engine.Update([]storage.Record{{
		ID:     child[0].ID,
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(otherParent[0].ID),
		},
	}})
	assertDMLErrorDetail(t, changed[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "ParentId")
}

func TestDMLStillRejectsNonCreateableReadonlyRelationshipAndTypeFields(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "CampaignMember")
	engine := NewEngine(&org)

	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Acme"),
			"MasterRecordId": storage.IDValue("001000000000001AAA"),
		},
	}})
	assertDMLErrorDetail(t, account[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "MasterRecordId")

	member := engine.Insert([]storage.Record{{
		Object: "CampaignMember",
		Fields: map[string]storage.Value{
			"Type": storage.StringValue("Default"),
		},
	}})
	assertDMLErrorDetail(t, member[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "Type")

	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent", Required: true, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	child := engine.Insert([]storage.Record{{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Child"),
			"ParentId": storage.IDValue("001000000000001AAA"),
		},
	}})
	assertDMLErrorDetail(t, child[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "ParentId")
}

func TestDMLAllowsLocalLeadCommunicationFieldsForSecurityHarnesses(t *testing.T) {
	org := testOrg()
	storage.EnsureStandardObject(&org, "Lead")
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Lead",
		Fields: map[string]storage.Value{
			"LastName":           storage.StringValue("Test"),
			"Company":            storage.StringValue("Test"),
			"DoNotCall":          storage.BooleanValue(true),
			"HasOptedOutOfEmail": storage.BooleanValue(true),
			"HasOptedOutOfFax":   storage.BooleanValue(true),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}

	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Lead",
		Fields: map[string]storage.Value{
			"DoNotCall": storage.BooleanValue(false),
		},
	}})
	if !update[0].Success {
		t.Fatalf("update = %#v", update[0])
	}

	missing := engine.Insert([]storage.Record{{
		Object: "Lead",
		Fields: map[string]storage.Value{
			"LastName": storage.StringValue(""),
		},
	}})
	if missing[0].Success || missing[0].StatusCode != "REQUIRED_FIELD_MISSING" || missing[0].Error != "Required fields are missing: [LastName, Company]" {
		t.Fatalf("missing required = %#v", missing[0])
	}
}

func TestDMLAppliesLocalUserRequiredDefaults(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "User")
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "User",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Local"),
			"LastName":  storage.StringValue("Provider"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	user := org.Objects["User"].Records[insert[0].ID]
	for _, fieldName := range []string{"Alias", "Email", "EmailEncodingKey", "IsActive", "LanguageLocaleKey", "LocaleSidKey", "ProfileId", "TimeZoneSidKey", "Username"} {
		if value, ok := user.Fields[fieldName]; !ok || value.Kind == "" {
			t.Fatalf("User.%s default = %#v, %v", fieldName, value, ok)
		}
	}
	if !user.Fields["IsActive"].Boolean {
		t.Fatalf("IsActive = %#v", user.Fields["IsActive"])
	}
	if got := user.Fields["ProfileId"]; got.Kind != storage.ValueID || got.ID != "00e000000000001" {
		t.Fatalf("ProfileId = %#v", got)
	}
	foundLogin := false
	for _, login := range org.Objects["UserLogin"].Records {
		if login.Fields["UserId"].ID == insert[0].ID && !login.Fields["IsFrozen"].Boolean {
			foundLogin = true
		}
	}
	if !foundLogin {
		t.Fatalf("missing UserLogin for %s: %#v", insert[0].ID, org.Objects["UserLogin"].Records)
	}
}

func TestDMLDefaultsUserAccountFromContact(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "Account")
	accountState := org.Objects["Account"]
	accountState.Records["001000000000777"] = storage.Record{
		ID:     "001000000000777",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Local Account"),
		},
	}
	org.Objects["Account"] = accountState
	contactState := org.Objects["Contact"]
	contactState.Records["003000000000777"] = storage.Record{
		ID:     "003000000000777",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Member"),
			"AccountId": storage.IDValue("001000000000777"),
		},
	}
	org.Objects["Contact"] = contactState
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "User",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Local"),
			"LastName":  storage.StringValue("Member"),
			"ContactId": storage.IDValue("003000000000777"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert[0])
	}
	user := org.Objects["User"].Records[insert[0].ID]
	if got := user.Fields["AccountId"]; got.Kind != storage.ValueID || got.ID != "001000000000777" {
		t.Fatalf("User.AccountId = %#v", got)
	}
}

func TestInsertReplacesGeneratedPlaceholderID(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "User")
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		ID:     "#a#b#c#d#e#f#",
		Object: "User",
		Fields: map[string]storage.Value{
			"LastName": storage.StringValue("Local"),
		},
	}})
	if !insert[0].Success || !strings.HasPrefix(string(insert[0].ID), "005") {
		t.Fatalf("insert = %#v", insert[0])
	}
}

func TestDMLDoesNotStripAbsentExplicitNullForNonUpdateableField(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Readonly__c"] = storage.Field{APIName: "Readonly__c", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
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
		ExplicitNulls: map[string]bool{"Readonly__c": true},
	}})
	assertDMLErrorDetail(t, update[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "Readonly__c")
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

	insert := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue("Acme"),
			"External_Key__c": storage.StringValue("ext-1"),
			"Code__c":         storage.StringValue("A"),
		},
	}}, "External_Key__c")
	if !insert[0].Success || !insert[0].Created {
		t.Fatalf("external insert = %#v", insert)
	}
	update := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"External_Key__c": storage.StringValue("EXT-1"),
			"Name":            storage.StringValue("Changed"),
		},
	}}, "External_Key__c")
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

func TestUpsertWithMissingExplicitIDReturnsInvalidCrossReference(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)

	results := engine.Upsert([]storage.Record{{
		ID:     "001999999999999AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Missing"),
		},
	}})
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1: %#v", len(results), results)
	}
	if results[0].Success || results[0].StatusCode != "INVALID_CROSS_REFERENCE_KEY" {
		t.Fatalf("upsert result = %#v", results[0])
	}
	if !strings.Contains(strings.ToLower(results[0].Error), "invalid cross reference id") {
		t.Fatalf("upsert error = %q", results[0].Error)
	}
}

func TestUpsertStripsRelationshipPseudoFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Parent__c",
				ParentObjects:      []string{"Parent__c"},
				ParentRelationship: "Parent__r",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	results := engine.Upsert([]storage.Record{{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Line"),
			"Parent__r": storage.NullValue(),
		},
	}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("upsert result = %#v", results)
	}
	stored := org.Objects["Child__c"].Records[results[0].ID]
	if _, ok := stored.Fields["Parent__r"]; ok {
		t.Fatalf("stored relationship pseudo-field: %#v", stored.Fields)
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

func TestUpsertWithExplicitIDFieldUsesRecordID(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	update := engine.UpsertWithExternalID([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Changed"),
		},
	}}, "Id")
	if !update[0].Success || update[0].Created || update[0].ID != insert[0].ID {
		t.Fatalf("explicit id update = %#v", update)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Changed" {
		t.Fatalf("updated name = %q", got)
	}

	updateFromField := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Id":   storage.IDValue(insert[0].ID),
			"Name": storage.StringValue("Changed Again"),
		},
	}}, "Id")
	if !updateFromField[0].Success || updateFromField[0].Created || updateFromField[0].ID != insert[0].ID {
		t.Fatalf("explicit id field update = %#v", updateFromField)
	}
	if got := org.Objects["Account"].Records[insert[0].ID].Fields["Name"].String; got != "Changed Again" {
		t.Fatalf("updated name from field = %q", got)
	}
}

func TestUpsertListCustomSettingWithNameExternalID(t *testing.T) {
	org := testOrg()
	org.Objects["Page__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Page__c",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Title__c": {APIName: "Title__c", Type: storage.FieldString},
			},
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Page__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Checkout"),
			"Title__c": storage.StringValue("Old"),
		},
	}}, "Name")
	if !insert[0].Success || !insert[0].Created {
		t.Fatalf("custom setting insert = %#v", insert)
	}
	update := engine.UpsertWithExternalID([]storage.Record{{
		Object: "Page__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Checkout"),
			"Title__c": storage.StringValue("New"),
		},
	}}, "Name")
	if !update[0].Success || update[0].Created || update[0].ID != insert[0].ID {
		t.Fatalf("custom setting update = %#v", update)
	}
	if got := org.Objects["Page__c"].Records[insert[0].ID].Fields["Title__c"].String; got != "New" {
		t.Fatalf("updated title = %q", got)
	}
}

func TestInsertListCustomSettingRequiresName(t *testing.T) {
	org := testOrg()
	org.Objects["Page__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Page__c",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Title__c": {APIName: "Title__c", Type: storage.FieldString},
			},
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Page__c",
		Fields: map[string]storage.Value{
			"Title__c": storage.StringValue("Missing name"),
		},
	}})
	if insert[0].Success || insert[0].StatusCode != "REQUIRED_FIELD_MISSING" || len(insert[0].Fields) != 1 || insert[0].Fields[0] != "Name" {
		t.Fatalf("missing list custom setting name = %#v", insert[0])
	}
}

func TestInsertListCustomSettingRequiresNameWithoutNameFieldOverlay(t *testing.T) {
	org := testOrg()
	org.Objects["Page__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Page__c",
			Fields: map[string]storage.Field{
				"Title__c": {APIName: "Title__c", Type: storage.FieldString},
			},
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Page__c",
		Fields: map[string]storage.Value{
			"Title__c": storage.StringValue("Missing name"),
		},
	}})
	if insert[0].Success || insert[0].StatusCode != "REQUIRED_FIELD_MISSING" || len(insert[0].Fields) != 1 || insert[0].Fields[0] != "Name" {
		t.Fatalf("missing list custom setting name without overlay = %#v", insert[0])
	}
}

func TestInsertListCustomSettingRequiresNameWithTypeMetadataOnly(t *testing.T) {
	org := testOrg()
	org.Objects["Page__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Page__c",
			Fields: map[string]storage.Field{
				"Title__c": {APIName: "Title__c", Type: storage.FieldString},
			},
			Metadata: map[string]string{"customSettingsType": "List"},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Page__c",
		Fields: map[string]storage.Value{
			"Title__c": storage.StringValue("Missing name"),
		},
	}})
	if insert[0].Success || insert[0].StatusCode != "REQUIRED_FIELD_MISSING" || len(insert[0].Fields) != 1 || insert[0].Fields[0] != "Name" {
		t.Fatalf("missing list custom setting name with type metadata only = %#v", insert[0])
	}
}

func TestReferenceValidationRestrictedDeleteAndUndelete(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
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
	placeholderParent := engine.Insert([]storage.Record{{
		Object: "Contact",
		Fields: map[string]storage.Value{
			"LastName":  storage.StringValue("Placeholder"),
			"AccountId": storage.IDValue("#local-placeholder#"),
		},
	}})
	if !placeholderParent[0].Success {
		t.Fatalf("placeholder parent = %#v", placeholderParent)
	}
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Owner__c": {APIName: "Owner__c", Type: storage.FieldReference, ReferenceTo: []string{"User"}},
				"Who__c":   {APIName: "Who__c", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine.IDs.Prefixes["Widget__c"] = "a00"
	userBacked := engine.Insert([]storage.Record{{
		Object: "Widget__c",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Widget"),
			"Owner__c": storage.IDValue("005999999999999"),
			"Who__c":   storage.IDValue("003999999999999"),
		},
	}})
	if !userBacked[0].Success {
		t.Fatalf("user-backed missing parent = %#v", userBacked)
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

func TestReferenceValidationMatchesFifteenAndEighteenCharacterIDs(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
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
			"AccountId": storage.IDValue(account[0].ID + "AAA"),
		},
	}})
	if !contact[0].Success {
		t.Fatalf("contact = %#v", contact)
	}
}

func TestValidationRules(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "BlockBadName",
		Active:                true,
		ErrorConditionFormula: `Name = "Blocked"`,
		ErrorMessage:          `blocked by "validation" rule`,
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
	if blockedUpdate[0].Success || blockedUpdate[0].Error != "blocked by &quot;validation&quot; rule" {
		t.Fatalf("blocked update = %#v", blockedUpdate)
	}
}

func TestValidationRulePriorValueOnInsertUsesCurrentValue(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "ReadOnlyAfterCreate",
		Active:                true,
		ErrorConditionFormula: `PRIORVALUE(Name) != Name`,
		ErrorMessage:          "rating changed",
		ErrorDisplayField:     "Name",
	}}
	org.Objects["Account"] = account
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
	update := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Other"),
		},
	}})
	if update[0].Success || update[0].Error != "rating changed" {
		t.Fatalf("update = %#v", update)
	}
}

func TestValidationRulesUseFormulaScaleForPercentFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Discount__c"] = storage.Field{APIName: "Discount__c", Type: storage.FieldDecimal, DisplayType: "PERCENT"}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "DiscountUnder100",
		Active:                true,
		ErrorConditionFormula: `Discount__c > 1`,
		ErrorMessage:          "discount too large",
		ErrorDisplayField:     "Discount__c",
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Allowed"),
			"Discount__c": storage.DecimalValue("50"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	blocked := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Blocked"),
			"Discount__c": storage.DecimalValue("150"),
		},
	}})
	if blocked[0].Success || blocked[0].Error != "discount too large" {
		t.Fatalf("blocked insert = %#v", blocked)
	}
}

func TestValidationRuleRelationalNullIsFalse(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "PositiveAmount",
		Active:                true,
		ErrorConditionFormula: `Amount__c < 0`,
		ErrorMessage:          "amount must be positive",
		ErrorDisplayField:     "Amount__c",
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Allowed")},
	}})
	if !insert[0].Success {
		t.Fatalf("insert with null amount = %#v", insert)
	}
	blocked := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Blocked"),
			"Amount__c": storage.DecimalValue("-1"),
		},
	}})
	if blocked[0].Success || blocked[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" {
		t.Fatalf("negative amount insert = %#v", blocked)
	}
}

func TestValidationRulesResolveRecordTypeName(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{ID: "012000000000001AAA", Name: "Business", DeveloperName: "Business"},
		{ID: "012000000000002AAA", Name: "Consumer", DeveloperName: "Consumer"},
	}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "BusinessRequiresCode",
		Active:                true,
		ErrorConditionFormula: `ISBLANK(Code__c) && $RecordType.Name != 'Consumer'`,
		ErrorMessage:          "code required",
		ErrorDisplayField:     "Code__c",
	}}
	org.Objects["Account"] = account
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000001AAA": {Object: "RecordType", ID: "012000000000001AAA"},
			"012000000000002AAA": {Object: "RecordType", ID: "012000000000002AAA"},
		},
	}
	engine := NewEngine(&org)

	allowed := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Allowed"),
			"RecordTypeId": storage.IDValue("012000000000002AAA"),
		},
	}})
	if !allowed[0].Success {
		t.Fatalf("allowed insert = %#v", allowed)
	}
	blocked := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Blocked"),
			"RecordTypeId": storage.IDValue("012000000000001AAA"),
		},
	}})
	if blocked[0].Success || blocked[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" {
		t.Fatalf("blocked insert = %#v", blocked)
	}
}

func TestValidationRuleISNULLDoesNotTreatBlankLookupAsNull(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Parent__c"] = storage.Field{APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "LegacyLookupIsNull",
		Active:                true,
		ErrorConditionFormula: `ISNULL(Parent__c)`,
		ErrorMessage:          "parent missing",
	}, {
		Name:                  "LookupIsBlank",
		Active:                true,
		ErrorConditionFormula: `ISBLANK(Parent__c)`,
		ErrorMessage:          "parent blank",
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	blocked := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Child")},
	}})
	if blocked[0].Success || blocked[0].Error != "parent blank" {
		t.Fatalf("blank lookup insert = %#v", blocked)
	}

	account.Definition.ValidationRules = account.Definition.ValidationRules[:1]
	org.Objects["Account"] = account
	allowed := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Legacy Allowed")},
	}})
	if !allowed[0].Success {
		t.Fatalf("ISNULL lookup insert = %#v", allowed)
	}
}

func TestValidationRulesResolveParentRelationshipFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["GLAccount__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "GLAccount__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Entity__c": {APIName: "Entity__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["BankAccount__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "BankAccount__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"Entity__c":     {APIName: "Entity__c", Type: storage.FieldString},
				"GLAccount__c":  {APIName: "GLAccount__c", Type: storage.FieldReference, ReferenceTo: []string{"GLAccount__c"}},
				"OtherGL__c":    {APIName: "OtherGL__c", Type: storage.FieldReference, ReferenceTo: []string{"GLAccount__c"}, RelationshipName: "OtherGL"},
				"LegacyOwnerId": {APIName: "LegacyOwnerId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "LegacyOwner"},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "BankAccountGLAccountEntityMustMatch",
				Active:                true,
				ErrorConditionFormula: `AND(IsBlank(Entity__c)=False, IsBlank(GLAccount__c)=False, Entity__c <> GLAccount__r.Entity__c)`,
				ErrorMessage:          "entity mismatch",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	parent := engine.Insert([]storage.Record{{
		Object: "GLAccount__c",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Cash"),
			"Entity__c": storage.StringValue("Entity A"),
		},
	}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent)
	}

	matching := engine.Insert([]storage.Record{{
		Object: "BankAccount__c",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Bank"),
			"Entity__c":    storage.StringValue("Entity A"),
			"GLAccount__c": storage.IDValue(parent[0].ID),
		},
	}})
	if !matching[0].Success {
		t.Fatalf("matching insert = %#v", matching)
	}

	mismatched := engine.Insert([]storage.Record{{
		Object: "BankAccount__c",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Other Bank"),
			"Entity__c":    storage.StringValue("Entity B"),
			"GLAccount__c": storage.IDValue(parent[0].ID),
		},
	}})
	if mismatched[0].Success || mismatched[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" || mismatched[0].Error != "entity mismatch" {
		t.Fatalf("mismatched insert = %#v", mismatched)
	}
}

func TestValidationRuleNullInequalityDoesNotMatch(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":   {APIName: "Name", Type: storage.FieldString},
				"Score":  {APIName: "Score", Type: storage.FieldDecimal},
				"Target": {APIName: "Target", Type: storage.FieldDecimal},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "TargetDiffers",
				Active:                true,
				ErrorConditionFormula: "Score <> Target",
				ErrorMessage:          "target differs",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	missingTarget := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":  storage.StringValue("Acme"),
			"Score": storage.DecimalValue("12"),
		},
	}})
	if !missingTarget[0].Success {
		t.Fatalf("missing target insert = %#v", missingTarget)
	}

	differentTarget := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":   storage.StringValue("Other"),
			"Score":  storage.DecimalValue("12"),
			"Target": storage.DecimalValue("13"),
		},
	}})
	if differentTarget[0].Success || differentTarget[0].Error != "target differs" {
		t.Fatalf("different target insert = %#v", differentTarget)
	}
}

func TestValidationRulesSupportIsChangedWithParentRelationshipFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["DeferredSchedule__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "DeferredSchedule__c",
			KeyPrefix: "a14",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["OrderItemLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OrderItemLine__c",
			KeyPrefix: "a15",
			Fields: map[string]storage.Field{
				"Name":                {APIName: "Name", Type: storage.FieldString},
				"DeferredSchedule__c": {APIName: "DeferredSchedule__c", Type: storage.FieldReference, ReferenceTo: []string{"DeferredSchedule__c"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Subscription__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Subscription__c",
			KeyPrefix: "a16",
			Fields: map[string]storage.Field{
				"Name":               {APIName: "Name", Type: storage.FieldString},
				"StartDate__c":       {APIName: "StartDate__c", Type: storage.FieldDate},
				"EndDate__c":         {APIName: "EndDate__c", Type: storage.FieldDate},
				"EndDateOverride__c": {APIName: "EndDateOverride__c", Type: storage.FieldDate},
				"OrderItemLine__c":   {APIName: "OrderItemLine__c", Type: storage.FieldReference, ReferenceTo: []string{"OrderItemLine__c"}},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:   "TermUnchangeableForDeferredMembership",
				Active: true,
				ErrorConditionFormula: `AND(
OR(
ISCHANGED(StartDate__c),
AND(ISCHANGED(EndDate__c), !ISCHANGED(EndDateOverride__c))
),
IsBlank(OrderItemLine__c) = False,
IsBlank(OrderItemLine__r.DeferredSchedule__c) = False
)`,
				ErrorMessage: "deferred membership term cannot change",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	schedule := engine.Insert([]storage.Record{{Object: "DeferredSchedule__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Schedule")}}})
	if !schedule[0].Success {
		t.Fatalf("schedule insert = %#v", schedule)
	}
	line := engine.Insert([]storage.Record{{
		Object: "OrderItemLine__c",
		Fields: map[string]storage.Value{
			"Name":                storage.StringValue("Line"),
			"DeferredSchedule__c": storage.IDValue(schedule[0].ID),
		},
	}})
	if !line[0].Success {
		t.Fatalf("line insert = %#v", line)
	}
	membership := engine.Insert([]storage.Record{{
		Object: "Subscription__c",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Subscription"),
			"StartDate__c":     storage.DateValue("2026-01-01"),
			"EndDate__c":       storage.DateValue("2026-12-31"),
			"OrderItemLine__c": storage.IDValue(line[0].ID),
		},
	}})
	if !membership[0].Success {
		t.Fatalf("membership insert = %#v", membership)
	}

	update := engine.Update([]storage.Record{{
		ID:     membership[0].ID,
		Object: "Subscription__c",
		Fields: map[string]storage.Value{
			"EndDate__c": storage.DateValue("2026-02-28"),
		},
	}})
	if update[0].Success || update[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" || update[0].Error != "deferred membership term cannot change" {
		t.Fatalf("membership term update = %#v", update)
	}
}

func TestValidationRulesIsChangedIgnoresStatusTransitionSideEffects(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a17",
			Fields: map[string]storage.Field{
				"Name":        {APIName: "Name", Type: storage.FieldString},
				"Status__c":   {APIName: "Status__c", Type: storage.FieldString},
				"PostedOn__c": {APIName: "PostedOn__c", Type: storage.FieldDate},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "Cannot_Edit_Posted_Or_Exported",
				Active:                true,
				ErrorConditionFormula: `AND(!ISChanged(Status__c), OR(Text(Status__c) = 'Posted', Text(Status__c) = 'Exported'))`,
				ErrorMessage:          "Batches with a status of Posted or Exported cannot be edited.",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Batch"),
			"Status__c": storage.StringValue("Open"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}

	post := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"Status__c":   storage.StringValue("Posted"),
			"PostedOn__c": storage.DateValue("2026-05-20"),
		},
	}})
	if !post[0].Success {
		t.Fatalf("post transition = %#v", post)
	}

	editPosted := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"PostedOn__c": storage.DateValue("2026-05-21"),
		},
	}})
	if editPosted[0].Success || editPosted[0].Error != "Batches with a status of Posted or Exported cannot be edited." {
		t.Fatalf("posted edit = %#v", editPosted)
	}
}

func TestWorkflowFieldUpdateKeepsValidationIsChangedContext(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a17",
			Fields: map[string]storage.Field{
				"Name":        {APIName: "Name", Type: storage.FieldString},
				"Status__c":   {APIName: "Status__c", Type: storage.FieldString},
				"PostedOn__c": {APIName: "PostedOn__c", Type: storage.FieldDate},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "Cannot_Edit_Posted_Or_Exported",
				Active:                true,
				ErrorConditionFormula: `AND(!ISChanged(Status__c), OR(Text(Status__c) = 'Posted', Text(Status__c) = 'Exported'))`,
				ErrorMessage:          "Batches with a status of Posted or Exported cannot be edited.",
			}},
			WorkflowRules: []storage.WorkflowRule{{
				Name:   "Batch Posted On Population",
				Active: true,
				Criteria: []storage.WorkflowCriteriaItem{{
					Field:     "Status__c",
					Operation: "equals",
					Value:     "Posted",
				}},
				FieldUpdates: []storage.WorkflowFieldUpdate{{
					Name:    "BatchPostedOnPopulation",
					Field:   "PostedOn__c",
					Formula: "Today()",
				}},
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	engine.Now = func() time.Time { return time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC) }

	insert := engine.Insert([]storage.Record{{
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Manual"),
			"Status__c": storage.StringValue("Open"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	post := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"Status__c": storage.StringValue("Posted"),
		},
	}})
	if !post[0].Success {
		t.Fatalf("post = %#v", post)
	}
	posted := org.Objects["Batch__c"].Records[insert[0].ID]
	if got := posted.Fields["PostedOn__c"].String; got != "2026-05-20" {
		t.Fatalf("posted on = %q", got)
	}

	edit := engine.Update([]storage.Record{{
		ID:     insert[0].ID,
		Object: "Batch__c",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Edited"),
		},
	}})
	if edit[0].Success || edit[0].Error != "Batches with a status of Posted or Exported cannot be edited." {
		t.Fatalf("posted edit = %#v", edit)
	}
}

func TestValidationRulesResolveParentFormulaFields(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Payment__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Payment__c",
			KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"Name":                      {APIName: "Name", Type: storage.FieldString},
				"IsCredit__c":               {APIName: "IsCredit__c", Type: storage.FieldBoolean},
				"PaymentAmount__c":          {APIName: "PaymentAmount__c", Type: storage.FieldDecimal},
				"TotalPaymentApplied__c":    {APIName: "TotalPaymentApplied__c", Type: storage.FieldSummary, SummaryOperation: "sum", SummaryForeignKey: "PaymentLine__c.Payment__c", SummarizedField: "PaymentLine__c.PaymentAmount__c"},
				"AvailableCreditBalance__c": {APIName: "AvailableCreditBalance__c", Type: storage.FieldCalculated, Formula: "IF(IsCredit__c, PaymentAmount__c - TotalPaymentApplied__c, 0)"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["CartPayment__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "CartPayment__c",
			KeyPrefix: "a13",
			Fields: map[string]storage.Field{
				"Name":             {APIName: "Name", Type: storage.FieldString},
				"PaymentAmount__c": {APIName: "PaymentAmount__c", Type: storage.FieldDecimal},
				"CreditPayment__c": {APIName: "CreditPayment__c", Type: storage.FieldReference, ReferenceTo: []string{"Payment__c"}, RelationshipName: "CreditCartPayments"},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "PrepaymentAmountCannotExceedBalance",
				Active:                true,
				ErrorConditionFormula: "IsBlank(CreditPayment__c) = false && PaymentAmount__c > CreditPayment__r.AvailableCreditBalance__c",
				ErrorMessage:          "The prepayment amount cannot exceed the available credit balance on the prepayment.",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)

	credit := engine.Insert([]storage.Record{{
		Object: "Payment__c",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Credit"),
			"IsCredit__c":      storage.BooleanValue(true),
			"PaymentAmount__c": storage.DecimalValue("9"),
		},
	}})
	if !credit[0].Success {
		t.Fatalf("credit insert = %#v", credit)
	}

	applied := engine.Upsert([]storage.Record{{
		Object: "CartPayment__c",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Applied"),
			"PaymentAmount__c": storage.DecimalValue("10"),
			"CreditPayment__c": storage.IDValue(storage.ID(string(credit[0].ID) + "AAA")),
		},
	}})
	if applied[0].Success || applied[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" || applied[0].Error != "The prepayment amount cannot exceed the available credit balance on the prepayment." {
		t.Fatalf("over-applied credit insert = %#v", applied)
	}
}

func TestValidationRulesReadTransientParentRelationshipShells(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Entity__c": {APIName: "Entity__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Product__c",
				Fields: map[string]storage.Value{
					"Entity__c": storage.StringValue("Member"),
				},
			},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "CartItem__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Entity__c": {APIName: "Entity__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Line__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Product__c":  {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
				"CartItem__c": {APIName: "CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"CartItem__c"}},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "ProductEntityMatchesCartItem",
				Active:                true,
				ErrorConditionFormula: "ISBLANK(CartItem__r.Entity__c) || Product__r.Entity__c <> CartItem__r.Entity__c",
				ErrorMessage:          "The product entity must match the cart item's entity.",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}

	engine := NewEngine(&org)
	withoutShell := engine.Insert([]storage.Record{{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"CartItem__c": storage.IDValue("a02000000000001"),
		},
	}})
	if withoutShell[0].Success || withoutShell[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" {
		t.Fatalf("insert without relationship shell = %#v; want validation failure", withoutShell[0])
	}

	withShell := engine.Insert([]storage.Record{{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"CartItem__c": storage.IDValue("a02000000000001"),
		},
		ParentRelationships: map[string]storage.Record{
			"CartItem__r": {
				Object: "CartItem__c",
				Fields: map[string]storage.Value{
					"Entity__c": storage.StringValue("Member"),
				},
			},
		},
	}})
	if !withShell[0].Success {
		t.Fatalf("insert with relationship shell = %#v; want success", withShell[0])
	}
	stored := org.Objects["Line__c"].Records[withShell[0].ID]
	if len(stored.ParentRelationships) != 0 {
		t.Fatalf("stored record kept parent relationship shell: %#v", stored.ParentRelationships)
	}
}

func TestValidationFormulaComparesReferenceIDsByApexIdentity(t *testing.T) {
	parentDefinition := storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Entity__c": {APIName: "Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"Entity__c"}},
		},
	}
	childDefinition := storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"Entity__c": {APIName: "Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"Entity__c"}},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: parentDefinition,
			Records: map[storage.ID]storage.Record{
				"a01000000000001": {
					ID:     "a01000000000001",
					Object: "Parent__c",
					Fields: map[string]storage.Value{
						"Entity__c": storage.IDValue("a0E000000000001"),
					},
				},
			},
		},
		"Child__c": {Definition: childDefinition},
	}}

	matches, ok := evaluateValidationFormulaInOrg("Parent__r.Entity__c <> Entity__c", &org, childDefinition, storage.Record{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Parent__c": storage.IDValue("a01000000000001"),
			"Entity__c": storage.IDValue("a0E000000000001AAA"),
		},
	}, nil, true)
	if !ok || matches {
		t.Fatalf("15/18 character id comparison = %v, ok=%v; want false", matches, ok)
	}
}

func TestInsertAllowsTaskWhatIDToReferenceCustomObject(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Task")
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Invoice__c",
			KeyPrefix: "a0S",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0S000000000001": {
				ID:     "a0S000000000001",
				Object: "Invoice__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("INV-1"),
				},
			},
		},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Task",
		Fields: map[string]storage.Value{
			"Subject": storage.StringValue("Follow up"),
			"WhatId":  storage.IDValue("a0S000000000001"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
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
		{
			Name:                  "FloorMod",
			Active:                true,
			ErrorConditionFormula: `FLOOR(2.9) = 2 && MOD(5, 2) = 1 && Name = "Bad Math"`,
			ErrorMessage:          "bad math",
			ErrorDisplayField:     "Name",
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
	badMath := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Bad Math"),
		},
	}})
	if badMath[0].Success || badMath[0].Error != "bad math" || badMath[0].Fields[0] != "Name" {
		t.Fatalf("bad math = %#v", badMath)
	}
}

func TestCalculatedFormulaSupportsCaseAndLower(t *testing.T) {
	field := storage.Field{APIName: "Member__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `CASE(LOWER(MemberOverride__c), 'yes', 'Yes', 'no', 'No', IF(ISBLANK(LapsedOn__c) || LapsedOn__c <= TODAY(), 'No', 'Yes'))`}
	definition := storage.ObjectDefinition{
		APIName: "Account",
		Fields: map[string]storage.Field{
			"MemberOverride__c": {APIName: "MemberOverride__c", Type: storage.FieldString},
			"LapsedOn__c":       {APIName: "LapsedOn__c", Type: storage.FieldDate},
			"Member__c":         field,
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{"Account": {Definition: definition}}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, storage.Record{
		Object: "Account",
		Fields: map[string]storage.Value{
			"LapsedOn__c": storage.DateValue("2099-01-01"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Yes" {
		t.Fatalf("future lapsed formula = %#v, ok=%v; want Yes", value, ok)
	}

	value, _, ok = EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, storage.Record{
		Object: "Account",
		Fields: map[string]storage.Value{
			"MemberOverride__c": storage.StringValue("NO"),
			"LapsedOn__c":       storage.DateValue("2099-01-01"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "No" {
		t.Fatalf("override formula = %#v, ok=%v; want No", value, ok)
	}
}

func TestCalculatedTextFormulaStringifiesDecimalAmount(t *testing.T) {
	field := storage.Field{
		APIName:     "Combined_Text__c",
		Type:        storage.FieldCalculated,
		DisplayType: "STRING",
		Formula:     "TEXT(ServiceDate__c) + ';|;'+ TEXT(Amount__c) + ';|;'+ TEXT(Level__c) + ';|;'+ TEXT(Source__c) + ';|;' +  Id",
	}
	org := storage.NewOrgState()
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Invoice__c",
			Fields: map[string]storage.Field{
				field.APIName:    field,
				"ServiceDate__c": {APIName: "ServiceDate__c", Type: storage.FieldDate},
				"Amount__c":      {APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY"},
				"Level__c":       {APIName: "Level__c", Type: storage.FieldString},
				"Source__c":      {APIName: "Source__c", Type: storage.FieldString},
			},
		},
	}
	definition := org.Objects["Invoice__c"].Definition
	definition.Fields[field.APIName] = field
	record := storage.Record{
		ID:     "a00000000000001",
		Object: "Invoice__c",
		Fields: map[string]storage.Value{
			"ServiceDate__c": storage.DateValue("2026-06-03"),
			"Amount__c":      storage.DecimalValue("160"),
		},
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, record)
	if !ok || value.Kind != storage.ValueString {
		t.Fatalf("formula value = %#v, ok=%v; want string", value, ok)
	}
	if value.String != "2026-06-03;|;160;|;;|;;|;a00000000000001" {
		t.Fatalf("formula value = %q", value.String)
	}
}

func TestCalculatedDatetimeFormulaAddsFractionalDayFromMilliseconds(t *testing.T) {
	field := storage.Field{
		APIName:     "CompletedAt__c",
		Type:        storage.FieldCalculated,
		DisplayType: "DATETIME",
		Formula:     "StartedAt__c + (DurationMs__c / 86400000)",
	}
	org := storage.NewOrgState()
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Invoice__c",
			Fields: map[string]storage.Field{
				field.APIName:   field,
				"StartedAt__c":  {APIName: "StartedAt__c", Type: storage.FieldDateTime},
				"DurationMs__c": {APIName: "DurationMs__c", Type: storage.FieldDecimal},
			},
		},
	}
	definition := org.Objects["Invoice__c"].Definition
	record := storage.Record{
		ID:     "a00000000000001",
		Object: "Invoice__c",
		Fields: map[string]storage.Value{
			"StartedAt__c":  storage.DateTimeValue("2026-05-02T12:00:00Z"),
			"DurationMs__c": storage.DecimalValue("100000"),
		},
	}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, record)
	if !ok || value.Kind != storage.ValueDateTime {
		t.Fatalf("formula value = %#v, ok=%v; want datetime", value, ok)
	}
	if value.String != "2026-05-02T12:01:40Z" {
		t.Fatalf("formula value = %q", value.String)
	}
}

func TestCalculatedFormulaSupportsRoundAndImage(t *testing.T) {
	amountField := storage.Field{
		APIName:     "RecognitionAmount__c",
		Type:        storage.FieldCalculated,
		DisplayType: "CURRENCY",
		Formula:     `Round(Floor(DeferredAmount__c / DeferredTerm__c * 100) / 100, 2)`,
	}
	healthField := storage.Field{
		APIName:     "RecognitionHealth__c",
		Type:        storage.FieldCalculated,
		DisplayType: "STRING",
		Formula: `IF(OR((RecognizedAmount__c != Round((DeferredTerm__c - RemainingRecognitionCount__c) * RecognitionAmount__c, 2) && RemainingRecognitionCount__c > 0),
(RecognizedAmount__c != DeferredAmount__c && RemainingRecognitionCount__c = 0)),
IMAGE('/resource/' + $Setup.Namespace__c.Prefix__c + 'StatusIcons/ERROR.png', 'Status'),
IMAGE('/resource/' + $Setup.Namespace__c.Prefix__c + 'StatusIcons/OK.png', 'Status'))`,
	}
	definition := storage.ObjectDefinition{
		APIName: "DeferredSchedule__c",
		Fields: map[string]storage.Field{
			"DeferredAmount__c":            {APIName: "DeferredAmount__c", Type: storage.FieldDecimal},
			"DeferredTerm__c":              {APIName: "DeferredTerm__c", Type: storage.FieldDecimal},
			"RecognizedAmount__c":          {APIName: "RecognizedAmount__c", Type: storage.FieldDecimal},
			"RemainingRecognitionCount__c": {APIName: "RemainingRecognitionCount__c", Type: storage.FieldDecimal},
			"RecognitionAmount__c":         amountField,
			"RecognitionHealth__c":         healthField,
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{"DeferredSchedule__c": {Definition: definition}}}
	record := storage.Record{
		Object: "DeferredSchedule__c",
		Fields: map[string]storage.Value{
			"DeferredAmount__c":            storage.DecimalValue("6405578.59"),
			"DeferredTerm__c":              storage.DecimalValue("12"),
			"RecognizedAmount__c":          storage.DecimalValue("4804183.89"),
			"RemainingRecognitionCount__c": storage.DecimalValue("3"),
		},
	}

	value, _, ok := EvaluateRecordFormulaValueInOrg(amountField.Formula, amountField, &org, definition, record)
	if !ok || value.Kind != storage.ValueDecimal || value.Decimal != "533798.21" {
		t.Fatalf("recognition amount formula = %#v, ok=%v; want 533798.21", value, ok)
	}

	value, _, ok = EvaluateRecordFormulaValueInOrg(healthField.Formula, healthField, &org, definition, record)
	if !ok || value.Kind != storage.ValueString || !strings.Contains(value.String, "OK.png") {
		t.Fatalf("recognition health formula = %#v, ok=%v; want OK image", value, ok)
	}
}

func TestCalculatedFormulaSupportsSubstituteAndBR(t *testing.T) {
	field := storage.Field{APIName: "Address__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `SUBSTITUTE(Street__c, '\n', '<br />') & BR() & City__c`}
	definition := storage.ObjectDefinition{
		APIName: "Account",
		Fields: map[string]storage.Field{
			"Street__c":  {APIName: "Street__c", Type: storage.FieldString},
			"City__c":    {APIName: "City__c", Type: storage.FieldString},
			"Address__c": field,
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{"Account": {Definition: definition}}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, storage.Record{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Street__c": storage.StringValue(`Line1\nLine2`),
			"City__c":   storage.StringValue("Austin"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Line1<br />Line2<br />Austin" {
		t.Fatalf("address formula = %#v, ok=%v; want substituted text", value, ok)
	}
}

func TestCalculatedFormulaSupportsSalesforceEqualityOperator(t *testing.T) {
	field := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF(Code__c == 'ABC' && RuleStatus__c == 'Active', 'Active', 'Inactive')`}
	definition := storage.ObjectDefinition{
		APIName: "CouponCode__c",
		Fields: map[string]storage.Field{
			"Code__c":       {APIName: "Code__c", Type: storage.FieldString},
			"RuleStatus__c": {APIName: "RuleStatus__c", Type: storage.FieldString},
			"Status__c":     field,
		},
	}
	org := storage.OrgState{
		Objects: map[string]storage.ObjectState{"CouponCode__c": {Definition: definition}},
		Now:     func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) },
	}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, storage.Record{
		Object: "CouponCode__c",
		Fields: map[string]storage.Value{
			"Code__c":       storage.StringValue("ABC"),
			"RuleStatus__c": storage.StringValue("Active"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Active" {
		t.Fatalf("formula value = %#v, ok=%v; want Active", value, ok)
	}
}

func TestCalculatedFormulaEvaluatesCouponStatusShape(t *testing.T) {
	field := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF (ISBLANK(Code__c), 'Inactive',
    IF(OR(!ISBLANK(EndDate__c) && EndDate__c < TODAY(), RuleStatus__c == 'Inactive'), 'Expired',
        IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active')
    )
)`}
	definition := storage.ObjectDefinition{
		APIName: "CouponCode__c",
		Fields: map[string]storage.Field{
			"Code__c":       {APIName: "Code__c", Type: storage.FieldString},
			"StartDate__c":  {APIName: "StartDate__c", Type: storage.FieldDate},
			"EndDate__c":    {APIName: "EndDate__c", Type: storage.FieldDate},
			"RuleStatus__c": {APIName: "RuleStatus__c", Type: storage.FieldString},
			"Status__c":     field,
		},
	}
	org := storage.OrgState{
		Objects: map[string]storage.ObjectState{"CouponCode__c": {Definition: definition}},
		Now:     func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) },
	}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, storage.Record{
		Object: "CouponCode__c",
		Fields: map[string]storage.Value{
			"Code__c":       storage.StringValue("TESTCODE"),
			"StartDate__c":  storage.DateValue("2026-05-03"),
			"RuleStatus__c": storage.StringValue("Active"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Future" {
		t.Fatalf("future formula value = %#v, ok=%v; want Future", value, ok)
	}
}

func TestCalculatedFormulaEvaluatesCouponStatusWithParentRelationship(t *testing.T) {
	field := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF (ISBLANK(Code__c), 'Inactive',
    IF(OR(!ISBLANK(EndDate__c) && EndDate__c < TODAY(), CouponRule__r.Status__c == 'Inactive'), 'Expired',
        IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active')
    )
)`}
	codeDefinition := storage.ObjectDefinition{
		APIName: "CouponCode__c",
		Fields: map[string]storage.Field{
			"Code__c":       {APIName: "Code__c", Type: storage.FieldString},
			"CouponRule__c": {APIName: "CouponRule__c", Type: storage.FieldReference, ReferenceTo: []string{"CouponRule__c"}, RelationshipName: "CouponRule__r"},
			"StartDate__c":  {APIName: "StartDate__c", Type: storage.FieldDate},
			"EndDate__c":    {APIName: "EndDate__c", Type: storage.FieldDate},
			"Status__c":     field,
		},
	}
	ruleDefinition := storage.ObjectDefinition{
		APIName: "CouponRule__c",
		Fields: map[string]storage.Field{
			"Status__c": {APIName: "Status__c", Type: storage.FieldString},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"CouponCode__c": {Definition: codeDefinition},
		"CouponRule__c": {
			Definition: ruleDefinition,
			Records: map[storage.ID]storage.Record{
				"a02000000000001": {ID: "a02000000000001", Object: "CouponRule__c", Fields: map[string]storage.Value{"Status__c": storage.StringValue("Active")}},
			},
		},
	}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, codeDefinition, storage.Record{
		Object: "CouponCode__c",
		Fields: map[string]storage.Value{
			"Code__c":       storage.StringValue("TESTCODE"),
			"CouponRule__c": storage.IDValue("a02000000000001"),
			"StartDate__c":  storage.DateValue("2999-01-01"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Future" {
		t.Fatalf("future relationship formula value = %#v, ok=%v; want Future", value, ok)
	}
}

func TestCalculatedFormulaEvaluatesNestedParentCalculatedField(t *testing.T) {
	codeStatus := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF(CouponRule__r.Status__c == 'Inactive', 'Expired', IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active'))`}
	ruleStatus := storage.Field{APIName: "Status__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `IF(!ISBLANK(StartDate__c) && StartDate__c > TODAY(), 'Future', 'Active')`}
	codeDefinition := storage.ObjectDefinition{
		APIName: "CouponCode__c",
		Fields: map[string]storage.Field{
			"Code__c":       {APIName: "Code__c", Type: storage.FieldString},
			"CouponRule__c": {APIName: "CouponRule__c", Type: storage.FieldReference, ReferenceTo: []string{"CouponRule__c"}},
			"StartDate__c":  {APIName: "StartDate__c", Type: storage.FieldDate},
			"Status__c":     codeStatus,
		},
	}
	ruleDefinition := storage.ObjectDefinition{
		APIName: "CouponRule__c",
		Fields: map[string]storage.Field{
			"StartDate__c": {APIName: "StartDate__c", Type: storage.FieldDate},
			"Status__c":    ruleStatus,
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"CouponCode__c": {Definition: codeDefinition},
		"CouponRule__c": {
			Definition: ruleDefinition,
			Records: map[storage.ID]storage.Record{
				"a02000000000001": {ID: "a02000000000001", Object: "CouponRule__c", Fields: map[string]storage.Value{"StartDate__c": storage.DateValue("2999-01-01")}},
			},
		},
	}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(codeStatus.Formula, codeStatus, &org, codeDefinition, storage.Record{
		Object: "CouponCode__c",
		Fields: map[string]storage.Value{
			"Code__c":       storage.StringValue("TESTCODE"),
			"CouponRule__c": storage.IDValue("a02000000000001"),
		},
	})
	if !ok || value.Kind != storage.ValueString || value.String != "Active" {
		t.Fatalf("nested parent formula value = %#v, ok=%v; want Active", value, ok)
	}
}

func TestCalculatedFormulaBlankValueUsesParentFallback(t *testing.T) {
	displayField := storage.Field{APIName: "ProductDisplayName__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `BLANKVALUE(ProductNameOverride__c, Product__r.Name)`}
	linkDefinition := storage.ObjectDefinition{
		APIName: "PlanTypeProductLink__c",
		Fields: map[string]storage.Field{
			"ProductNameOverride__c": {APIName: "ProductNameOverride__c", Type: storage.FieldString},
			"Product__c":             {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
			"ProductDisplayName__c":  displayField,
		},
		Relations: []storage.Relationship{{
			Field:              "Product__c",
			ParentObjects:      []string{"Product__c"},
			ParentRelationship: "Product__r",
		}},
	}
	productDefinition := storage.ObjectDefinition{
		APIName: "Product__c",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"PlanTypeProductLink__c": {Definition: linkDefinition},
		"Product__c": {
			Definition: productDefinition,
			Records: map[storage.ID]storage.Record{
				"a01000000000001": {ID: "a01000000000001", Object: "Product__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Peanut Butter")}},
			},
		},
	}}
	record := storage.Record{
		Object: "PlanTypeProductLink__c",
		Fields: map[string]storage.Value{
			"Product__c": storage.IDValue("a01000000000001"),
		},
	}
	value, _, ok := EvaluateRecordFormulaValueInOrg(displayField.Formula, displayField, &org, linkDefinition, record)
	if !ok || value.Kind != storage.ValueString || value.String != "Peanut Butter" {
		t.Fatalf("blankvalue parent fallback = %#v, ok=%v; want Peanut Butter", value, ok)
	}

	record.Fields["ProductNameOverride__c"] = storage.StringValue("New Product Name")
	value, _, ok = EvaluateRecordFormulaValueInOrg(displayField.Formula, displayField, &org, linkDefinition, record)
	if !ok || value.Kind != storage.ValueString || value.String != "New Product Name" {
		t.Fatalf("blankvalue override = %#v, ok=%v; want New Product Name", value, ok)
	}
}

func TestRelationshipFormulaEvaluatesParentFormulaBackedField(t *testing.T) {
	childDefinition := storage.ObjectDefinition{
		APIName: "Line__c",
		Fields: map[string]storage.Field{
			"OrderItemLine__c":       {APIName: "OrderItemLine__c", Type: storage.FieldReference, ReferenceTo: []string{"OrderItemLine__c"}},
			"ParentBundleSubtype__c": {APIName: "ParentBundleSubtype__c", Type: storage.FieldString},
			"Product2__c":            {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
			"Product__c":             {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product__r"},
			"Quantity__c":            {APIName: "Quantity__c", Type: storage.FieldDecimal},
		},
	}
	productDefinition := storage.ObjectDefinition{
		APIName: "Product__c",
		Fields: map[string]storage.Field{
			"Inventory__c":       {APIName: "Inventory__c", Type: storage.FieldDecimal},
			"InventoryUsed__c":   {APIName: "InventoryUsed__c", Type: storage.FieldDecimal},
			"InventoryOnHand__c": {APIName: "InventoryOnHand__c", Type: storage.FieldDecimal, Formula: "Inventory__c - InventoryUsed__c"},
			"RecordTypeName__c":  {APIName: "RecordTypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
			"TrackInventory__c":  {APIName: "TrackInventory__c", Type: storage.FieldBoolean, DefaultValue: "false"},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Line__c": {Definition: childDefinition},
		"Product__c": {
			Definition: productDefinition,
			Records: map[storage.ID]storage.Record{
				"a01000000000001": {
					ID:     "a01000000000001",
					Object: "Product__c",
					Fields: map[string]storage.Value{
						"Inventory__c":      storage.DecimalValue("1000"),
						"InventoryUsed__c":  storage.DecimalValue("10"),
						"TrackInventory__c": storage.BooleanValue(true),
					},
				},
			},
		},
	}}

	matches, ok := evaluateValidationFormulaInOrg("Quantity__c > Product__r.InventoryOnHand__c", &org, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("relationship formula-backed validation = %v, ok=%v; want true", matches, ok)
	}
	matches, ok = evaluateValidationFormulaInOrg("AND(ISNEW(), Quantity__c - IF(ISNEW(), 0, PRIORVALUE(Quantity__c)) > Product__r.InventoryOnHand__c)", &org, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("insert validation with ISNEW = %v, ok=%v; want true", matches, ok)
	}
	prior := storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("20"),
		},
	}
	matches, ok = evaluateValidationFormulaInOrg("Quantity__c - IF(ISNEW(), 0, PRIORVALUE(Quantity__c)) > Product__r.InventoryOnHand__c", &org, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1011"),
		},
	}, &prior, false)
	if !ok || !matches {
		t.Fatalf("update validation with PRIORVALUE = %v, ok=%v; want true", matches, ok)
	}
	matches, ok = evaluateValidationFormulaInOrg(`AND(Product__r.TrackInventory__c,
Product__r.RecordTypeName__c != 'Merchandise',
Quantity__c - IF(ISNEW(), 0, PRIORVALUE(Quantity__c)) > Product__r.InventoryOnHand__c,
ISBLANK(OrderItemLine__c) || (!ISNEW() && Quantity__c > PRIORVALUE(Quantity__c)),
TEXT(ParentBundleSubtype__c) != 'Assembled')`, &org, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__c":  storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("cart-style inventory validation = %v, ok=%v; want true", matches, ok)
	}
	matches, ok = evaluateValidationFormulaInOrg(`AND(Product2__r.TrackInventory__c,
Product2__r.RecordTypeName__c != 'Merchandise' || !$Setup.ManagedAppPublicSettings__c.CanBackorderStaffView__c,
Quantity__c - IF(ISNEW(), 0, PRIORVALUE(Quantity__c)) > Product2__r.InventoryOnHand__c,
ISBLANK(OrderItemLine__c) || (!ISNEW() && Quantity__c > PRIORVALUE(Quantity__c)),
TEXT(ParentBundleSubtype__c) != 'Assembled')`, &org, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product2__c": storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("exact cart inventory validation = %v, ok=%v; want true", matches, ok)
	}
	orgWithSetup := org
	productWithMerchandise := orgWithSetup.Objects["Product__c"]
	merchandiseRecord := productWithMerchandise.Records["a01000000000001"]
	merchandiseRecord.Fields["RecordTypeName__c"] = storage.StringValue("Merchandise")
	productWithMerchandise.Records["a01000000000001"] = merchandiseRecord
	orgWithSetup.Objects["Product__c"] = productWithMerchandise
	orgWithSetup.Objects["ManagedAppPublicSettings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ManagedAppPublicSettings__c",
			Metadata: map[string]string{
				"kind":               "customSetting",
				"customSettingsType": "Hierarchy",
			},
			Fields: map[string]storage.Field{
				"Name":                       {APIName: "Name", Type: storage.FieldString},
				"CanBackorderStaffView__c":   {APIName: "CanBackorderStaffView__c", Type: storage.FieldBoolean},
				"OtherBackorderSetting__c":   {APIName: "OtherBackorderSetting__c", Type: storage.FieldBoolean},
				"UnrelatedBackorderFlag__c":  {APIName: "UnrelatedBackorderFlag__c", Type: storage.FieldBoolean},
				"UnrelatedBackorderValue__c": {APIName: "UnrelatedBackorderValue__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0s000000000001": {
				ID:     "a0s000000000001",
				Object: "ManagedAppPublicSettings__c",
				Fields: map[string]storage.Value{
					"Name":                     storage.StringValue("00D000000000001"),
					"CanBackorderStaffView__c": storage.BooleanValue(true),
				},
			},
		},
	}
	matches, ok = evaluateValidationFormulaInOrg(`AND(Product2__r.TrackInventory__c,
Product2__r.RecordTypeName__c != 'Merchandise' || !$Setup.ManagedAppPublicSettings__c.CanBackorderStaffView__c,
Quantity__c - IF(ISNEW(), 0, PRIORVALUE(Quantity__c)) > Product2__r.InventoryOnHand__c,
ISBLANK(OrderItemLine__c) || (!ISNEW() && Quantity__c > PRIORVALUE(Quantity__c)),
TEXT(ParentBundleSubtype__c) != 'Assembled')`, &orgWithSetup, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product2__c": storage.IDValue("a01000000000001"),
			"Quantity__c": storage.DecimalValue("1001"),
		},
	}, nil, true)
	if !ok || matches {
		t.Fatalf("setup-backed cart inventory validation = %v, ok=%v; want false", matches, ok)
	}
	namespacedProductDefinition := productDefinition
	namespacedProductDefinition.Fields = map[string]storage.Field{
		"NU__Inventory__c":       {APIName: "NU__Inventory__c", Type: storage.FieldDecimal},
		"NU__InventoryUsed__c":   {APIName: "NU__InventoryUsed__c", Type: storage.FieldDecimal},
		"NU__InventoryOnHand__c": {APIName: "NU__InventoryOnHand__c", Type: storage.FieldDecimal, Formula: "Inventory__c - InventoryUsed__c"},
		"NU__RecordTypeName__c":  {APIName: "NU__RecordTypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
		"NU__TrackInventory__c":  {APIName: "NU__TrackInventory__c", Type: storage.FieldBoolean, DefaultValue: "false"},
	}
	namespacedOrg := org
	namespacedOrg.Namespace = "NU"
	namespacedProduct := namespacedOrg.Objects["Product__c"]
	namespacedProduct.Definition = namespacedProductDefinition
	namespacedOrg.Objects["Product__c"] = namespacedProduct
	matches, ok = evaluateValidationFormulaInOrg("Product2__r.TrackInventory__c", &namespacedOrg, childDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product2__c": storage.IDValue("a01000000000001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("namespaced parent boolean field = %v, ok=%v; want true", matches, ok)
	}
	namespacedChildDefinition := childDefinition
	namespacedChildDefinition.Fields = map[string]storage.Field{
		"NU__Product2__c": {APIName: "NU__Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
	}
	matches, ok = evaluateValidationFormulaInOrg("Product2__r.TrackInventory__c", &namespacedOrg, namespacedChildDefinition, storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product2__c": storage.IDValue("a01000000000001"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("namespaced relationship lookup field = %v, ok=%v; want true", matches, ok)
	}
}

func TestValidationFormulaSupportsABSWithRelationshipField(t *testing.T) {
	childDefinition := storage.ObjectDefinition{
		APIName: "CartPayment__c",
		Fields: map[string]storage.Field{
			"Cart__c":          {APIName: "Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"Cart__c"}, RelationshipName: "Cart__r"},
			"PaymentAmount__c": {APIName: "PaymentAmount__c", Type: storage.FieldDecimal},
		},
	}
	parentDefinition := storage.ObjectDefinition{
		APIName: "Cart__c",
		Fields: map[string]storage.Field{
			"Balance__c": {APIName: "Balance__c", Type: storage.FieldDecimal},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"CartPayment__c": {Definition: childDefinition},
		"Cart__c": {
			Definition: parentDefinition,
			Records: map[storage.ID]storage.Record{
				"a01000000000001": {
					ID:     "a01000000000001",
					Object: "Cart__c",
					Fields: map[string]storage.Value{
						"Balance__c": storage.DecimalValue("0"),
					},
				},
			},
		},
	}}

	matches, ok := evaluateValidationFormulaInOrg("ABS(PaymentAmount__c) > ABS(Cart__r.Balance__c)", &org, childDefinition, storage.Record{
		Object: "CartPayment__c",
		Fields: map[string]storage.Value{
			"Cart__c":          storage.IDValue("a01000000000001"),
			"PaymentAmount__c": storage.DecimalValue("-10"),
		},
	}, nil, true)
	if !ok || !matches {
		t.Fatalf("abs relationship validation = %v, ok=%v; want true", matches, ok)
	}
}

func TestStandaloneRecordTypeNameDefaultEvaluatesFromRecordTypeID(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName: "DeferredRevenueMethod__c",
		Fields: map[string]storage.Field{
			"RecordTypeId":      {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			"RecordTypeName__c": {APIName: "RecordTypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
		},
		RecordTypes: []storage.RecordTypeInfo{{
			ID:            "012000000000001",
			Name:          "Subscription",
			DeveloperName: "Subscription",
			Active:        true,
			Available:     true,
		}},
	}
	value, ok := defaultValueForRecordField(nil, definition, storage.Record{
		Object: "DeferredRevenueMethod__c",
		Fields: map[string]storage.Value{
			"RecordTypeId": storage.IDValue("012000000000001"),
		},
	}, definition.Fields["RecordTypeName__c"])
	if !ok || value.Kind != storage.ValueString || value.String != "Subscription" {
		t.Fatalf("record type name default = %#v, ok=%v; want Subscription", value, ok)
	}
}

func TestInsertRefreshesStaleRecordTypeNameDefault(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName:   "DeferredRevenueMethod__c",
		KeyPrefix: "a6p",
		Fields: map[string]storage.Field{
			"RecordTypeId":      {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			"RecordTypeName__c": {APIName: "RecordTypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
		},
		RecordTypes: []storage.RecordTypeInfo{
			{ID: "012000000000020", Name: "Coupon", DeveloperName: "Coupon", Active: true, Available: true, Default: true},
			{ID: "012000000000021", Name: "Subscription", DeveloperName: "Subscription", Active: true, Available: true},
		},
	}
	org := storage.NewOrgState()
	org.Objects["DeferredRevenueMethod__c"] = storage.ObjectState{
		Definition: definition,
		Records:    map[storage.ID]storage.Record{},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "DeferredRevenueMethod__c",
		Fields: map[string]storage.Value{
			"RecordTypeId":      storage.IDValue("012000000000021"),
			"RecordTypeName__c": storage.StringValue("Coupon"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	record := org.Objects["DeferredRevenueMethod__c"].Records[insert[0].ID]
	value, ok := record.GetField("RecordTypeName__c")
	if !ok || value.Kind != storage.ValueString || value.String != "Subscription" {
		t.Fatalf("record type name after insert = %#v, ok=%v; want Subscription", value, ok)
	}
}

func TestInsertRefreshesRawRecordTypeNameDefault(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName:   "Product__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"RecordTypeId":      {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			"RecordTypeName__c": {APIName: "RecordTypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
		},
		RecordTypes: []storage.RecordTypeInfo{{
			ID:            "012000000000001",
			Name:          "Standard",
			DeveloperName: "Standard",
			Active:        true,
			Available:     true,
			Default:       true,
		}},
	}
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: definition,
		Records:    map[storage.ID]storage.Record{},
	}
	engine := NewEngine(&org)

	insert := engine.Insert([]storage.Record{{
		Object: "Product__c",
		Fields: map[string]storage.Value{
			"RecordTypeId":      storage.IDValue("012000000000001"),
			"RecordTypeName__c": storage.StringValue("$RecordType.Name"),
		},
	}})
	if !insert[0].Success {
		t.Fatalf("insert = %#v", insert)
	}
	record := org.Objects["Product__c"].Records[insert[0].ID]
	value, ok := record.GetField("RecordTypeName__c")
	if !ok || value.Kind != storage.ValueString || value.String != "Standard" {
		t.Fatalf("record type name after insert = %#v, ok=%v; want Standard", value, ok)
	}
}

func TestFormulaDownloadURLStringFunctionsAndBlankCalculatedText(t *testing.T) {
	entityDefinition := storage.ObjectDefinition{
		APIName: "Entity__c",
		Fields: map[string]storage.Field{
			"DownloadProxyUrl__c": {APIName: "DownloadProxyUrl__c", Type: storage.FieldString},
			"CommunityHubUrl__c":  {APIName: "CommunityHubUrl__c", Type: storage.FieldString},
		},
	}
	productDefinition := storage.ObjectDefinition{
		APIName: "Product__c",
		Fields: map[string]storage.Field{
			"IsDownloadable__c": {APIName: "IsDownloadable__c", Type: storage.FieldBoolean},
			"Entity__c":         {APIName: "Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"Entity__c"}, RelationshipName: "Entity__r"},
		},
	}
	lineDefinition := storage.ObjectDefinition{
		APIName: "OrderItemLine__c",
		Fields: map[string]storage.Field{
			"Product2__c": {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product2__r"},
			"DownloadUrl__c": {
				APIName:     "DownloadUrl__c",
				Type:        storage.FieldCalculated,
				DisplayType: "STRING",
				Formula: `IF(
  Product2__r.IsDownloadable__c,
  IF(
    /** keep absolute proxy URLs as-is */
    OR(BEGINS(Product2__r.Entity__r.DownloadProxyUrl__c, 'http:'), BEGINS(Product2__r.Entity__r.DownloadProxyUrl__c, 'https:')),
    Product2__r.Entity__r.DownloadProxyUrl__c,
    IF(CONTAINS(RIGHT(Product2__r.Entity__r.CommunityHubUrl__c, 1), '/'), MID(Product2__r.Entity__r.CommunityHubUrl__c, 0, LEN(Product2__r.Entity__r.CommunityHubUrl__c) - 1), Product2__r.Entity__r.CommunityHubUrl__c)
      + IF(CONTAINS(LEFT(Product2__r.Entity__r.DownloadProxyUrl__c, 1), '/'), '', '/')
      + Product2__r.Entity__r.DownloadProxyUrl__c
  ) + '?id=' + CASESAFEID(Id),
  ''
)`,
			},
		},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Entity__c": {
			Definition: entityDefinition,
			Records: map[storage.ID]storage.Record{
				"a0E000000000001AAA": {
					ID:     "a0E000000000001AAA",
					Object: "Entity__c",
					Fields: map[string]storage.Value{
						"DownloadProxyUrl__c": storage.StringValue("/download"),
						"CommunityHubUrl__c":  storage.StringValue("https://hub.example/"),
					},
				},
			},
		},
		"Product__c": {
			Definition: productDefinition,
			Records: map[storage.ID]storage.Record{
				"a0P000000000001AAA": {
					ID:     "a0P000000000001AAA",
					Object: "Product__c",
					Fields: map[string]storage.Value{
						"IsDownloadable__c": storage.BooleanValue(true),
						"Entity__c":         storage.IDValue("a0E000000000001AAA"),
					},
				},
				"a0P000000000002AAA": {
					ID:     "a0P000000000002AAA",
					Object: "Product__c",
					Fields: map[string]storage.Value{
						"IsDownloadable__c": storage.BooleanValue(false),
						"Entity__c":         storage.IDValue("a0E000000000001AAA"),
					},
				},
			},
		},
		"OrderItemLine__c": {Definition: lineDefinition},
	}}
	line := storage.Record{
		ID:     "a0L000000000001AAA",
		Object: "OrderItemLine__c",
		Fields: map[string]storage.Value{
			"Product2__c": storage.IDValue("a0P000000000001AAA"),
		},
	}

	value, explicitNull, ok := EvaluateRecordFormulaValueInOrg(lineDefinition.Fields["DownloadUrl__c"].Formula, lineDefinition.Fields["DownloadUrl__c"], &org, lineDefinition, line)
	if !ok || explicitNull || value.Kind != storage.ValueString || value.String != "https://hub.example/download?id=a0L000000000001AAA" {
		t.Fatalf("download formula = %#v explicitNull=%v ok=%v", value, explicitNull, ok)
	}

	line.Fields["Product2__c"] = storage.IDValue("a0P000000000002AAA")
	value, explicitNull, ok = EvaluateRecordFormulaValueInOrg(lineDefinition.Fields["DownloadUrl__c"].Formula, lineDefinition.Fields["DownloadUrl__c"], &org, lineDefinition, line)
	if !ok || !explicitNull || value.Kind != storage.ValueNull {
		t.Fatalf("blank download formula = %#v explicitNull=%v ok=%v; want null", value, explicitNull, ok)
	}
}

func TestValidationRulesDateFunctionsAndDateArithmetic(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["StartDate__c"] = storage.Field{APIName: "StartDate__c", Type: storage.FieldDate}
	account.Definition.Fields["EndDate__c"] = storage.Field{APIName: "EndDate__c", Type: storage.FieldDate}
	account.Definition.ValidationRules = []storage.ValidationRule{
		{
			Name:                  "StartDateFirstDayOfMonth",
			Active:                true,
			ErrorConditionFormula: `AND(ISBLANK(StartDate__c) = FALSE, DAY(StartDate__c) <> 1)`,
			ErrorMessage:          "start date must be first day",
			ErrorDisplayField:     "StartDate__c",
		},
		{
			Name:                  "EndDateLastDayOfMonth",
			Active:                true,
			ErrorConditionFormula: `AND(ISBLANK(EndDate__c) = FALSE, DAY(EndDate__c) <> IF(MONTH(EndDate__c)=12, 31, DAY(DATE(YEAR(EndDate__c), MONTH(EndDate__c)+1, 1) - 1)))`,
			ErrorMessage:          "end date must be last day",
			ErrorDisplayField:     "EndDate__c",
		},
	}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	allowed := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Allowed"),
			"StartDate__c": storage.DateValue("2026-05-01"),
			"EndDate__c":   storage.DateValue("2026-05-31"),
		},
	}})
	if !allowed[0].Success {
		t.Fatalf("allowed insert = %#v", allowed)
	}

	badStart := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Bad Start"),
			"StartDate__c": storage.DateValue("2026-05-02"),
			"EndDate__c":   storage.DateValue("2026-05-31"),
		},
	}})
	if badStart[0].Success || badStart[0].Error != "start date must be first day" || badStart[0].Fields[0] != "StartDate__c" {
		t.Fatalf("bad start = %#v", badStart)
	}

	badEnd := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Bad End"),
			"StartDate__c": storage.DateValue("2026-05-01"),
			"EndDate__c":   storage.DateValue("2026-05-30"),
		},
	}})
	if badEnd[0].Success || badEnd[0].Error != "end date must be last day" || badEnd[0].Fields[0] != "EndDate__c" {
		t.Fatalf("bad end = %#v", badEnd)
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

func TestWorkflowFieldUpdateWithIsolationJournalAvoidsRollbackSnapshot(t *testing.T) {
	storage.ResetCloneStats()
	t.Cleanup(storage.ResetCloneStats)
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "MarkActive",
		Active: true,
		FieldUpdates: []storage.WorkflowFieldUpdate{{
			Name:         "SetStatus",
			Field:        "Status__c",
			LiteralValue: "Active",
		}},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	engine.IsolationJournal = storage.NewIsolationJournal(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !result[0].Success {
		t.Fatalf("insert = %#v", result)
	}
	if got := org.Objects["Account"].Records[result[0].ID].Fields["Status__c"].String; got != "Active" {
		t.Fatalf("workflow status = %q", got)
	}
	if got := storage.SnapshotCloneStats().CloneRollbackSnapshotCalls; got != 0 {
		t.Fatalf("rollback snapshots = %d, want journal rollback path", got)
	}
}

func TestWorkflowFieldUpdateBooleanNumericLiteral(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Type: storage.FieldBoolean}
	account.Definition.Fields["Primary__c"] = storage.Field{APIName: "Primary__c", Type: storage.FieldBoolean}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "SetBooleans",
		Active: true,
		FieldUpdates: []storage.WorkflowFieldUpdate{
			{Name: "SetInactive", Field: "Active__c", LiteralValue: "0"},
			{Name: "SetPrimary", Field: "Primary__c", LiteralValue: "1"},
		},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Active__c": storage.BooleanValue(true)},
	}})
	if !result[0].Success {
		t.Fatalf("insert = %#v", result)
	}
	record := org.Objects["Account"].Records[result[0].ID]
	if got := record.Fields["Active__c"].Boolean; got {
		t.Fatalf("numeric false literal = %v", got)
	}
	if got := record.Fields["Primary__c"].Boolean; !got {
		t.Fatalf("numeric true literal = %v", got)
	}
}

func TestWorkflowNotEqualBlankCriteriaDoesNotMatchMissingOrBlankField(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RemovalReason__c"] = storage.Field{APIName: "RemovalReason__c", Type: storage.FieldString}
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "MarkInactive",
		Active: true,
		Criteria: []storage.WorkflowCriteriaItem{{
			Field:     "RemovalReason__c",
			Operation: "notEqual",
		}},
		FieldUpdates: []storage.WorkflowFieldUpdate{{
			Name:         "SetInactive",
			Field:        "Status__c",
			LiteralValue: "Inactive",
		}},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)

	missing := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Missing")},
	}})
	if !missing[0].Success {
		t.Fatalf("missing insert = %#v", missing)
	}
	if _, ok := org.Objects["Account"].Records[missing[0].ID].Fields["Status__c"]; ok {
		t.Fatalf("workflow should not match missing blank criterion: %#v", org.Objects["Account"].Records[missing[0].ID])
	}

	blank := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Blank"),
			"RemovalReason__c": storage.StringValue(""),
		},
	}})
	if !blank[0].Success {
		t.Fatalf("blank insert = %#v", blank)
	}
	if _, ok := org.Objects["Account"].Records[blank[0].ID].Fields["Status__c"]; ok {
		t.Fatalf("workflow should not match explicit blank criterion: %#v", org.Objects["Account"].Records[blank[0].ID])
	}

	hit := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Hit"),
			"RemovalReason__c": storage.StringValue("Moved"),
		},
	}})
	if !hit[0].Success {
		t.Fatalf("hit insert = %#v", hit)
	}
	if got := org.Objects["Account"].Records[hit[0].ID].Fields["Status__c"].String; got != "Inactive" {
		t.Fatalf("workflow status after nonblank value = %q", got)
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

func TestWorkflowFieldUpdateResolvesRelationshipFormulaOnIdOnlyUpdate(t *testing.T) {
	org := storage.NewOrgState()
	parentDefinition := storage.ObjectDefinition{
		APIName:   "Parent__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"Name":     {APIName: "Name", Type: storage.FieldString},
			"Email__c": {APIName: "Email__c", Type: storage.FieldString},
		},
	}
	childDefinition := storage.ObjectDefinition{
		APIName:   "Child__c",
		KeyPrefix: "a02",
		Fields: map[string]storage.Field{
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"Parent__c":    {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"EmailCopy__c": {APIName: "EmailCopy__c", Type: storage.FieldString},
		},
		WorkflowRules: []storage.WorkflowRule{{
			Name:    "CopyParentEmail",
			Active:  true,
			Formula: "!ISBLANK(Parent__r.Email__c)",
			FieldUpdates: []storage.WorkflowFieldUpdate{{
				Name:    "SetEmailCopy",
				Field:   "EmailCopy__c",
				Formula: "Parent__r.Email__c",
			}},
		}},
	}
	org.Objects["Parent__c"] = storage.ObjectState{Definition: parentDefinition, Records: map[storage.ID]storage.Record{}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: childDefinition, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)

	parent := engine.Insert([]storage.Record{{Object: "Parent__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent)
	}
	child := engine.Insert([]storage.Record{{Object: "Child__c", Fields: map[string]storage.Value{
		"Name":      storage.StringValue("Child"),
		"Parent__c": storage.IDValue(parent[0].ID),
	}}})
	if !child[0].Success {
		t.Fatalf("child insert = %#v", child)
	}
	if _, ok := org.Objects["Child__c"].Records[child[0].ID].Fields["EmailCopy__c"]; ok {
		t.Fatalf("workflow should not copy blank parent email: %#v", org.Objects["Child__c"].Records[child[0].ID])
	}

	parentUpdate := engine.Update([]storage.Record{{ID: parent[0].ID, Object: "Parent__c", Fields: map[string]storage.Value{"Email__c": storage.StringValue("test@example.com")}}})
	if !parentUpdate[0].Success {
		t.Fatalf("parent update = %#v", parentUpdate)
	}
	childUpdate := engine.Update([]storage.Record{{ID: child[0].ID, Object: "Child__c"}})
	if !childUpdate[0].Success {
		t.Fatalf("child update = %#v", childUpdate)
	}
	if got := org.Objects["Child__c"].Records[child[0].ID].Fields["EmailCopy__c"].String; got != "test@example.com" {
		t.Fatalf("relationship workflow formula copy = %q", got)
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

func TestFormulaEvaluatesMultiplication(t *testing.T) {
	org := testOrg()
	definition := storage.ObjectDefinition{
		APIName: "Line__c",
		Fields: map[string]storage.Field{
			"UnitPrice__c": {APIName: "UnitPrice__c", Type: storage.FieldDecimal},
			"Quantity__c":  {APIName: "Quantity__c", Type: storage.FieldInteger},
			"Fees__c":      {APIName: "Fees__c", Type: storage.FieldDecimal},
			"Total__c":     {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "(UnitPrice__c * Quantity__c) + Fees__c"},
		},
	}
	record := storage.Record{Object: "Line__c", Fields: map[string]storage.Value{
		"UnitPrice__c": storage.DecimalValue("10"),
		"Quantity__c":  storage.IntegerValue(2),
		"Fees__c":      storage.DecimalValue("1.5"),
	}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(definition.Fields["Total__c"].Formula, definition.Fields["Total__c"], &org, definition, record)
	if !ok || value.Kind != storage.ValueDecimal || value.Decimal != "21.5" {
		t.Fatalf("formula value = %#v, ok=%v", value, ok)
	}
}

func TestFormulaEvaluatesRelatedObjectFormulaField(t *testing.T) {
	org := storage.NewOrgState()
	parentDefinition := storage.ObjectDefinition{
		APIName:   "Parent__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			"Fees__c":   {APIName: "Fees__c", Type: storage.FieldDecimal},
			"Total__c":  {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "Amount__c + Fees__c"},
		},
	}
	childDefinition := storage.ObjectDefinition{
		APIName:   "Child__c",
		KeyPrefix: "a02",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"Total__c":  {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "Currency", Formula: "Parent__r.Total__c"},
		},
	}
	parent := storage.Record{ID: "a01000000000001", Object: "Parent__c", Fields: map[string]storage.Value{
		"Amount__c": storage.DecimalValue("10"),
		"Fees__c":   storage.DecimalValue("2.5"),
	}}
	child := storage.Record{ID: "a02000000000001", Object: "Child__c", Fields: map[string]storage.Value{
		"Parent__c": storage.IDValue(parent.ID),
	}}
	org.Objects["Parent__c"] = storage.ObjectState{Definition: parentDefinition, Records: map[storage.ID]storage.Record{parent.ID: parent}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: childDefinition, Records: map[storage.ID]storage.Record{child.ID: child}}

	value, _, ok := EvaluateRecordFormulaValueInOrg(childDefinition.Fields["Total__c"].Formula, childDefinition.Fields["Total__c"], &org, childDefinition, child)
	if !ok || value.Kind != storage.ValueDecimal || value.Decimal != "12.5" {
		t.Fatalf("related formula value = %#v, ok=%v", value, ok)
	}
}

func TestFlowDecisionBranchesRouteFirstMatchOrDefaultAndTraceValue(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldString}
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:   "RouteStatus",
		Active: true,
		Branches: []storage.FlowBranch{
			{
				Name:     "Enterprise",
				Criteria: []storage.WorkflowCriteriaItem{{Field: "Score__c", Operation: "greaterThanOrEqualTo", Value: "90"}},
				FieldUpdates: []storage.WorkflowFieldUpdate{{
					Name:         "SetEnterpriseStatus",
					Field:        "Status__c",
					LiteralValue: "Priority",
				}},
			},
			{
				Name:     "Startup",
				Criteria: []storage.WorkflowCriteriaItem{{Field: "Score__c", Operation: "greaterThanOrEqualTo", Value: "50"}},
				FieldUpdates: []storage.WorkflowFieldUpdate{{
					Name:         "SetStartupStatus",
					Field:        "Status__c",
					LiteralValue: "Nurture",
				}},
			},
			{
				Name:    "Default",
				Default: true,
				FieldUpdates: []storage.WorkflowFieldUpdate{{
					Name:         "SetDefaultStatus",
					Field:        "Status__c",
					LiteralValue: "Standard",
				}},
			},
		},
	}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	var decisions []map[string]any
	var updates []map[string]any
	engine.AutomationTracer = func(name string, args map[string]any) {
		switch name {
		case "apex.flow.decision":
			decisions = append(decisions, args)
		case "apex.flow.field_update":
			updates = append(updates, args)
		}
	}

	insert := engine.Insert([]storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme"), "Score__c": storage.IntegerValue(95)}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta"), "Score__c": storage.IntegerValue(65)}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Core"), "Score__c": storage.IntegerValue(20)}},
	})
	for _, result := range insert {
		if !result.Success {
			t.Fatalf("insert = %#v", insert)
		}
	}
	assertAccountStatus := func(id storage.ID, want string) {
		t.Helper()
		if got := org.Objects["Account"].Records[id].Fields["Status__c"].String; got != want {
			t.Fatalf("status for %s = %q, want %q", id, got, want)
		}
	}
	assertAccountStatus(insert[0].ID, "Priority")
	assertAccountStatus(insert[1].ID, "Nurture")
	assertAccountStatus(insert[2].ID, "Standard")
	if len(decisions) != 3 || decisions[0]["branch"] != "Enterprise" || decisions[1]["branch"] != "Startup" || decisions[2]["branch"] != "Default" || decisions[2]["default"] != true {
		t.Fatalf("decision traces = %#v", decisions)
	}
	if len(updates) != 3 || updates[0]["value"] != "Priority" || updates[1]["value"] != "Nurture" || updates[2]["value"] != "Standard" {
		t.Fatalf("field update traces = %#v", updates)
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

func TestFlowRecordCreateFailureRollsBackPriorCreatesAndSourceInsert(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:   "CreateThenFail",
		Active: true,
		RecordCreates: []storage.FlowRecordCreate{
			{
				Name:       "CreateRequest",
				ObjectName: "ActionRequest__c",
				InputAssignments: []storage.WorkflowFieldUpdate{
					{Name: "ActionName__c", Field: "ActionName__c", LiteralValue: "Notify"},
					{Name: "SourceRecordId__c", Field: "SourceRecordId__c", SourceField: "Id"},
				},
			},
			{
				Name:       "CreateMissing",
				ObjectName: "Missing__c",
			},
		},
	}}
	org.Objects["Account"] = account
	org.Objects["ActionRequest__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ActionRequest__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"ActionName__c":     {APIName: "ActionName__c", Type: storage.FieldString, Required: true},
				"SourceRecordId__c": {APIName: "SourceRecordId__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	var events []string
	engine.AutomationTracer = func(name string, args map[string]any) {
		events = append(events, name)
	}

	result := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if result[0].Success {
		t.Fatalf("insert succeeded despite failing flow create: %#v events=%#v", result, events)
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("source insert was not rolled back, got %d accounts", got)
	}
	if got := len(org.Objects["ActionRequest__c"].Records); got != 0 {
		t.Fatalf("prior flow create was not rolled back, got %d requests", got)
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
				"Name":         {APIName: "Name", Type: storage.FieldString, Required: true},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			},
			RecordTypes: []storage.RecordTypeInfo{{
				ID:            "012000000000ORG",
				DeveloperName: "Organization",
				Name:          "Organization",
				Active:        true,
				Available:     true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
				"SobjectType":   {APIName: "SobjectType", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000ORG": {Object: "RecordType", ID: "012000000000ORG", Fields: map[string]storage.Value{
				"Name":          storage.StringValue("Organization"),
				"DeveloperName": storage.StringValue("Organization"),
				"SobjectType":   storage.StringValue("Account"),
			}},
		},
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

func TestAttachmentBodyDMLRoundTrip(t *testing.T) {
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
	if !attachment[0].Success || !strings.HasPrefix(string(attachment[0].ID), "00P") {
		t.Fatalf("attachment insert = %#v", attachment)
	}
	row := org.Objects["Attachment"].Records[attachment[0].ID]
	if row.Fields["ParentId"].ID != account[0].ID || row.Fields["Body"].Kind != storage.ValueBlob || row.Fields["Body"].String != "hello bytes" {
		t.Fatalf("attachment row = %#v", row)
	}
}

func TestAttachmentParentCanReferenceCurrentUser(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	attachment := engine.Insert([]storage.Record{{
		Object: "Attachment",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("user-note.txt"),
			"ParentId": storage.IDValue("005000000000001"),
			"Body":     storage.BlobValue("hello user"),
		},
	}})
	if !attachment[0].Success {
		t.Fatalf("attachment insert = %#v", attachment)
	}
	stored := org.Objects["Attachment"].Records[attachment[0].ID]
	if stored.Fields["ParentId"].ID != "005000000000001" {
		t.Fatalf("parent id = %#v", stored.Fields["ParentId"])
	}
}

func TestDocumentBodyDMLAndDelete(t *testing.T) {
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
	if !document[0].Success || !strings.HasPrefix(string(document[0].ID), "015") {
		t.Fatalf("document insert = %#v", document)
	}
	row := org.Objects["Document"].Records[document[0].ID]
	if row.Fields["Body"].Kind != storage.ValueBlob || row.Fields["Body"].String != "document bytes" || row.Fields["ContentType"].String != "application/pdf" || !row.Fields["IsPublic"].Boolean {
		t.Fatalf("document row = %#v", row)
	}
	if folderID := org.Objects["Document"].Records[document[0].ID].Fields["FolderId"].ID; folderID != "005000000000001" {
		t.Fatalf("document folder id = %q", folderID)
	}
	deleted := engine.Delete([]storage.Record{{Object: "Document", ID: document[0].ID}})
	if !deleted[0].Success {
		t.Fatalf("document delete = %#v", deleted)
	}
	if stored := org.Objects["Document"].Records[document[0].ID]; !stored.System.IsDeleted {
		t.Fatalf("deleted document was not marked deleted: %#v", stored)
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
	if !first[0].Success || !strings.HasPrefix(string(first[0].ID), "068") {
		t.Fatalf("content version insert = %#v", first)
	}
	version := org.Objects["ContentVersion"].Records[first[0].ID]
	documentID := version.Fields["ContentDocumentId"].ID
	if !strings.HasPrefix(string(documentID), "069") {
		t.Fatalf("content document id = %s", documentID)
	}
	document := org.Objects["ContentDocument"].Records[documentID]
	if document.Fields["LatestPublishedVersionId"].ID != first[0].ID || document.Fields["Title"].String != "Spec" || document.Fields["FileExtension"].String != "pdf" || document.Fields["FileType"].String != "PDF" {
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
	if !second[0].Success || !strings.HasPrefix(string(second[0].ID), "068") || second[0].ID == first[0].ID {
		t.Fatalf("second content version insert = %#v", second)
	}
	if got := len(org.Objects["ContentDocument"].Records); got != 1 {
		t.Fatalf("content documents = %d", got)
	}
	document = org.Objects["ContentDocument"].Records[documentID]
	if document.Fields["LatestPublishedVersionId"].ID != second[0].ID || document.Fields["Title"].String != "Spec v2" || document.Fields["FileExtension"].String != "pdf" || document.Fields["FileType"].String != "PDF" {
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
	if !explicitLink[0].Success || !strings.HasPrefix(string(explicitLink[0].ID), "06A") || explicitLink[0].ID == autoLink.ID {
		t.Fatalf("explicit link insert = %#v", explicitLink)
	}
}

func TestContentVersionWithoutPublishLocationCreatesOwnerLink(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	engine.UserID = "005000000000777"
	inserted := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":        storage.StringValue("Spec"),
			"PathOnClient": storage.StringValue("docs/spec.pdf"),
			"VersionData":  storage.BlobValue("pdf bytes"),
		},
	}})
	if !inserted[0].Success {
		t.Fatalf("content version insert = %#v", inserted)
	}
	version := org.Objects["ContentVersion"].Records[inserted[0].ID]
	documentID := version.Fields["ContentDocumentId"].ID
	if got := len(org.Objects["ContentDocumentLink"].Records); got != 1 {
		t.Fatalf("content document links = %d", got)
	}
	for _, link := range org.Objects["ContentDocumentLink"].Records {
		if link.Fields["ContentDocumentId"].ID != documentID || link.Fields["LinkedEntityId"].ID != engine.UserID {
			t.Fatalf("owner link = %#v", link)
		}
	}
}

func TestContentVersionRejectsEmptyVersionData(t *testing.T) {
	org := fileTestOrg()
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":        storage.StringValue("Empty"),
			"PathOnClient": storage.StringValue("docs/empty.txt"),
			"VersionData":  storage.BlobValue(""),
		},
	}})
	if len(result) != 1 || result[0].Success || result[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("content version insert = %#v", result)
	}
	if len(result[0].Fields) != 1 || result[0].Fields[0] != "VersionData" {
		t.Fatalf("required fields = %#v", result[0].Fields)
	}
}

func TestSnapshotContentVersionSideEffectsDoNotMutateBaseOrg(t *testing.T) {
	base := fileTestOrg()
	snapshot := storage.NewRuntimeTemplate(base).CloneSnapshotOrg()
	engine := NewEngine(&snapshot)
	account := engine.Insert([]storage.Record{{
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
	}})
	if !account[0].Success {
		t.Fatalf("account insert = %#v", account)
	}
	version := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":                  storage.StringValue("Spec"),
			"PathOnClient":           storage.StringValue("docs/spec.pdf"),
			"VersionData":            storage.BlobValue("pdf bytes"),
			"FirstPublishLocationId": storage.IDValue(account[0].ID),
		},
	}})
	if !version[0].Success {
		t.Fatalf("content version insert = %#v", version)
	}
	if got := len(base.Objects["Account"].Records); got != 0 {
		t.Fatalf("base accounts after snapshot insert = %d", got)
	}
	if got := len(base.Objects["ContentVersion"].Records); got != 0 {
		t.Fatalf("base content versions after snapshot insert = %d", got)
	}
	if got := len(base.Objects["ContentDocument"].Records); got != 0 {
		t.Fatalf("base content documents after snapshot insert = %d", got)
	}
	if got := len(base.Objects["ContentDocumentLink"].Records); got != 0 {
		t.Fatalf("base content document links after snapshot insert = %d", got)
	}
}

func TestContentDistributionGeneratesLocalUrls(t *testing.T) {
	org := fileTestOrg()
	storage.EnsureStandardObject(&org, "ContentDistribution")
	engine := NewEngine(&org)
	account := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}})
	version := engine.Insert([]storage.Record{{
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Title":                  storage.StringValue("Spec"),
			"PathOnClient":           storage.StringValue("spec.pdf"),
			"VersionData":            storage.BlobValue("pdf bytes"),
			"FirstPublishLocationId": storage.IDValue(account[0].ID),
		},
	}})
	if !account[0].Success || !version[0].Success {
		t.Fatalf("setup account=%#v version=%#v", account, version)
	}
	dist := engine.Insert([]storage.Record{{
		Object: "ContentDistribution",
		Fields: map[string]storage.Value{
			"Name":             storage.StringValue("Public Link"),
			"ContentVersionId": storage.IDValue(version[0].ID),
			"RelatedRecordId":  storage.IDValue(account[0].ID),
		},
	}})
	if !dist[0].Success {
		t.Fatalf("content distribution insert = %#v", dist)
	}
	record := org.Objects["ContentDistribution"].Records[dist[0].ID]
	if record.Fields["ContentDownloadUrl"].String == "" || record.Fields["DistributionPublicUrl"].String == "" {
		t.Fatalf("content distribution urls = %#v", record)
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

func TestContentVersionCreatesDocumentWithGeneratedStandardSchema(t *testing.T) {
	org := storage.NewOrgState()
	for _, objectName := range []string{"Account", "ContentVersion", "ContentDocument", "ContentDocumentLink"} {
		storage.EnsureStandardObject(&org, objectName)
	}
	storage.EnsureDeterministicPlatformData(&org)
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
	if !first[0].Success {
		t.Fatalf("content version insert = %#v", first)
	}
	version := org.Objects["ContentVersion"].Records[first[0].ID]
	documentID := version.Fields["ContentDocumentId"].ID
	document := org.Objects["ContentDocument"].Records[documentID]
	if document.Fields["LatestPublishedVersionId"].ID != first[0].ID || document.Fields["Title"].String != "Spec" || document.Fields["FileExtension"].String != "pdf" {
		t.Fatalf("content document = %#v", document)
	}

	direct := engine.Insert([]storage.Record{{
		Object: "ContentDocument",
		Fields: map[string]storage.Value{
			"Title":                    storage.StringValue("Direct"),
			"LatestPublishedVersionId": storage.IDValue(first[0].ID),
		},
	}})
	if direct[0].Success || direct[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
		t.Fatalf("direct content document insert = %#v", direct)
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
