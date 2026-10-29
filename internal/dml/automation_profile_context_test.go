package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native match, opposite-profile, wrong-user and restoration observations:
// internal/apextest/testdata/conformance/flow_profile_context/org.tsv:2-5.
func TestAutomationFormulaUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	const standardUser storage.ID = "005000000000002AAA"
	const standardProfile storage.ID = "00e000000000002AAA"
	const adminProfile storage.ID = "00e000000000001AAA"
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		storage.DefaultUserID: {ID: storage.DefaultUserID, Fields: map[string]storage.Value{"ProfileId": storage.IDValue(adminProfile)}},
		standardUser:          {ID: standardUser, Fields: map[string]storage.Value{"ProfileId": storage.IDValue(standardProfile)}},
	}}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		adminProfile:    {ID: adminProfile, Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}},
		standardProfile: {ID: standardProfile, Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}},
	}}
	definition := storage.ObjectDefinition{APIName: "FlowProfileRecord__c", Fields: map[string]storage.Field{
		"Name":      {APIName: "Name", Type: storage.FieldString},
		"Marked__c": {APIName: "Marked__c", Type: storage.FieldBoolean},
	}}
	engine := NewEngine(&org)
	for _, user := range []storage.ID{"", standardUser, storage.DefaultUserID} {
		engine.UserID = user
		profile := "System Administrator"
		if user == standardUser {
			profile = "Standard User"
		}
		for _, expression := range []string{
			fmt.Sprintf(`$Profile.Name = "%s"`, profile),
			fmt.Sprintf(`$Profile.Name <> "%s"`, profile),
			fmt.Sprintf(`LEFT($User.Id, 15) = "%s"`, string(engine.systemUserID())[:15]),
			`$User.Id = ""`,
			`Name = "ordinary"`,
		} {
			want := expression != `$User.Id = ""` && expression != fmt.Sprintf(`$Profile.Name <> "%s"`, profile)
			record := storage.Record{ID: "a00000000000001AAA", Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("ordinary")}}
			frame := &flowFrame{records: map[string]storage.Record{flowFrameKey("$Record"): record}}
			routes := map[string]func() (bool, bool){
				"entry": func() (bool, bool) {
					return engine.evaluateFlowRule(storage.FlowRule{Formula: expression}, record, definition)
				},
				"branch": func() (bool, bool) {
					return engine.evaluateFlowBranch(storage.FlowBranch{Formula: expression}, record, definition, false)
				},
				"decision": func() (bool, bool) { return engine.evaluateFlowDecisionFormula(expression, record, definition, frame) },
				"resource": func() (bool, bool) {
					value, ok := engine.flowContextFormulaValue(expression, frame)
					return value.Boolean, ok && value.Kind == storage.ValueBoolean
				},
				"workflow": func() (bool, bool) {
					object := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{record.ID: record}}
					object.Definition.WorkflowRules = []storage.WorkflowRule{{Name: "Context", Active: true, Formula: expression, FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Mark", Field: "Marked__c", LiteralValue: "true"}}}}
					org.Objects[definition.APIName] = object
					_, err := engine.applyWorkflowFieldUpdates(definition.APIName, record.ID)
					value := org.Objects[definition.APIName].Records[record.ID].Fields["Marked__c"]
					return value.Boolean, err == nil
				},
			}
			for route, evaluate := range routes {
				t.Run(fmt.Sprintf("%s/%s/%s", user, route, expression), func(t *testing.T) {
					got, ok := evaluate()
					if !ok || got != want {
						t.Fatalf("got %v, supported=%v; want %v", got, ok, want)
					}
				})
			}
		}
	}
}
