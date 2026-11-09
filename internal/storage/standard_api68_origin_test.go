package storage

import "testing"

// Project fields are captured before the standard overlay, even for an object
// whose complete API68 describe is supplied by the separate sidecar.
func TestStandardAPI68ConfiguredFieldOrigins(t *testing.T) {
	baseline, ok := StandardObjectDefinition("AssociatedLocationHistory")
	if !ok {
		t.Fatal("missing historical definition")
	}
	name, ok := ResolveFieldName(baseline, "", "Division")
	if !ok {
		t.Fatal("missing historical field")
	}
	for _, row := range []struct {
		name       string
		definition ObjectDefinition
		configured bool
	}{
		{"bare", ObjectDefinition{APIName: "AssociatedLocationHistory"}, false},
		{"configured", ObjectDefinition{APIName: "AssociatedLocationHistory", Fields: map[string]Field{name: baseline.Fields[name]}}, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			EnsureStandardObjectFieldsForFeatures(&row.definition, []string{"__glade_edition_developer"})
			configured, recorded := StandardAPI68ConfiguredField(row.definition, name)
			if !recorded || configured != row.configured {
				t.Fatalf("configured=%t recorded=%t, want %t/true", configured, recorded, row.configured)
			}
		})
	}
	legacy := ObjectDefinition{APIName: "AssociatedLocationHistory", Fields: map[string]Field{name: baseline.Fields[name]}}
	if _, recorded := StandardAPI68ConfiguredField(legacy, name); recorded {
		t.Fatal("unrecorded legacy definition acquired invented provenance")
	}
}
