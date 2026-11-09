package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce observations: ../apextest/testdata/conformance/q3semijoin/org.tsv.
func TestChildRelationshipSemiJoinCapturedShapes(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		valid       bool
	}{
		{"positive", "SELECT Id, (SELECT LastName FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account", true},
		{"negative", "SELECT Id, (SELECT LastName FROM Contacts WHERE Id NOT IN (SELECT ContactId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account", true},
		{"adjacent", "SELECT Id, (SELECT Name, StageName FROM Opportunities WHERE Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker')) FROM Account", true},
		{"or_rejected", "SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole) OR LastName = 'Example') FROM Account", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.query)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v; parse error: %v", tc.valid, err)
			}
		})
	}
}

func TestChildRelationshipSemiJoinOrNativeRejection(t *testing.T) {
	_, err := Parse("SELECT Id, (SELECT Id FROM Contacts WHERE Id IN (SELECT ContactId FROM OpportunityContactRole) OR LastName = 'Example') FROM Account")
	if err == nil || !strings.Contains(err.Error(), "Semi join sub-selects are not allowed with the 'OR' operator") {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

func TestQ3ConstructedChildSemiJoinRejectsNestedAndLimit(t *testing.T) {
	for _, kind := range []string{"nested", "limit"} {
		t.Run(kind, func(t *testing.T) {
			inner := Query{Object: "OpportunityContactRole", Fields: []string{"ContactId"}}
			if kind == "nested" {
				inner.Where = &Condition{Field: "OpportunityId", Op: "IN", Subquery: &Query{Object: "Opportunity", Fields: []string{"Id"}}}
			} else {
				inner.HasLimit = true
				inner.Limit = 1
			}
			child := Query{Object: "Contacts", Fields: []string{"Id"}, Where: &Condition{Field: "Id", Op: "IN", Subquery: &inner}}
			parent := Query{Object: "Account", Fields: []string{"Id"}, ChildQueries: []ChildQuery{{Relationship: "Contacts", Query: child}}}
			validationErr := validateSemiAntiQuery(parent)
			if validationErr == nil {
				t.Fatal("accepted invalid inner semi-join")
			}
			if _, err := Execute(storage.NewOrgState(), parent); err == nil || err.Error() != validationErr.Error() {
				t.Fatalf("constructed query execution error=%v; want %v", err, validationErr)
			}
		})
	}
}
