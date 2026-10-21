package storage

import "testing"

func TestAccountTeamReferenceOverlayRefreshesCachedDefinitions(t *testing.T) {
	for _, relationship := range []string{"Account", "User"} {
		t.Run(relationship, func(t *testing.T) {
			definition, ok := StandardObjectDefinition("AccountTeamMember")
			if !ok {
				t.Fatal("missing standard definition")
			}
			name := relationship + "Id"
			field := definition.Fields[name]
			field.ReferenceTo, field.RelationshipName = nil, ""
			definition.Fields[name] = field
			if !StandardObjectFieldsNeedWrite(definition) || !RefreshStandardObjectFieldOverlay(&definition) {
				t.Fatal("cached overlay did not request missing native reference metadata")
			}
			field = definition.Fields[name]
			if len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != relationship || field.RelationshipName != relationship {
				t.Fatalf("reference metadata was not restored: %+v", field)
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("repaired overlay still requests writes")
			}
		})
	}
}
