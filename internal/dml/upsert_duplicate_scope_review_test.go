package dml_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Shipping-preservation controls for uncaptured upsert rule shapes. These do
// not claim Salesforce parity; the native row covers one nonblank exact email.
func TestUpsertDuplicateUncapturedRuleShapesPreserveShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"standard", "multi_field", "blank_match", "other_field"} {
			for _, operation := range []string{"insert", "update"} {
				t.Run(api+"/"+shape+"/"+operation, func(t *testing.T) {
					org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
					storage.EnsureStandardObject(&org, "Contact")
					object := org.Objects["Contact"]
					object.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
					org.Objects["Contact"] = object
					engine := dml.NewEngine(&org)
					seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Shape"), "Email": storage.StringValue("same@example.invalid"), "Phone": storage.StringValue("5551234567"), "RouteKey__c": storage.StringValue("seed")}}
					if shape == "blank_match" {
						delete(seed.Fields, "Email")
					}
					inserted := engine.Insert([]storage.Record{seed})
					if len(inserted) != 1 || !inserted[0].Success {
						t.Fatalf("seed: %+v", inserted)
					}
					var targetID storage.ID
					if operation == "update" {
						target := seed.Clone()
						target.Fields["RouteKey__c"] = storage.StringValue("target")
						target.Fields["Email"] = storage.StringValue("other@example.invalid")
						target.Fields["Phone"] = storage.StringValue("5559999999")
						r := engine.Insert([]storage.Record{target})
						if len(r) != 1 || !r[0].Success {
							t.Fatalf("target: %+v", r)
						}
						targetID = r[0].ID
					}
					item := storage.MatchingRuleItem{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}
					match := storage.MatchingRule{Object: "Contact", Name: "Synthetic_Exact", Status: "Active", Items: []storage.MatchingRuleItem{item}}
					switch shape {
					case "standard":
						match = storage.MatchingRule{Object: "Contact", Name: "Standard_Contact_Match_Rule_v1_1", Status: "Active"}
					case "multi_field":
						match.Items = append(match.Items, storage.MatchingRuleItem{Field: "Phone", Method: "Exact", BlankBehavior: "NullNotAllowed"})
					case "blank_match":
						match.Items[0].BlankBehavior = "MatchBlanks"
					case "other_field":
						match.Items[0].Field = "Phone"
					}
					object = org.Objects["Contact"]
					object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "ShapeRule", Object: "Contact", Active: true, ActionOnInsert: "Block", ActionOnUpdate: "Block", MatchRules: []storage.MatchingRule{match}}}
					org.Objects["Contact"] = object
					engine.Options.EnforceDuplicateRules = true
					engine.Options.EnforceUpsertIDInsertDuplicateRules = true
					engine.Options.UpsertChangedExactDuplicatesOnly = true
					row := seed.Clone()
					row.Fields["RouteKey__c"] = storage.StringValue("new")
					if operation == "update" {
						row.Fields["RouteKey__c"] = storage.StringValue("target")
						row.ID = targetID
					}
					got := engine.UpsertWithExternalID([]storage.Record{row}, "RouteKey__c")
					if len(got) != 1 || !got[0].Success {
						t.Fatalf("uncaptured %s %s should preserve shipping success: %+v", shape, operation, got)
					}
				})
			}
		}
	}
}

func TestUpsertDuplicateUncapturedReportFanoutPreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, scenario := range []string{"two_candidates", "two_rules"} {
			t.Run(api+"/"+scenario, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				object := org.Objects["Contact"]
				object.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
				org.Objects["Contact"] = object
				engine := dml.NewEngine(&org)
				seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Report"), "Email": storage.StringValue("same@example.invalid"), "RouteKey__c": storage.StringValue("seed")}}
				rows := []storage.Record{seed}
				if scenario == "two_candidates" {
					second := seed.Clone()
					second.Fields["RouteKey__c"] = storage.StringValue("seed2")
					rows = append(rows, second)
				}
				for _, r := range engine.Insert(rows) {
					if !r.Success {
						t.Fatalf("seed: %+v", r)
					}
				}
				match := storage.MatchingRule{Object: "Contact", Name: "Synthetic_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
				rule := storage.DuplicateRule{Name: "ReportOne", Object: "Contact", Active: true, ActionOnInsert: "Allow", OperationsOnInsert: []string{"Report"}, MatchRules: []storage.MatchingRule{match}}
				object = org.Objects["Contact"]
				object.Definition.DuplicateRules = []storage.DuplicateRule{rule}
				if scenario == "two_rules" {
					rule.Name = "ReportTwo"
					object.Definition.DuplicateRules = append(object.Definition.DuplicateRules, rule)
				}
				org.Objects["Contact"] = object
				engine.Options.EnforceDuplicateRules = true
				engine.Options.EnforceUpsertIDInsertDuplicateRules = true
				row := seed.Clone()
				row.Fields["RouteKey__c"] = storage.StringValue("new")
				got := engine.UpsertWithExternalID([]storage.Record{row}, "RouteKey__c")
				if len(got) != 1 || !got[0].Success {
					t.Fatalf("save: %+v", got)
				}
				if reports := engine.TakeDuplicateReports(); len(reports) != 0 {
					t.Fatalf("uncaptured fanout %s should preserve zero reports; got %d", scenario, len(reports))
				}
			})
		}
	}
}

// The native changed-email row starts with a nonblank prior value. Preserve
// the shipping update path when the matching key was missing or explicitly null.
func TestUpsertDuplicateUncapturedBlankPriorPreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, prior := range []string{"missing", "null"} {
			t.Run(api+"/"+prior, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				object := org.Objects["Contact"]
				object.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
				org.Objects["Contact"] = object
				engine := dml.NewEngine(&org)
				seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Shape"), "Email": storage.StringValue("same@example.invalid"), "RouteKey__c": storage.StringValue("seed")}}
				target := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Shape"), "RouteKey__c": storage.StringValue("target")}}
				if prior == "null" {
					target.ExplicitNulls = map[string]bool{"Email": true}
				}
				inserted := engine.Insert([]storage.Record{seed, target})
				if len(inserted) != 2 || !inserted[0].Success || !inserted[1].Success {
					t.Fatalf("seed: %+v", inserted)
				}
				match := storage.MatchingRule{Object: "Contact", Name: "Synthetic_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
				object = org.Objects["Contact"]
				object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "ShapeRule", Object: "Contact", Active: true, ActionOnUpdate: "Block", MatchRules: []storage.MatchingRule{match}}}
				org.Objects["Contact"] = object
				engine.Options.EnforceDuplicateRules = true
				engine.Options.UpsertChangedExactDuplicatesOnly = true
				row := storage.Record{Object: "Contact", ID: inserted[1].ID, Fields: map[string]storage.Value{"Email": storage.StringValue("same@example.invalid"), "RouteKey__c": storage.StringValue("target")}}
				got := engine.UpsertWithExternalID([]storage.Record{row}, "RouteKey__c")
				if len(got) != 1 || !got[0].Success {
					t.Fatalf("uncaptured blank prior %s should preserve shipping success: %+v", prior, got)
				}
			})
		}
	}
}
