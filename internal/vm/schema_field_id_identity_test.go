package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestReadSchemaIDFieldRetainsIdentityWithoutChangingRawString(t *testing.T) {
	raw := String("m0000000000000000E")
	for _, tc := range []struct {
		name     string
		field    storage.Field
		input    Value
		wantType string
	}{
		{"Id", storage.Field{APIName: "Id", Type: storage.FieldID}, raw, "Id"},
		{"text", storage.Field{APIName: "Name", Type: storage.FieldString}, raw, raw.Type},
		{"invalid shape", storage.Field{APIName: "Id", Type: storage.FieldID}, String("invalid"), String("invalid").Type},
		{"null", storage.Field{APIName: "Id", Type: storage.FieldID}, Null, "Id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := coerceReadSchemaFieldIdentity(Object("Owned__mdt"), tc.input, tc.field)
			if got.Type != tc.wantType || got.Text != tc.input.Text || got.Kind != tc.input.Kind {
				t.Fatalf("read = %#v, input = %#v, want type %q", got, tc.input, tc.wantType)
			}
		})
	}
	if raw.Type == "Id" {
		t.Fatal("read mutated raw String")
	}
}

func TestReadNamedMetadataRelationshipNormalizesQualifiedAPINameToString(t *testing.T) {
	value := String("RollupParent__c")
	value.Type = "Id"
	field := storage.Field{
		APIName:     "LookupFieldOnCalcItem__c",
		Type:        storage.FieldReference,
		ReferenceTo: []string{"FieldDefinition"},
	}
	got := coerceReadSchemaFieldIdentity(Object("Rollup__mdt"), value, field)
	if got.Kind != ValueString || got.Text != "RollupParent__c" || got.Type != "" {
		t.Fatalf("named metadata relationship read = %#v", got)
	}
}

func TestSyntheticIDFieldDefinitionWithoutStoredField(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Owned__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Owned__mdt", KeyPrefix: "m00"}}
	machine := New(nil)
	machine.SetOrg(&org)
	_, field, ok := machine.sObjectFieldDefinition("Owned__mdt", "Id")
	if !ok || field.Type != storage.FieldID || field.APIName != "Id" {
		t.Fatalf("synthetic Id = %#v, found %v", field, ok)
	}
	row := Object("Owned__mdt")
	row.Fields["Id"] = String("m0000000000000000E")
	got, err := machine.lookupPath(row, []string{"Id"})
	if err != nil || got.Type != "Id" {
		t.Fatalf("Id read = %#v, %v", got, err)
	}
	if row.Fields["Id"].Type == "Id" {
		t.Fatal("read mutated stored field")
	}
	if err := machine.assignPath(row, []string{"Id"}, String("invalid")); err == nil {
		t.Fatal("invalid Id assignment accepted")
	}
	if row.Fields["Id"].Text != "m0000000000000000E" {
		t.Fatal("invalid assignment changed stored Id")
	}
}

func TestSyntheticIDFieldDefinitionResolvesLocalNamespaceAlias(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Owned__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "pkg__Owned__mdt", KeyPrefix: "m00"}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, name := range []string{"Owned__mdt", "pkg__Owned__mdt"} {
		_, field, ok := machine.sObjectFieldDefinition(name, "Id")
		if !ok || field.Type != storage.FieldID {
			t.Fatalf("%s Id = %#v, found %v", name, field, ok)
		}
	}
}

func TestUserClassWithStandardObjectNameDoesNotApplySObjectIDValidation(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Folder"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Folder",
		Fields: map[string]storage.Field{
			"Id": {APIName: "Id", Type: storage.FieldID},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.Classes["Folder"] = Class{Name: "Folder"}

	row := Object("Folder")
	if err := machine.assignPath(row, []string{"Id"}, String("1")); err != nil {
		t.Fatalf("user-class Id assignment returned %v", err)
	}
	if got := row.Fields["Id"]; got.Kind != ValueObject || got.Type != "Id" || got.Fields["value"].Text != "1" {
		t.Fatalf("user-class Id assignment = %#v", got)
	}
}
