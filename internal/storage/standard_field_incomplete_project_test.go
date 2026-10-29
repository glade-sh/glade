package storage

import "testing"

// The token describes uncertainty, never a Salesforce feature. Low-level nil
// profiles remain explicit bare-org controls used by the native conformance
// suite. The project loader supplies the token for incomplete source trees.
func TestStandardFieldIncompleteProjectCatalogTransitions(t *testing.T) {
	unknown := []string{"__glade_unspecified_standard_field_shape"}
	for _, row := range []struct{ object, field string }{
		{"Account", "RecordTypeId"}, {"Contact", "RecordTypeId"}, {"Case", "RecordTypeId"},
		{"Account", "Division"}, {"Case", "HasCommentsUnreadByOwner"},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			def := ObjectDefinition{APIName: row.object}
			for _, shape := range []struct {
				name     string
				features []string
				present  bool
			}{
				{"unknown", unknown, true}, {"declared-bare", nil, false}, {"unknown-again", unknown, true},
			} {
				EnsureStandardObjectFieldsForFeatures(&def, shape.features)
				if _, got := ResolveFieldName(def, "", row.field); got != shape.present {
					t.Errorf("%s: membership=%t, want %t", shape.name, got, shape.present)
				}
				if row.field == "RecordTypeId" {
					found := false
					for _, relation := range def.Relations {
						found = found || relation.Field == "RecordTypeId"
					}
					if found != shape.present {
						t.Errorf("%s: effective RecordType relationship=%t, want %t", shape.name, found, shape.present)
					}
				}
				if StandardObjectFieldsNeedWrite(def) {
					t.Errorf("%s: completed overlay still needs repair", shape.name)
				}
				copy := def.Clone()
				if RefreshStandardObjectFieldOverlay(&copy) {
					t.Errorf("%s: warm refresh changed completed overlay", shape.name)
				}
			}
		})
	}
}

// Persisted runtime profiles must not make uncertainty sticky after a project
// gains an explicit scratch shape. Conversely, removing that declaration must
// recover the compatibility catalog without promoting it to configured schema.
func TestStandardFieldIncompleteProjectOrgTransitions(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Account")
	for _, shape := range []struct {
		name     string
		features []string
		present  bool
	}{
		{"unknown", []string{"__glade_unspecified_standard_field_shape"}, true},
		{"declared-bare", nil, false},
		{"unknown-again", []string{"__glade_unspecified_standard_field_shape"}, true},
	} {
		ApplyProjectOrgShape(&org, shape.features)
		def := org.Objects["Account"].Definition
		if _, got := ResolveFieldName(def, "", "RecordTypeId"); got != shape.present {
			t.Errorf("%s: membership=%t, want %t", shape.name, got, shape.present)
		}
		if StandardObjectFieldsNeedWrite(def) {
			t.Errorf("%s: overlay still needs repair", shape.name)
		}
	}
}

// The compile gate also consumes the catalog's relationship visitor directly,
// without first constructing an ObjectDefinition.
func TestStandardFieldIncompleteProjectRelationshipRoutes(t *testing.T) {
	// Account has a raw catalog relationship. Contact and Case synthesize
	// theirs from the effective reference fields (checked above); the raw
	// visitor is partial, and its consumers already fall back to that overlay.
	for _, object := range []string{"Account"} {
		for _, unknown := range []bool{false, true} {
			var features []string
			if unknown {
				features = []string{"__glade_unspecified_standard_field_shape"}
			}
			found := false
			VisitStandardObjectRelationships(object, features, func(relation Relationship) {
				if relation.Field == "RecordTypeId" {
					found = true
				}
			})
			if found != unknown {
				t.Errorf("%s unknown=%t: RecordType relationship=%t", object, unknown, found)
			}
		}
	}
}

// SOQL and fixture imports can add capabilities after the project overlay.
// Adding a capability is not a declaration that all other fields are absent.
func TestStandardFieldIncompleteProjectAdditiveCapabilities(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Account")
	ApplyOrgShape(&org, []string{"__glade_unspecified_standard_field_shape"})
	ApplyOrgShape(&org, []string{"PlatformCache"})
	if !org.HasFeature("PlatformCache") {
		t.Fatal("additive capability missing")
	}
	if _, present := ResolveFieldName(org.Objects["Account"].Definition, "", "RecordTypeId"); !present {
		t.Fatal("adding a capability closed the unknown project shape")
	}
}
