package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The same captured org-default value must reach the engine's formula callers.
func TestSetupFormulaOrganizationAutomationRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			t.Run(api+map[bool]string{true: "/true", false: "/false"}[enabled], func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.OrgID = "00D000000000mH2"
				def := storage.ObjectDefinition{APIName: "Prefs__c", Label: "Prefs", KeyPrefix: "a02", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference}, "Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
				org.Objects[def.APIName] = storage.ObjectState{Definition: def}
				engine := NewEngine(&org)
				inserted := engine.Insert([]storage.Record{{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Chosen"), "Flag__c": storage.BooleanValue(enabled)}}})
				if len(inserted) != 1 || !inserted[0].Success {
					t.Fatalf("setting insert: %+v", inserted)
				}
				record := storage.Record{ID: "a00000000000001", Object: "Example__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Example")}}
				target := storage.ObjectDefinition{APIName: record.Object, Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Marked__c": {APIName: "Marked__c", Type: storage.FieldBoolean}}}
				expression := "$Setup.Prefs__c.Flag__c"
				frame := &flowFrame{records: map[string]storage.Record{flowFrameKey("$Record"): record}}
				routes := map[string]func() (bool, bool){
					"validation": func() (bool, bool) {
						return evaluateValidationFormulaInOrg(expression, &org, target, record, nil, false)
					},
					"assignment": func() (bool, bool) {
						return engine.assignmentEntryMatches(storage.AssignmentRuleEntry{Formula: expression}, record, target)
					},
					"flow_entry": func() (bool, bool) {
						return engine.evaluateFlowRule(storage.FlowRule{Formula: expression}, record, target)
					},
					"flow_branch": func() (bool, bool) {
						return engine.evaluateFlowBranch(storage.FlowBranch{Formula: expression}, record, target, false)
					},
					"flow_decision": func() (bool, bool) { return engine.evaluateFlowDecisionFormula(expression, record, target, frame) },
					"workflow": func() (bool, bool) {
						target.WorkflowRules = []storage.WorkflowRule{{Name: "Mark", Active: true, Formula: expression, FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Mark", Field: "Marked__c", LiteralValue: "true"}}}}
						org.Objects[target.APIName] = storage.ObjectState{Definition: target, Records: map[storage.ID]storage.Record{record.ID: record.Clone()}}
						_, err := engine.applyWorkflowFieldUpdates(target.APIName, record.ID)
						value := org.Objects[target.APIName].Records[record.ID].Fields["Marked__c"]
						return value.Boolean, err == nil
					},
				}
				for name, run := range routes {
					t.Run(name, func(t *testing.T) {
						got, ok := run()
						if !ok || got != enabled {
							t.Fatalf("value=%v supported=%v want=%v", got, ok, enabled)
						}
					})
				}
			})
		}
	}
}
