package automation

import "testing"

func TestFlowRecordVariableAssignmentKeepsTriggeringRecordSource(t *testing.T) {
	variables := map[string]flowVariableXML{
		"pending": {DataType: "SObject", ObjectType: "Account"},
		"rows":    {DataType: "SObject", ObjectType: "Account", IsCollection: true},
		"count":   {DataType: "Number"},
		"text":    {DataType: "String"},
	}
	for _, test := range []struct{ name, target, source, operator, want string }{
		{"record-id", "pending.Id", "$Record.Id", "Assign", "$Record.Id"},
		{"record-name", "pending.Name", "$Record.Name", "Assign", "$Record.Name"},
		{"numeric-control", "count", "$Record.Total__c", "Assign", "$Record.Total__c"},
		{"undeclared-control", "unknown.Id", "$Record.Id", "Assign", "Id"},
		{"collection-control", "rows.Id", "$Record.Id", "Assign", "Id"},
		{"nested-target-control", "pending.Parent__r.Id", "$Record.Id", "Assign", "Id"},
		{"nested-source-control", "pending.Id", "$Record.Parent__r.Id", "Assign", "Parent__r.Id"},
		{"primitive-target-control", "text.Id", "$Record.Id", "Assign", "Id"},
		{"operator-control", "pending.Name", "$Record.Name", "Add", "Name"},
		{"prior-control", "pending.Name", "$Record__Prior.Name", "Assign", "$Record__Prior.Name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assignment, ok := modeledFlowAssignment(flowAssignmentXML{Name: "PreparePending"}, flowAssignmentItemXML{
				AssignToReference: test.target, Operator: test.operator,
				Value: flowValueXML{ElementReference: test.source},
			}, nil, variables, nil)
			if !ok || assignment.SourceField != test.want {
				t.Fatalf("source=%q, modeled=%t; want %q", assignment.SourceField, ok, test.want)
			}
		})
	}
}
