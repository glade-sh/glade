package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A standard object reached through a project child relationship must resolve
// fields absent from the catalog exactly as the same object does without the
// relationship.
func TestStandardParentWithChildRelationshipResolvesUnknownFieldsLikeLazyObject(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		project := typesys.ProjectInfo{SourceAPIVersion: version}
		withRelationship := typesys.Index{
			Project: project,
			Objects: []schema.Object{{
				Name: "Appl__c",
				Fields: []schema.Field{{
					Name: "Contact__c", Type: "Lookup", ReferenceTo: []string{"Contact"},
					RelationshipName: "Contact__r", ChildRelationshipName: "Appls",
				}},
			}},
		}
		lazy := typesys.Index{Project: project}
		// Real projects also extend the standard parent with custom fields,
		// which loads as a partial object after the child that points at it.
		partialParent := withRelationship
		partialParent.Objects = append(append([]schema.Object{}, withRelationship.Objects...), schema.Object{
			Name: "Contact", Partial: true,
			Fields: []schema.Field{{Name: "Local__c", Type: "Text"}},
		})
		// Each case runs on the lazy object and on a Contact that gained a
		// child relationship. Both must agree.
		cases := []struct{ name, source string }{
			{"namespaced read", "Contact c = new Contact(); String s = c.pkgns__Extra__c;"},
			{"namespaced constructor", "Contact c = new Contact(pkgns__Extra__c = 'x');"},
			{"custom suffix constructor", "Contact c = new Contact(Extra__c = 'x');"},
			{"missing ordinary read", "Contact c = new Contact(); Object s = c.NotInCatalogXyz;"},
			{"missing ordinary constructor", "Contact c = new Contact(NotInCatalogXyz = 'x');"},
		}
		for _, tc := range cases {
			want := AnalyzeAnonymous(lazy, tc.source, version).HasErrors()
			got := AnalyzeAnonymous(withRelationship, tc.source, version)
			if got.HasErrors() != want {
				t.Errorf("%s %s: lazy errors=%v relationship errors=%v: %v", version, tc.name, want, got.HasErrors(), got.Diagnostics)
			}
		}
		for _, field := range []string{"pkgns__Extra__c", "NotInCatalogXyz"} {
			source := "Appl__c rec = new Appl__c(); Object s = rec.Contact__r." + field + ";"
			if got := AnalyzeAnonymous(partialParent, source, version); got.HasErrors() {
				t.Errorf("%s partial parent path %s: %v", version, field, got.Diagnostics)
			}
		}
		for _, source := range []string{
			"Contact c = new Contact(); String s = c.pkgns__Extra__c;",
			"Contact c = new Contact(NotInCatalogXyz = 'x');",
		} {
			if got := AnalyzeAnonymous(partialParent, source, version); got.HasErrors() {
				t.Errorf("%s partial parent %q: %v", version, source, got.Diagnostics)
			}
		}
		// Standard fields still hydrate on a parent with a child relationship.
		control := AnalyzeAnonymous(withRelationship, "Contact c = new Contact(LastName = 'x'); String n = c.LastName; List<Appl__c> k = c.Appls;", version)
		if control.HasErrors() {
			t.Errorf("%s control: %v", version, control.Diagnostics)
		}
	}
}
