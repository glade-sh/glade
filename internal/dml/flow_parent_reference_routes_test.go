package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowParentTargetExplicitAndNamedResourceRoutes(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
	}}
	for _, route := range []struct {
		name, reference string
		parent          bool
	}{
		{"trigger_parent", "$Record.Account", true},
		{"record_parent", "Record.Account", true},
		{"named_record", "Account", false},
		{"padded_named_record", " Account ", false},
		{"named_lookup", "Account.Name", false},
		{"nested_parent_boundary", "$Record.Account.Name", false},
	} {
		t.Run(route.name, func(t *testing.T) {
			object, parent := engine.flowParentRelationshipObject(route.reference, definition)
			if parent != route.parent || parent && object != "Account" {
				t.Fatalf("reference=%q object=%q parent=%t; want explicit parent=%t", route.reference, object, parent, route.parent)
			}
		})
	}
}
