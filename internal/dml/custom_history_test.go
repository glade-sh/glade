package dml

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestGeneratedHistoryInsertValidatorBoundaries(t *testing.T) {
	definition := historyInsertValidatorDefinition()
	for _, tc := range []struct {
		name      string
		seed      bool
		fields    map[string]storage.Value
		wantCode  string
		wantField string
		wantError string
	}{
		{
			// This is a local unsupported-surface guard, not a native claim
			// about unprobed asynchronous or upsert exception semantics.
			name:     "empty input without seed capability",
			wantCode: "INVALID_TYPE",
		},
		{
			name: "captured seed inputs", seed: true,
			fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000001"), "Field": storage.StringValue("Stage__c")},
		},
		{
			name: "missing parent", seed: true,
			fields:    map[string]storage.Value{"Field": storage.StringValue("Stage__c")},
			wantCode:  "INVALID_API_INPUT",
			wantError: "Invalid null value for field: ParentId",
		},
		{
			name: "missing field", seed: true,
			fields:    map[string]storage.Value{"ParentId": storage.IDValue("a00000000000001")},
			wantCode:  "INVALID_API_INPUT",
			wantError: "Invalid null value for field: Field",
		},
		{
			name: "extra readonly value", seed: true,
			fields: map[string]storage.Value{
				"ParentId": storage.IDValue("a00000000000001"),
				"Field":    storage.StringValue("Stage__c"),
				"OldValue": storage.StringValue("previous"),
			},
			wantCode: "INVALID_FIELD_FOR_INSERT_UPDATE", wantField: "OldValue",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			engine.Options.AllowTestHistoryInsert = tc.seed
			record := storage.Record{Object: definition.APIName, Fields: tc.fields}
			fieldCount := len(record.Fields)
			err := engine.validateInsertFieldWriteability(definition, record)
			assertHistoryInsertValidatorError(t, err, tc.wantCode, tc.wantField, tc.wantError)
			if len(record.Fields) != fieldCount {
				t.Fatal("validation mutated caller fields")
			}
		})
	}
}

func TestGeneratedHistoryInsertValidatorPreservesOrdinaryObjects(t *testing.T) {
	for _, seed := range []bool{false, true} {
		org := storage.NewOrgState()
		engine := NewEngine(&org)
		engine.Options.AllowTestHistoryInsert = seed
		ordinary := storage.ObjectDefinition{
			APIName: "PlainParcel__c",
			Fields: map[string]storage.Field{
				"Editable__c": {APIName: "Editable__c", Type: storage.FieldString, Createable: storage.BoolFlag(true)},
				"Readonly__c": {APIName: "Readonly__c", Type: storage.FieldString, Createable: storage.BoolFlag(false)},
			},
		}
		if err := engine.validateInsertFieldWriteability(ordinary, storage.Record{Object: ordinary.APIName, Fields: map[string]storage.Value{"Editable__c": storage.StringValue("accepted")}}); err != nil {
			t.Fatalf("ordinary writable control with seed=%v: %v", seed, err)
		}
		err := engine.validateInsertFieldWriteability(ordinary, storage.Record{Object: ordinary.APIName, Fields: map[string]storage.Value{"Readonly__c": storage.StringValue("denied")}})
		assertHistoryInsertValidatorError(t, err, "INVALID_FIELD_FOR_INSERT_UPDATE", "Readonly__c", "")
	}
}

func TestGeneratedHistoryInsertValidatorRequiresProvenanceMarker(t *testing.T) {
	definition := historyInsertValidatorDefinition()
	definition.Metadata = nil
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	engine.Options.AllowTestHistoryInsert = true
	// A source/fixture object's suffix alone must neither acquire the local
	// empty-history boundary nor bypass its ordinary readonly field checks.
	if err := engine.validateInsertFieldWriteability(definition, storage.Record{Object: definition.APIName}); err != nil {
		t.Fatalf("suffix-only empty input changed: %v", err)
	}
	err := engine.validateInsertFieldWriteability(definition, storage.Record{
		Object: definition.APIName,
		Fields: map[string]storage.Value{"Field": storage.StringValue("Stage__c")},
	})
	assertHistoryInsertValidatorError(t, err, "INVALID_FIELD_FOR_INSERT_UPDATE", "Field", "")
}

func historyInsertValidatorDefinition() storage.ObjectDefinition {
	return storage.ObjectDefinition{
		APIName:  "TrackedParcel__History",
		Metadata: map[string]string{"kind": "generatedHistory", "parentObject": "TrackedParcel__c"},
		Fields: map[string]storage.Field{
			"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"TrackedParcel__c"}, RelationshipName: "Parent", Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
			"Field":    {APIName: "Field", Type: storage.FieldPicklist, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
			"OldValue": {APIName: "OldValue", Type: storage.FieldAny, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)},
		},
	}
}

func assertHistoryInsertValidatorError(t *testing.T, err error, code, field, message string) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var typed dmlError
	if !errors.As(err, &typed) || typed.code != code {
		t.Fatalf("validator error=%v, want %s", err, code)
	}
	if field == "" {
		if len(typed.fields) != 0 {
			t.Fatalf("validator fields=%v, want empty", typed.fields)
		}
	} else if len(typed.fields) != 1 || typed.fields[0] != field {
		t.Fatalf("validator fields=%v, want [%s]", typed.fields, field)
	}
	if message != "" && typed.message != message {
		t.Fatalf("validator message=%q, want %q", typed.message, message)
	}
}
