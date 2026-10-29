package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's self-lookup result is in flow_relationship_id_target_boundary.
// The other cases pin the behavior at the v0.3.2 shipping base.
func TestFlowRelationshipIDUpdateTargetBoundary(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	definition := storage.ObjectDefinition{Fields: map[string]storage.Field{
		"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"OwnerId":  {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}}
	for _, source := range []struct {
		name, relationship, lookup string
		populated                  bool
	}{
		{"parentPopulated", "Parent.Id", "ParentId", true},
		{"parentEmpty", "Parent.Id", "ParentId", false},
		{"ownerPopulated", "Owner.Id", "OwnerId", true},
		{"ownerEmpty", "Owner.Id", "OwnerId", false},
	} {
		fields := map[string]storage.Value{}
		if source.populated {
			fields[source.lookup] = storage.IDValue(storage.ID("001000000000001"))
		}
		for _, target := range []struct {
			name  string
			field storage.Field
			want  bool
		}{
			{"sameLookup", storage.Field{APIName: source.lookup, Type: storage.FieldReference}, true},
			{"otherLookup", storage.Field{APIName: "Other__c", Type: storage.FieldReference}, true},
			{"id", storage.Field{APIName: "OtherId", Type: storage.FieldID}, true},
			{"boolean", storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean}, true},
			{"text", storage.Field{APIName: "Description", Type: storage.FieldString}, true},
		} {
			t.Run(source.name+"/"+target.name, func(t *testing.T) {
				value, ok := engine.flowRefsRelationshipIDValue(source.relationship, storage.Record{Fields: fields}, definition, target.field)
				if ok != target.want {
					t.Fatalf("resolved=%v, want %v (value=%#v)", ok, target.want, value)
				}
				if ok && !source.populated && value.Kind != storage.ValueNull {
					t.Fatalf("empty lookup value=%#v, want null", value)
				}
				if ok && source.populated && target.field.Type != storage.FieldString && value.Kind != storage.ValueID {
					t.Fatalf("value=%#v, want ID", value)
				}
			})
		}
	}
}
