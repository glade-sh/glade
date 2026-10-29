package schema

import "testing"

// A custom Address field expands into its eight native components in native
// describe order (testdata/conformance/custom_address_fields/describe.tsv).
func TestCustomAddressFieldExpandsComponents(t *testing.T) {
	objects := map[string]*Object{"Account": {Name: "Account", Fields: []Field{
		{Name: "Home__c", Label: "Home", Type: "Address"},
		{Name: "Spot__c", Label: "Spot", Type: "Location"},
	}}}
	addCustomAddressComponentFields(objects)
	fields := objects["Account"].Fields
	want := []string{"Home__Street__s", "Home__City__s", "Home__PostalCode__s", "Home__StateCode__s", "Home__CountryCode__s", "Home__Latitude__s", "Home__Longitude__s", "Home__GeocodeAccuracy__s"}
	if len(fields) != 2+len(want) {
		t.Fatalf("fields = %+v", fields)
	}
	for i, name := range want {
		field := fields[2+i]
		if field.Name != name || field.CompoundFieldName != "Home__c" || field.Createable == nil || !*field.Createable {
			t.Fatalf("component %d = %+v", i, field)
		}
	}
	if got := fields[2+3]; got.Label != "Home (State/Province)" || got.PicklistController != "Home__CountryCode__s" || !got.RestrictedPicklist {
		t.Fatalf("StateCode = %+v", got)
	}
	compound := fields[0]
	if *compound.Createable || *compound.Updateable || !*compound.Filterable || *compound.Sortable || *compound.Groupable {
		t.Fatalf("compound facets = %+v", compound)
	}
	if fields[1].CompoundFieldName != "" {
		t.Fatalf("Location field changed: %+v", fields[1])
	}
}
