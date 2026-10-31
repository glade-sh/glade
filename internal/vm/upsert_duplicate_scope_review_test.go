package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured rule shapes and fanout preserve the earlier local upsert behavior.
func TestUpsertDuplicateUncapturedScopeVMRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, scenario := range []string{"other_field", "multi_field", "blank_match", "two_candidates", "two_rules"} {
			t.Run(api+"/"+scenario, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				object := org.Objects["Contact"]
				object.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
				org.Objects["Contact"] = object
				seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Scope"), "Email": storage.StringValue("scope@example.invalid"), "Phone": storage.StringValue("5551234567"), "RouteKey__c": storage.StringValue("seed")}}
				if scenario == "blank_match" {
					delete(seed.Fields, "Email")
				}
				rows := []storage.Record{seed}
				if scenario == "two_candidates" {
					second := seed.Clone()
					second.Fields["RouteKey__c"] = storage.StringValue("seed2")
					rows = append(rows, second)
				}
				engine := dml.NewEngine(&org)
				for _, r := range engine.Insert(rows) {
					if !r.Success {
						t.Fatalf("seed: %+v", r)
					}
				}
				match := storage.MatchingRule{Object: "Contact", Name: "Synthetic_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
				switch scenario {
				case "other_field":
					match.Items[0].Field = "Phone"
				case "multi_field":
					match.Items = append(match.Items, storage.MatchingRuleItem{Field: "Phone", Method: "Exact", BlankBehavior: "NullNotAllowed"})
				case "blank_match":
					match.Items[0].BlankBehavior = "MatchBlanks"
				}
				rule := storage.DuplicateRule{Name: "ScopeRule", Object: "Contact", Active: true, ActionOnInsert: "Block", MatchRules: []storage.MatchingRule{match}}
				report := scenario == "two_candidates" || scenario == "two_rules"
				if report {
					rule.ActionOnInsert = "Allow"
					rule.OperationsOnInsert = []string{"Report"}
				}
				object = org.Objects["Contact"]
				object.Definition.DuplicateRules = []storage.DuplicateRule{rule}
				if scenario == "two_rules" {
					rule.Name = "SecondRule"
					object.Definition.DuplicateRules = append(object.Definition.DuplicateRules, rule)
				}
				org.Objects["Contact"] = object
				email := ", Email='scope@example.invalid'"
				if scenario == "blank_match" {
					email = ""
				}
				source := "Contact row=new Contact(LastName='Scope', Phone='5551234567', RouteKey__c='new'" + email + "); Database.UpsertResult saved=Database.upsert(row, Contact.RouteKey__c, false); System.assertEquals(true,saved.isSuccess());"
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				if report {
					sets := len(machine.Org.Objects["DuplicateRecordSet"].Records)
					items := len(machine.Org.Objects["DuplicateRecordItem"].Records)
					if sets != 0 || items != 0 {
						t.Fatalf("uncaptured fanout: %d sets and %d items", sets, items)
					}
				}
			})
		}
	}
}
