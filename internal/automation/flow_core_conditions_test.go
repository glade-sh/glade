package automation

import "testing"

func TestFlowCoreEntryFormulaSupportsLiteralOperatorsInCustomLogic(t *testing.T) {
	cases := []struct {
		name     string
		operator string
		value    flowValueXML
		formula  string
	}{
		{name: "equal", operator: "EqualTo", value: flowValueXML{StringValue: "ready"}, formula: `Text__c = "ready"`},
		{name: "not-equal", operator: "NotEqualTo", value: flowValueXML{StringValue: "blocked"}, formula: `Text__c != "blocked"`},
		{name: "greater-than", operator: "GreaterThan", value: flowValueXML{NumberValue: "5"}, formula: "Text__c > 5"},
		{name: "greater-than-or-equal", operator: "GreaterThanOrEqualTo", value: flowValueXML{NumberValue: "5"}, formula: "Text__c >= 5"},
		{name: "less-than", operator: "LessThan", value: flowValueXML{NumberValue: "5"}, formula: "Text__c < 5"},
		{name: "less-than-or-equal", operator: "LessThanOrEqualTo", value: flowValueXML{NumberValue: "5"}, formula: "Text__c <= 5"},
		{name: "contains", operator: "Contains", value: flowValueXML{StringValue: "go"}, formula: `CONTAINS(Text__c,"go")`},
		{name: "is-null", operator: "IsNull", value: flowValueXML{BooleanValue: "true"}, formula: "ISBLANK(Text__c) = true"},
		{name: "starts-with", operator: "StartsWith", value: flowValueXML{StringValue: "go"}, formula: `BEGINS(Text__c,"go")`},
		{name: "ends-with", operator: "EndsWith", value: flowValueXML{StringValue: "now"}, formula: `RIGHT(Text__c,LEN("now")) = "now"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := flowStartXML{
				DoesRequireRecordChangedToMeetCriteria: true,
				TriggerType:                            "RecordAfterSave",
				RecordTriggerType:                      "Update",
				FilterLogic:                            "1 OR 2",
				Filters: []flowFilterXML{
					{Field: "Name", Operator: "EqualTo", Value: flowValueXML{StringValue: "subject"}},
					{Field: "Text__c", Operator: tc.operator, Value: tc.value},
				},
			}
			got := flowCoreEntryFormula(start, nil, nil)
			want := `OR(Name = "subject",` + tc.formula + ")"
			if got != want {
				t.Fatalf("formula = %q, want %q", got, want)
			}
		})
	}
}

func TestFlowCoreEntryFormulaRejectsUnbackedOperatorAliases(t *testing.T) {
	cases := []struct {
		name     string
		operator string
		value    flowValueXML
	}{
		{name: "equals", operator: "Equals", value: flowValueXML{StringValue: "ready"}},
		{name: "notequals", operator: "Notequals", value: flowValueXML{StringValue: "ready"}},
		{name: "isblank", operator: "IsBlank", value: flowValueXML{BooleanValue: "true"}},
		{name: "empty", operator: "", value: flowValueXML{StringValue: "ready"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := flowStartXML{
				DoesRequireRecordChangedToMeetCriteria: true,
				TriggerType:                            "RecordAfterSave",
				RecordTriggerType:                      "Update",
				FilterLogic:                            "1 OR 2",
				Filters: []flowFilterXML{
					{Field: "Name", Operator: "EqualTo", Value: flowValueXML{StringValue: "subject"}},
					{Field: "Text__c", Operator: tc.operator, Value: tc.value},
				},
			}
			if got := flowCoreEntryFormula(start, nil, nil); got != "" {
				t.Fatalf("operator alias %q produced formula %q", tc.operator, got)
			}
		})
	}
}

func TestFlowCoreEntryFormulaPreservesCustomLogicForUpdate(t *testing.T) {
	start := flowStartXML{
		DoesRequireRecordChangedToMeetCriteria: true,
		TriggerType:                            "RecordAfterSave",
		RecordTriggerType:                      "Update",
		FilterLogic:                            "1 AND (2 OR 3)",
		Filters: []flowFilterXML{
			{Field: "Name", Operator: "EqualTo", Value: flowValueXML{StringValue: "entry-or-update"}},
			{Field: "Text__c", Operator: "Contains", Value: flowValueXML{StringValue: "go"}},
			{Field: "Number__c", Operator: "GreaterThan", Value: flowValueXML{NumberValue: "5"}},
		},
	}
	got := flowCoreEntryFormula(start, nil, nil)
	want := `OR(AND(Name = "entry-or-update",CONTAINS(Text__c,"go")),AND(Name = "entry-or-update",Number__c > 5))`
	if got != want {
		t.Fatalf("formula = %q, want %q", got, want)
	}
}
