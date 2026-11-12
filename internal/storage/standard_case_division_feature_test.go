package storage

import "testing"

// NATIVE Case.Division is present with the Division scratch feature
// internal/apextest/testdata/conformance/standard_case_field_features_division/org.tsv:38-49 anonymous 62.0 67.0
// NATIVE Case.Division is absent in an org without the feature
// internal/apextest/testdata/conformance/standard_case_field_features_none/org.tsv:38-49 anonymous 62.0 67.0
// The field carries the catalog facets: restricted picklist, neither createable
// nor updateable (assignment does not compile, a put throws).
func TestStandardCaseDivisionFollowsFeature(t *testing.T) {
	definition := ObjectDefinition{APIName: "Case"}
	for _, row := range []struct {
		name     string
		features []string
		present  bool
	}{
		{"baseline", nil, false},
		{"division", []string{"Division"}, true},
		{"return-to-baseline", nil, false},
		{"other feature", []string{"LiveAgent"}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			EnsureStandardObjectFieldsForFeatures(&definition, row.features)
			name, present := ResolveFieldName(definition, "", "Division")
			if present != row.present {
				t.Fatalf("Division membership=%t, want %t", present, row.present)
			}
			if present {
				field := definition.Fields[name]
				if field.Type != FieldPicklist || FieldFlagValue(field.Createable, true) || FieldFlagValue(field.Updateable, true) {
					t.Fatalf("Division facets: %+v", field)
				}
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("steady-state overlay requests repair")
			}
		})
	}
	if !StandardFieldAssignmentReadOnly("Case", "Division") {
		t.Fatal("Division assignment must be read-only")
	}
	if !StandardFeatureEnabledFieldDynamicPutReadOnly("Case", "Division") || StandardFeatureEnabledFieldDynamicPutReadOnly("Case", "Subject") {
		t.Fatal("dynamic put read-only decision")
	}
}
