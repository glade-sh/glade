package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Formats observed natively at API 62.0 and 67.0 (flow_gaps P01-P03).
func TestFlowMergeFieldTextMatchesNativeRows(t *testing.T) {
	for name, tc := range map[string]struct {
		value storage.Value
		want  string
		ok    bool
	}{
		"date":     {storage.DateValue("2031-03-09"), "March 9, 2031", true},
		"decimal":  {storage.Value{Kind: storage.ValueDecimal, String: "1234.50"}, "1,234.5", true},
		"small":    {storage.Value{Kind: storage.ValueDecimal, String: "5.0"}, "5", true},
		"integer":  {storage.IntegerValue(1234567), "1,234,567", true},
		"negative": {storage.IntegerValue(-1000), "-1,000", true},
		"null":     {storage.NullValue(), "", true},
		"boolean":  {storage.BooleanValue(true), "true", true},
		"string":   {storage.StringValue("ERR"), "ERR", true},
		"datetime": {storage.DateTimeValue("2031-03-09T10:00:00.000Z"), "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := flowMergeFieldText(tc.value)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("flowMergeFieldText = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// The record Number/DateTime/Time merge rule is captured only at API 62.0 and
// 67.0 (flow_record_merge); every other version keeps the generic merge text.
func TestFlowRecordMergeRuleIsGatedToCapturedAPIVersions(t *testing.T) {
	org := testOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
				"When__c":   {APIName: "When__c", Type: storage.FieldDateTime},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	engine := NewEngine(&org)
	frame := newFlowFrame()
	frame.records["$record"] = storage.Record{Object: "Widget__c", Fields: map[string]storage.Value{
		"Amount__c": {Kind: storage.ValueDecimal, String: "1.23456", Decimal: "1.23456"},
		"When__c":   storage.DateTimeValue("2031-03-09T10:00:00.000Z"),
	}}
	for _, tc := range []struct {
		api, message, want string
		fault              bool
	}{
		{"62.0", "{!$Record.Amount__c}", "1.235", false},
		{"60.0", "{!$Record.Amount__c}", "1.23456", false},
		{"", "{!$Record.Amount__c}", "1.23456", false},
		{"60.0", "{!$Record.When__c}", "{!$Record.When__c}", false},
		{"62.0", "{!$Record.When__c}", "{!$Record.When__c}", true},
	} {
		got, fault := engine.renderFlowCustomErrorMessageForAPI(tc.message, frame, nil, nil, nil, tc.api)
		if got != tc.want || fault != tc.fault {
			t.Errorf("api %q %s = %q, %v; want %q, %v", tc.api, tc.message, got, fault, tc.want, tc.fault)
		}
	}
}
