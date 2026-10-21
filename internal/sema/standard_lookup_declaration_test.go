package sema

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestStandardLookupDeclarationsRetainCatalogRelationships(t *testing.T) {
	for _, name := range []string{"Account", "User"} {
		t.Run(name, func(t *testing.T) {
			object := schema.Object{Name: "AccountTeamMember", Fields: []schema.Field{{Name: name + "Id", Type: "Lookup"}}}
			field, ok := newSemaSObjectFieldProvider("", object).lookup(name + "Id")
			if !ok || field.RelationshipName != name || !reflect.DeepEqual(field.ReferenceTo, []string{name}) {
				t.Fatalf("native relationship %s; Glade field=%+v", name, field)
			}
		})
	}
}
