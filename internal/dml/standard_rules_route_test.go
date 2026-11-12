package dml_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func standardRuleOrg(t *testing.T, object, matchObject, matchName string) *storage.OrgState {
	t.Helper()
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, object)
	storage.EnsureStandardObject(&org, matchObject)
	state := org.Objects[object]
	state.Definition.DuplicateRules = append(state.Definition.DuplicateRules, storage.DuplicateRule{
		Name: "Standard", Object: object, Active: true, ActionOnInsert: "Block", ActionOnUpdate: "Allow", AlertText: "duplicate",
		MatchRules: []storage.MatchingRule{{Object: matchObject, Name: matchName, Status: "Active"}},
	})
	org.Objects[object] = state
	return &org
}

func insertRow(engine dml.Engine, object string, fields map[string]string) dml.Result {
	record := storage.Record{Object: object, Fields: map[string]storage.Value{}}
	for k, v := range fields {
		record.Fields[k] = storage.StringValue(v)
	}
	return engine.Insert([]storage.Record{record})[0]
}

func blocked(r dml.Result) bool { return !r.Success && r.StatusCode == "DUPLICATES_DETECTED" }

// The standard Contact rule through the shared DML path at both API versions.
//
// NATIVE phone number stands in for the email address when both first names agree
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:59 anonymous 67.0
// (and :5 anonymous 62.0).
// NATIVE a candidate without a first name matches on the email address, an
// existing record without one never matches a candidate that has one
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:63 anonymous 67.0
// (and :9 anonymous 62.0).
func TestStandardContactRuleDMLRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := standardRuleOrg(t, "Contact", "Contact", "Standard_Contact_Match_Rule_v1_1")
			org.APIVersion = api
			engine := dml.NewEngine(org)
			engine.Options.EnforceDuplicateRules = true
			seed := map[string]string{"FirstName": "Nathaniel", "LastName": "Whitfield", "Email": "nate@quartz.example", "Phone": "(503) 555-0142"}
			if r := insertRow(engine, "Contact", seed); !r.Success {
				t.Fatalf("seed: %+v", r)
			}
			cases := []struct {
				name   string
				fields map[string]string
				block  bool
			}{
				{"same phone, other email", map[string]string{"FirstName": "Nathaniel", "LastName": "Whitfield", "Email": "other@example.org", "Phone": "503-555-0142"}, true},
				{"no first name, same email", map[string]string{"LastName": "Whitfield", "Email": "nate@quartz.example"}, true},
				{"other first name, same email", map[string]string{"FirstName": "Rosalind", "LastName": "Whitfield", "Email": "nate@quartz.example"}, false},
				{"same phone, other last name", map[string]string{"FirstName": "Nathaniel", "LastName": "Abernathy", "Email": "x@example.org", "Phone": "(503) 555-0142"}, false},
				{"no first name, same phone only", map[string]string{"LastName": "Whitfield", "Email": "y@example.org", "Phone": "(503) 555-0142"}, false},
			}
			for _, tc := range cases {
				if r := insertRow(engine, "Contact", tc.fields); blocked(r) != tc.block {
					t.Errorf("%s: blocked=%v want %v (%+v)", tc.name, blocked(r), tc.block, r)
				}
			}
		})
	}
}

// The standard Account rule: the name and a city, or the website, a city and a
// street.
//
// NATIVE Name path and Website path
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:55 anonymous 67.0
// (and :2 anonymous 62.0).
// NATIVE a name that differs in one word scores below 100
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:57 anonymous 67.0.
func TestStandardAccountRuleDMLRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := standardRuleOrg(t, "Account", "Account", "Standard_Account_Match_Rule_v1_0")
			org.APIVersion = api
			engine := dml.NewEngine(org)
			engine.Options.EnforceDuplicateRules = true
			seed := map[string]string{"Name": "Acme Widgets", "BillingCity": "Springfield", "BillingStreet": "100 Main St", "BillingPostalCode": "62701", "Website": "acmewidgets.example.com"}
			if r := insertRow(engine, "Account", seed); !r.Success {
				t.Fatalf("seed: %+v", r)
			}
			if r := insertRow(engine, "Account", map[string]string{"Name": "Acme Widgets", "BillingCity": "Springfield", "BillingStreet": "9 Elm Ave"}); !blocked(r) {
				t.Errorf("name and city must match: %+v", r)
			}
			if r := insertRow(engine, "Account", map[string]string{"Name": "Acme Widgets", "BillingStreet": "100 Main St"}); blocked(r) {
				t.Errorf("a name with a street and no city or ZIP must not match: %+v", r)
			}
			if r := insertRow(engine, "Account", map[string]string{"Name": "Zenith Corp", "BillingCity": "Springfield"}); blocked(r) {
				t.Errorf("another name must not match: %+v", r)
			}
		})
	}
}

// A person account is checked against the Person Account rules only, a
// business account against the Account rules.
//
// NATIVE a Person Account rule matches person accounts
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv:93 anonymous 67.0
// (and :38 anonymous 62.0). findDuplicates on a person account with no active
// Person Account rule fails with "No active duplicate rules are defined for
// the Account object type" (RUN-FACTS.txt).
func TestPersonAccountRulesSelectedByRecord(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Definition.DuplicateRules = []storage.DuplicateRule{
		{Name: "Business", Object: "Account", Active: true},
		{Name: "Person", Object: "Account", Subtype: "PersonAccount", Active: true},
	}
	org.Objects["Account"] = state
	person := storage.Record{Object: "Account", Fields: map[string]storage.Value{
		"IsPersonAccount": storage.BooleanValue(true), "FirstName": storage.StringValue("Nathaniel"), "PersonEmail": storage.StringValue("nate@quartz.example"),
	}}
	business := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}
	if rules := dml.ActiveDuplicateRulesForRecord(&org, "Account", person); len(rules) != 1 || rules[0].Name != "Person" {
		t.Fatalf("person account rules: %+v", rules)
	}
	if rules := dml.ActiveDuplicateRulesForRecord(&org, "Account", business); len(rules) != 1 || rules[0].Name != "Business" {
		t.Fatalf("business account rules: %+v", rules)
	}
}
