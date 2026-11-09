package sema

import (
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

// The native relationship capture establishes an Id child projection, not a
// general-purpose object schema. Child lookup must not register a root type.
func TestAPI68RelationshipProjectionStaysChildScoped(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: "68.0"}})
	if _, ok := checker.object("ContactPointOttSvc"); ok {
		t.Fatal("child-only projection exposed as a root object")
	}
	if _, ok := checker.childObjectForRelationship("Account", "ContactPointOttSvcs"); !ok {
		t.Fatal("captured API 68 relationship is missing")
	}
	if _, ok := checker.object("ContactPointOttSvc"); ok {
		t.Fatal("relationship lookup registered a root object")
	}
}
