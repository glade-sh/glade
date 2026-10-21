package storage

import "testing"

func TestUserLicenseStatusRefreshesStaleDescribeFacets(t *testing.T) {
	definition, ok := StandardObjectDefinition("UserLicense")
	if !ok {
		t.Fatal("UserLicense definition missing")
	}
	EnsureStandardObjectFields(&definition)
	field := definition.Fields["Status"]
	field.Type = FieldString
	field.DisplayType = "STRING"
	field.Nillable = BoolFlag(true)
	definition.Fields["Status"] = field
	if !RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("stale Status facets were not refreshed")
	}
	field = definition.Fields["Status"]
	if field.Type != FieldPicklist || field.DisplayType != "PICKLIST" || field.Nillable == nil || *field.Nillable {
		t.Fatalf("Status facets = %#v", field)
	}
	if RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("an unchanged describe must not need another refresh")
	}
}
