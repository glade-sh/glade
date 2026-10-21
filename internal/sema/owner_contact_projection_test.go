package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestUserModeOwnerContactProjectionAnonymous(t *testing.T) {
	// Owned native observations accept these four projected fields at these
	// source versions. The native replay also checks the returned values.
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Objects: ownerContactProjectionSchema([]string{"Group", "User"})}
			result := AnalyzeAnonymous(index, `List<Case> rows = [SELECT Id, Owner.FirstName, Owner.LastName, Owner.Phone, Owner.Email FROM Case WITH USER_MODE];`, api)
			if result.HasErrors() {
				t.Fatalf("native-accepted owner projection rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestUserModeOwnerContactProjectionBoundaries(t *testing.T) {
	// These are unchanged local rejection controls, not additional native claims.
	for _, test := range []struct {
		name    string
		targets []string
		query   string
		wantErr bool
	}{
		{"rootQualified", []string{"Group", "User"}, "SELECT Case.Owner.FirstName FROM Case WITH USER_MODE", false},
		{"caseInsensitive", []string{"gRoUp", "uSeR"}, "SELECT owner.firstname FROM Case WITH USER_MODE", false},
		{"reversedTargets", []string{"User", "Group"}, "SELECT Owner.FirstName FROM Case WITH USER_MODE", false},
		{"missingLeaf", []string{"Group", "User"}, "SELECT Owner.MissingSynthetic__c FROM Case WITH USER_MODE", true},
		{"otherUserLeaf", []string{"Group", "User"}, "SELECT Owner.TimeZoneSidKey FROM Case WITH USER_MODE", true},
		{"otherTarget", []string{"Group", "Contact"}, "SELECT Owner.FirstName FROM Case WITH USER_MODE", true},
		{"extraTarget", []string{"Group", "User", "Contact"}, "SELECT Owner.FirstName FROM Case WITH USER_MODE", true},
		{"onlyGroup", []string{"Group"}, "SELECT Owner.FirstName FROM Case WITH USER_MODE", true},
		{"repeatedGroup", []string{"Group", "Group"}, "SELECT Owner.FirstName FROM Case WITH USER_MODE", true},
		{"securityEnforced", []string{"Group", "User"}, "SELECT Owner.FirstName FROM Case WITH SECURITY_ENFORCED", true},
		{"noAccessMode", []string{"Group", "User"}, "SELECT Owner.FirstName FROM Case", true},
		{"systemMode", []string{"Group", "User"}, "SELECT Owner.FirstName FROM Case WITH SYSTEM_MODE", true},
		{"filter", []string{"Group", "User"}, "SELECT Id FROM Case WHERE Owner.FirstName = 'Atlas' WITH USER_MODE", true},
		{"order", []string{"Group", "User"}, "SELECT Id FROM Case WITH USER_MODE ORDER BY Owner.FirstName", true},
		{"aggregate", []string{"Group", "User"}, "SELECT COUNT(Owner.FirstName) FROM Case WITH USER_MODE", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			checker := newQuerySemanticsChecker(typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: "62.0"}, Objects: ownerContactProjectionSchema(test.targets)})
			diagnostics := checker.checkFile("OwnerContactProbe.cls", "List<Case> rows = ["+test.query+"];")
			gotErr := hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_FIELD")
			if gotErr != test.wantErr {
				t.Fatalf("field error = %t, want %t; diagnostics: %#v", gotErr, test.wantErr, diagnostics)
			}
		})
	}
}

func TestUserModeOwnerContactProjectionRequiresUserField(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: ownerContactProjectionSchema([]string{"Group", "User"})})
	// Supply an intentionally limited provider instead of falling through to the
	// standard User catalog, so the existence check is independently exercised.
	checker.providers[normalizeName("User")] = newSemaSObjectFieldMapProvider("", "User", []schema.Field{{Name: "Id", Type: "Id"}})
	diagnostics := checker.checkFile("OwnerContactProbe.cls", `List<Case> rows = [SELECT Owner.FirstName FROM Case WITH USER_MODE];`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_FIELD") {
		t.Fatalf("unprovided User field accepted: %#v", diagnostics)
	}
}

func ownerContactProjectionSchema(targets []string) []schema.Object {
	return []schema.Object{
		{Name: "Case", Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "OwnerId", Type: "Lookup", RelationshipName: "Owner", ReferenceTo: targets},
		}},
		{Name: "Group", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "Text"}}},
		{Name: "User", Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "FirstName", Type: "Text"},
			{Name: "LastName", Type: "Text"},
			{Name: "Phone", Type: "Phone"},
			{Name: "Email", Type: "Email"},
			{Name: "TimeZoneSidKey", Type: "Picklist"},
		}},
		{Name: "Contact", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "FirstName", Type: "Text"}}},
	}
}
