package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Partial declarations establish identity; inferred placeholders alone do not.
// A declared but unavailable dependency must remain open to its own names.
func TestFieldSetObjectIdentityBoundary(t *testing.T) {
	declared := schema.Object{Name: "Present__c", Partial: true}
	idx := typesys.Index{Objects: []schema.Object{declared, {Name: "Missing__c", Partial: true}, {Name: "vendor__Remote__c", Partial: true}}}
	model := buildSemaTypeMemberView(idx)
	a := NewAnalyzer()
	a.queryDeclaredObjects = []schema.Object{declared}
	a.deps = map[string]bool{"vendor": true}
	a.opaqueDeps = map[string]bool{"vendor": true}
	for _, tc := range []struct {
		name, receiver string
		known          bool
	}{
		{"declared_partial", "SObjectType.Present__c.FieldSets.Main", true},
		{"inferred_missing", "SObjectType.Missing__c.FieldSets.Main", false},
		{"opaque_dependency", "SObjectType.vendor__Remote__c.FieldSets.Main", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := semaQualifiedCallReceiverKnown(a, "", tc.receiver, newIRSemaScope(nil), model); got != tc.known {
				t.Fatalf("receiver known=%v; want %v", got, tc.known)
			}
		})
	}
}
