package automation

import "testing"

// Only AND-combined IsNull filters are captured; every other filter shape on a
// triggering-record update keeps its earlier lowering.
func TestFlowTriggerUpdateIsNullFilterBoundaries(t *testing.T) {
	isNull := flowFilterXML{Field: "Src__c", Operator: "IsNull", Value: flowValueXML{BooleanValue: "false"}}
	cases := []struct {
		name   string
		update flowRecordUpdateXML
		want   bool
	}{
		{"isnull", flowRecordUpdateXML{Filters: []flowFilterXML{isNull}}, true},
		{"isnull and logic", flowRecordUpdateXML{Filters: []flowFilterXML{isNull, isNull}, FilterLogic: "and"}, true},
		{"isnull true", flowRecordUpdateXML{Filters: []flowFilterXML{{Field: "Src__c", Operator: "IsNull", Value: flowValueXML{BooleanValue: "true"}}}}, false},
		{"no filters", flowRecordUpdateXML{}, false},
		{"equal to", flowRecordUpdateXML{Filters: []flowFilterXML{{Field: "Src__c", Operator: "EqualTo"}}}, false},
		{"mixed", flowRecordUpdateXML{Filters: []flowFilterXML{isNull, {Field: "Src__c", Operator: "EqualTo"}}}, false},
		{"or logic", flowRecordUpdateXML{Filters: []flowFilterXML{isNull}, FilterLogic: "or"}, false},
	}
	for _, tc := range cases {
		if got := flowTriggerUpdateHasIsNullFilters(tc.update); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFlowTriggerUpdateGateTriggerBoundaries(t *testing.T) {
	mk := func(process, trigger, record string) flowXML {
		var raw flowXML
		raw.ProcessType = process
		raw.Start.TriggerType = trigger
		raw.Start.RecordTriggerType = record
		return raw
	}
	cases := []struct {
		name string
		raw  flowXML
		want bool
	}{
		{"before save create and update", mk("AutoLaunchedFlow", "RecordBeforeSave", "CreateAndUpdate"), true},
		{"before save update", mk("AutoLaunchedFlow", "RecordBeforeSave", "Update"), false},
		{"before save create", mk("AutoLaunchedFlow", "RecordBeforeSave", "Create"), false},
		{"after save create and update", mk("AutoLaunchedFlow", "RecordAfterSave", "CreateAndUpdate"), false},
		{"process builder", mk("Workflow", "RecordBeforeSave", "CreateAndUpdate"), false},
	}
	for _, tc := range cases {
		if got := flowTriggerUpdateGateApplies(tc.raw); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
