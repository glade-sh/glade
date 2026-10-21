package storage

import (
	"reflect"
	"strings"
	"testing"
)

func TestPermissionSetCheckboxSchemaPreservesDescribeDefaults(t *testing.T) {
	describe, found, err := lookupStandardDescribeCatalogV2("PermissionSet")
	if err != nil || !found {
		t.Fatalf("PermissionSet describe: found=%v error=%v", found, err)
	}
	definition, found := StandardObjectDefinition("PermissionSet")
	if !found {
		t.Fatal("PermissionSet definition missing")
	}
	nonDefaultedCount := 0
	checked := 0
	for _, raw := range describe.Fields {
		if raw.Type != "boolean" || !strings.HasPrefix(raw.Name, "Permissions") || raw.Createable == nil || !*raw.Createable {
			continue
		}
		if raw.Nillable != nil && !*raw.Nillable && raw.DefaultedOnCreate != nil && !*raw.DefaultedOnCreate && raw.DefaultValue == nil {
			nonDefaultedCount++
		}
		field, found := definition.Fields[raw.Name]
		// This check does not broaden the runtime's field availability.
		if !found {
			continue
		}
		checked++
		if field.Type != FieldBoolean {
			t.Fatalf("writable checkbox missing: %s", raw.Name)
		}
		if field.Required {
			t.Fatalf("omitted checkbox is not a required input: %s", raw.Name)
		}
		if field.DefaultValue != describeField(raw).DefaultValue {
			t.Errorf("%s changed its native describe default value", raw.Name)
		}
		for _, facet := range []struct {
			name          string
			native, local *bool
		}{
			{"nillable", raw.Nillable, field.Nillable},
			{"createable", raw.Createable, field.Createable},
			{"defaultedOnCreate", raw.DefaultedOnCreate, field.DefaultedOnCreate},
		} {
			if (facet.native == nil) != (facet.local == nil) || facet.native != nil && *facet.native != *facet.local {
				t.Errorf("%s.%s changed native describe facet", raw.Name, facet.name)
			}
		}
		if raw.Nillable != nil && !*raw.Nillable && raw.DefaultedOnCreate != nil && !*raw.DefaultedOnCreate && raw.DefaultValue == nil {
			if field.DefaultValue != "" {
				t.Errorf("%s acquired an invented describe default: %q", raw.Name, field.DefaultValue)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no visible writable checkbox metadata checked")
	}
	if nonDefaultedCount != 571 {
		t.Fatalf("owned non-defaulted writable checkbox rows: %d, want 571", nonDefaultedCount)
	}
	name, found := definition.Fields["Name"]
	if !found || !name.Required {
		t.Fatal("PermissionSet.Name must remain required")
	}
	checkbox := definition.Fields["PermissionsApiEnabled"]
	checkbox.Required = true
	checkbox.Nillable = nil
	checkbox.DefaultedOnCreate = nil
	definition.Fields["PermissionsApiEnabled"] = checkbox
	if !RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("stale permission checkbox metadata was not refreshed")
	}
	checkbox = definition.Fields["PermissionsApiEnabled"]
	if checkbox.Required || checkbox.Nillable == nil || *checkbox.Nillable || checkbox.DefaultedOnCreate == nil || *checkbox.DefaultedOnCreate {
		t.Fatalf("refreshed checkbox metadata = %#v", checkbox)
	}
	if RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("unchanged permission checkbox metadata needs another refresh")
	}
	activation := definition.Fields["HasActivationRequired"]
	value, hasDefault := DefaultValueForField(activation)
	if !hasDefault || value.Kind != ValueBoolean || value.Boolean {
		t.Fatalf("generic captured activation default changed: %+v", activation)
	}
}

func TestPermissionSetCheckboxSchemaLeavesOtherBooleanInputs(t *testing.T) {
	field := Field{APIName: "HasActivationRequired", Type: FieldBoolean, Required: true, DefaultValue: "true"}
	definition := ObjectDefinition{APIName: "PermissionSet", Fields: map[string]Field{"HasActivationRequired": field}}
	applyPermissionSetCheckboxSchema(&definition)
	if !reflect.DeepEqual(definition.Fields["HasActivationRequired"], field) || permissionSetCheckboxSchemaNeedsRepair(definition) {
		t.Fatal("permission checkbox repair changed another Boolean input")
	}
}

func TestObjectPermissionCheckboxFacetsPreserveSparseInputs(t *testing.T) {
	definition, found := StandardObjectDefinition("ObjectPermissions")
	if !found {
		t.Fatal("ObjectPermissions definition missing")
	}
	describe, found, err := lookupStandardDescribeCatalogV2("ObjectPermissions")
	if err != nil || !found {
		t.Fatalf("ObjectPermissions describe: found=%v error=%v", found, err)
	}
	checked := 0
	for _, captured := range describe.Fields {
		if captured.Type != "boolean" {
			continue
		}
		field, exists := definition.Fields[captured.Name]
		if !exists || field.Required || !reflect.DeepEqual(field.Nillable, captured.Nillable) || !reflect.DeepEqual(field.Createable, captured.Createable) {
			t.Fatalf("sparse checkbox facets changed: %s %+v", captured.Name, field)
		}
		checked++
	}
	if checked != 7 {
		t.Fatalf("owned ObjectPermissions checkbox shapes: %d, want 7", checked)
	}
	field := definition.Fields["PermissionsEdit"]
	field.Nillable = nil
	definition.Fields["PermissionsEdit"] = field
	if !RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("stale object permission checkbox metadata was not refreshed")
	}
	field = definition.Fields["PermissionsEdit"]
	if field.Required || field.Nillable == nil || *field.Nillable || RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("object permission checkbox repair did not stabilize")
	}
	field.Required = true
	definition.Fields["PermissionsEdit"] = field
	applyObjectPermissionCheckboxFacets(&definition)
	if !definition.Fields["PermissionsEdit"].Required || objectPermissionCheckboxSchemaNeedsRepair(definition) {
		t.Fatal("explicit object permission requiredness was changed or marked stale")
	}
}
