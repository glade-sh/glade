package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Start filters with "or" or numbered logic match when any group matches
// (native rows: internal/apextest/testdata/conformance/flow_logic).
func TestFlowStartCriteriaGroupsMatchAsAlternatives(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["Size__c"] = storage.Field{APIName: "Size__c", Type: storage.FieldInteger}
	criteria := []storage.WorkflowCriteriaItem{
		{Field: "Name", Operation: "equals", Value: "Acme"},
		{Field: "Size__c", Operation: "LessThan", Value: "5"},
		{Field: "Name", Operation: "StartsWith", Value: "Ac"},
	}
	engine := NewEngine(&org)
	for _, tc := range []struct {
		name   string
		groups [][]int
		record storage.Record
		want   bool
	}{
		{"and_all", nil, account("Acme", 3), true},
		{"and_one_fails", nil, account("Acme", 9), false},
		{"or_second_only", [][]int{{0}, {1}, {2}}, account("Zed", 3), true},
		{"or_none", [][]int{{0}, {1}, {2}}, account("Zed", 9), false},
		{"custom_first_alone", [][]int{{0}, {1, 2}}, account("Acme", 9), true},
		{"custom_pair_needs_both", [][]int{{0}, {1, 2}}, account("Zed", 3), false},
		{"custom_pair_both", [][]int{{0}, {1, 2}}, account("Acx", 3), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := storage.FlowRule{Name: "R", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: criteria, CriteriaGroups: tc.groups}
			got, ok := engine.evaluateFlowRule(rule, tc.record, object.Definition)
			if !ok || got != tc.want {
				t.Fatalf("matches=%v ok=%v, want %v", got, ok, tc.want)
			}
		})
	}
}

func account(name string, size int64) storage.Record {
	return storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name), "Size__c": storage.IntegerValue(size)}}
}
