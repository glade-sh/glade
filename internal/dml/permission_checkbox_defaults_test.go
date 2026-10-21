package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInsertPermissionSetCheckboxDefaultsWithFieldAlias(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSet")
	object := org.Objects["PermissionSet"]
	object.Definition = object.Definition.Clone()
	field, found := object.Definition.Fields["PermissionsEmailSingle"]
	if !found || field.Type != storage.FieldBoolean {
		t.Fatal("catalog checkbox missing")
	}
	delete(object.Definition.Fields, "PermissionsEmailSingle")
	object.Definition.Fields["permissionsEmailSingle"] = field
	org.Objects["PermissionSet"] = object
	engine := NewEngine(&org)
	results := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: map[string]storage.Value{
		"Name": storage.StringValue("CheckboxAliasGrant"), "Label": storage.StringValue("Checkbox alias grant"),
	}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("sparse checkbox insert = %#v", results)
	}
	stored := org.Objects["PermissionSet"].Records[results[0].ID]
	value, exists := stored.GetField("permissionsEmailSingle")
	if !exists || value.Kind != storage.ValueBoolean || value.Boolean {
		t.Fatalf("omitted aliased checkbox = %#v, exists=%t", value, exists)
	}
}

// These controls preserve existing local null-write validation; they do not
// claim a new native null/update observation for permission checkboxes.
func TestPermissionSetCheckboxNullWritesPreserveValidation(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSet")
	engine := NewEngine(&org)
	fields := map[string]storage.Value{"Name": storage.StringValue("NullCheckboxGrant"), "Label": storage.StringValue("Null checkbox grant")}
	nullInsert := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: fields,
		ExplicitNulls: map[string]bool{"PermissionsEmailSingle": true}}})
	if len(nullInsert) != 1 || nullInsert[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("existing explicit-null insert validation = %#v", nullInsert)
	}
	inserted := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: fields}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("omitted checkbox insert = %#v", inserted)
	}
	nullUpdate := engine.Update([]storage.Record{{Object: "PermissionSet", ID: inserted[0].ID,
		ExplicitNulls: map[string]bool{"PermissionsEmailSingle": true}}})
	if len(nullUpdate) != 1 || nullUpdate[0].StatusCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("existing explicit-null update validation = %#v", nullUpdate)
	}
}

// Owned B44X01Q @IsTest rows at API 62/67 insert these four grants while
// omitting the view-all/modify-all checkboxes. The insert must succeed.
func TestInsertSparseObjectPermissionsCheckboxes(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSet")
	storage.EnsureStandardObject(&org, "ObjectPermissions")
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "PermissionSet", Fields: map[string]storage.Value{
		"Name": storage.StringValue("SparseObjectGrant"), "Label": storage.StringValue("Sparse object grant"),
	}}})
	if len(parent) != 1 || !parent[0].Success {
		t.Fatalf("permission set insert = %#v", parent)
	}
	results := engine.Insert([]storage.Record{{Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(parent[0].ID), "SObjectType": storage.StringValue("Contact"),
		"PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true),
		"PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true),
	}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("sparse object permission insert = %#v", results)
	}
	stored := org.Objects["ObjectPermissions"].Records[results[0].ID]
	for _, name := range []string{"PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete"} {
		value, exists := stored.GetField(name)
		if !exists || value.Kind != storage.ValueBoolean || !value.Boolean {
			t.Fatalf("supplied grant %s = %#v, exists=%t", name, value, exists)
		}
	}
}

func TestPermissionSetCheckboxDefaultsPreserveOtherBooleanInputs(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "PermissionSet", Fields: map[string]storage.Field{
		"HasActivationRequired": {APIName: "HasActivationRequired", Type: storage.FieldBoolean, Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false), DefaultedOnCreate: storage.BoolFlag(false)},
	}}
	record := storage.Record{Object: "PermissionSet"}
	applySetupInsertDefaults("PermissionSet", definition, &record)
	if _, found := record.GetField("HasActivationRequired"); found || isDMLRequiredField(definition, "HasActivationRequired", definition.Fields["HasActivationRequired"]) {
		t.Fatal("permission omission fallback affected another Boolean input")
	}
	definition.Fields["HasActivationRequired"] = storage.Field{APIName: "HasActivationRequired", Type: storage.FieldBoolean, Required: true}
	if !isDMLRequiredField(definition, "HasActivationRequired", definition.Fields["HasActivationRequired"]) {
		t.Fatal("explicit required input was removed")
	}
}
