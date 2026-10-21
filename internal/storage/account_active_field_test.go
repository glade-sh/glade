package storage

import "testing"

func TestAccountActiveFieldProfileAndRefresh(t *testing.T) {
	definition, ok := StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("Account definition missing")
	}
	EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	if _, exists := ResolveFieldName(definition, "", "IsActive"); exists {
		t.Fatal("Account active field leaked into the existing person-account profile")
	}
	EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts", "HealthCloudAddOn"})
	for _, refresh := range []func(){
		func() {},
		func() { EnsureStandardObjectFields(&definition) },
		func() {
			delete(definition.Fields, "IsActive")
			if !StandardObjectFieldsNeedWrite(definition) || !RefreshStandardObjectFieldOverlay(&definition) {
				t.Fatal("missing enabled field did not request an overlay repair")
			}
		},
	} {
		refresh()
		field, exists := definition.Fields["IsActive"]
		if !exists || field.Type != FieldBoolean || field.DisplayType != "BOOLEAN" || field.DefaultValue != "false" ||
			field.Createable == nil || !*field.Createable || field.Updateable == nil || !*field.Updateable ||
			field.Nillable == nil || *field.Nillable || field.DefaultedOnCreate == nil || !*field.DefaultedOnCreate {
			t.Fatalf("active field lost its native shape: %+v", field)
		}
	}
	EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	if _, exists := ResolveFieldName(definition, "", "IsActive"); exists || StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("disabled active field remained in the overlay")
	}
}
