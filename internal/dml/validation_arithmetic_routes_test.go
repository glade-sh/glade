package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native validation_rule_formula_semantics rows N014, N015 and N018-N020
// distinguish blank operands from division errors in custom validation rules.
func TestCustomValidationArithmeticInsertAndUpdateNativeRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, formula string
		blank, reject bool
	}{
		{"blank_division", "NumberValue__c / 2 < 1", true, false},
		{"zero_positive", "1 / NumberValue__c > 0", false, true},
		{"zero_less", "1 / NumberValue__c < 0", false, true},
		{"zero_negative", "-1 / NumberValue__c > 0", false, true},
		{"zero_undefined", "0 / NumberValue__c = 0", false, true},
	} {
		for _, route := range []string{"insert", "update"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				org := testOrg()
				account := org.Objects["Account"]
				account.Definition.Fields["NumberValue__c"] = storage.Field{APIName: "NumberValue__c", Type: storage.FieldDecimal}
				org.Objects["Account"] = account
				engine := NewEngine(&org)
				record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Arithmetic sample")}}
				if route == "update" {
					seed := engine.Insert([]storage.Record{record})
					if !seed[0].Success {
						t.Fatal(seed)
					}
					record.ID = seed[0].ID
				}
				account = org.Objects["Account"]
				account.Definition.ValidationRules = []storage.ValidationRule{{Name: "ArithmeticGate", Active: true, ErrorConditionFormula: tc.formula, ErrorMessage: "Arithmetic rejected"}}
				org.Objects["Account"] = account
				if !tc.blank {
					record.Fields["NumberValue__c"] = storage.DecimalValue("0")
				}
				var results []Result
				if route == "insert" {
					results = engine.Insert([]storage.Record{record})
				} else {
					results = engine.Update([]storage.Record{record})
				}
				if results[0].Success == tc.reject || (tc.reject && results[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION") {
					t.Fatalf("%s = %#v; native reject=%t", route, results, tc.reject)
				}
			})
		}
	}
}

func TestCustomValidationArithmeticAutomationUpdateNativeRoutes(t *testing.T) {
	for _, route := range []string{"workflow_update", "flow_update"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["NumberValue__c"] = storage.Field{APIName: "NumberValue__c", Type: storage.FieldDecimal}
			org.Objects["Account"] = account
			seedEngine := NewEngine(&org)
			seed := seedEngine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Automation sample"), "NumberValue__c": storage.DecimalValue("1")}}})
			if !seed[0].Success {
				t.Fatal(seed)
			}
			account = org.Objects["Account"]
			account.Definition.ValidationRules = []storage.ValidationRule{{Name: "ArithmeticGate", Active: true, ErrorConditionFormula: "1 / NumberValue__c < 0", ErrorMessage: "Arithmetic rejected"}}
			update := storage.WorkflowFieldUpdate{Name: "SetZero", Field: "NumberValue__c", LiteralValue: "0"}
			if route == "workflow_update" {
				account.Definition.WorkflowRules = []storage.WorkflowRule{{Name: "ZeroUpdate", Active: true, FieldUpdates: []storage.WorkflowFieldUpdate{update}}}
			} else {
				account.Definition.FlowRules = []storage.FlowRule{{Name: "ZeroUpdate", Active: true, TriggerType: "RecordAfterSave", FieldUpdates: []storage.WorkflowFieldUpdate{update}}}
			}
			org.Objects["Account"] = account
			engine := NewEngine(&org)
			var err error
			if route == "workflow_update" {
				_, err = engine.applyWorkflowFieldUpdates("Account", seed[0].ID)
			} else {
				_, err = engine.applyFlowFieldUpdates("Account", seed[0].ID)
			}
			if err == nil || !strings.Contains(err.Error(), "Arithmetic rejected") {
				t.Fatalf("native zero-divisor validation must reject the automation update: %v", err)
			}
		})
	}
}

func TestValidationArithmeticDoesNotChangeWorkflowAndFlowRoutes(t *testing.T) {
	org := testOrg()
	definition := org.Objects["Account"].Definition
	definition.Fields["NumberValue__c"] = storage.Field{APIName: "NumberValue__c", Type: storage.FieldDecimal}
	engine := NewEngine(&org)
	for _, tc := range []struct {
		name, formula string
		fields        map[string]storage.Value
		want          bool
	}{
		{"blank_operand", "NumberValue__c / 2 < 1", nil, true},
		{"zero_divisor", "1 / NumberValue__c < 0", map[string]storage.Value{"NumberValue__c": storage.DecimalValue("0")}, false},
	} {
		record := storage.Record{Object: "Account", Fields: tc.fields}
		for route, evaluate := range map[string]func() (bool, bool){
			"workflow": func() (bool, bool) {
				return evaluateWorkflowRule(storage.WorkflowRule{Formula: tc.formula}, record, definition, &org)
			},
			"flow_entry": func() (bool, bool) {
				return engine.evaluateFlowRule(storage.FlowRule{Formula: tc.formula}, record, definition)
			},
			"flow_branch": func() (bool, bool) {
				return engine.evaluateFlowBranch(storage.FlowBranch{Formula: tc.formula}, record, definition, false)
			},
		} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				got, supported := evaluate()
				if !supported || got != tc.want {
					t.Fatalf("value=%t supported=%t; unchanged=%t", got, supported, tc.want)
				}
			})
		}
	}
}

// Lookup rows L08, L14-L18 establish date-only blank equality rejection.
func TestLookupFilterBlankComparisonInsertAndUpdateNativeRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, operation string
		typ             storage.FieldType
		reject          bool
	}{
		{"blank_dates_equal", "equals", storage.FieldDate, true},
		{"blank_text_equal", "equals", storage.FieldString, false},
		{"blank_dates_not_equal", "notEqual", storage.FieldDate, true},
		{"blank_text_not_equal", "notEqual", storage.FieldString, true},
	} {
		for _, route := range []string{"insert", "update"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				org := testOrg()
				storage.EnsureStandardObject(&org, "Contact")
				account := org.Objects["Account"]
				account.Definition.Fields["Compare__c"] = storage.Field{APIName: "Compare__c", Type: tc.typ}
				org.Objects["Account"] = account
				contact := org.Objects["Contact"]
				contact.Definition.Fields["Compare__c"] = storage.Field{APIName: "Compare__c", Type: tc.typ}
				lookup := contact.Definition.Fields["AccountId"]
				lookup.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: "Account.Compare__c", ValueField: "$Source.Compare__c", Operation: tc.operation}}}
				contact.Definition.Fields["AccountId"] = lookup
				org.Objects["Contact"] = contact
				engine := NewEngine(&org)
				parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Blank target")}}})
				if !parent[0].Success {
					t.Fatal(parent)
				}
				record := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Blank source")}}
				if route == "update" {
					seed := engine.Insert([]storage.Record{record})
					if !seed[0].Success {
						t.Fatal(seed)
					}
					record.ID = seed[0].ID
				}
				record.Fields["AccountId"] = storage.IDValue(parent[0].ID)
				var results []Result
				if route == "insert" {
					results = engine.Insert([]storage.Record{record})
				} else {
					results = engine.Update([]storage.Record{record})
				}
				if results[0].Success == tc.reject || (tc.reject && results[0].StatusCode != "FIELD_FILTER_VALIDATION_EXCEPTION") {
					t.Fatalf("%s = %#v; native reject=%t", route, results, tc.reject)
				}
			})
		}
	}
}
