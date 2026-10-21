package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A project object with a lookup to a standard parent adds a child
// relationship to that parent. The parent must keep its standard fields, so a
// named constructor argument such as Account(Name = ...) still resolves.
func TestAnonymousStandardParentKeepsStandardFieldsWithChildRelationship(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{SourceAPIVersion: "67.0"},
		Objects: []schema.Object{{
			Name: "Child__c",
			Fields: []schema.Field{{
				Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}, ChildRelationshipName: "Children",
			}},
		}},
	}
	for _, source := range []string{
		"insert new Account(Name = 'x', Industry = 'Technology');",
		"Account a = new Account(); a.Name = 'x'; List<Child__c> kids = a.Children;",
	} {
		if result := AnalyzeAnonymous(index, source, "67.0"); result.HasErrors() {
			t.Errorf("%q: %v", source, result.Diagnostics)
		}
	}
}
