package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native rows capture a system LastModifiedDate only as a parent
// assignment value. Subflow input binding and Get Records filters have no
// native observation, so they keep resolving a source record that carries no
// explicit LastModifiedDate value as null, as before the audit fallback.
const flowAuditUncapturedStamp = "2026-10-19T01:02:03.000Z"

func flowAuditUncapturedFixture(api string, invocable bool, fields map[string]storage.Value) (*Engine, storage.Record, storage.ObjectDefinition, *flowFrame) {
	org := testOrg()
	org.APIVersion = api
	engine := NewEngine(&org)
	engine.InvocableFlow = invocable
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime}}}
	source := storage.Record{Object: "Contact", Fields: fields, System: storage.SystemFields{LastModifiedDate: flowAuditUncapturedStamp}}
	frame := newFlowFrame()
	frame.saveDecisions = !invocable
	frame.autolaunched = invocable
	seedFlowFrame(&engine, &source, frame)
	return &engine, source, definition, frame
}

func TestFlowAuditSystemTimestampSubflowInputRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous_interview", "istest_trigger"} {
			for _, reference := range []string{"$Record.LastModifiedDate", "LastModifiedDate"} {
				for _, control := range []struct {
					name   string
					fields map[string]storage.Value
					want   storage.Value
				}{
					{name: "system_only", want: storage.NullValue()},
					{name: "explicit_value", fields: map[string]storage.Value{"LastModifiedDate": storage.DateTimeValue("2026-10-18T00:00:00.000Z")}, want: storage.DateTimeValue("2026-10-18T00:00:00.000Z")},
				} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", api, route, reference, control.name), func(t *testing.T) {
						engine, source, definition, frame := flowAuditUncapturedFixture(api, route == "anonymous_interview", control.fields)
						child := storage.FlowRule{Name: "AuditLeaf", Active: true, Variables: []storage.FlowVariable{{Name: "stamp", DataType: "DateTime", IsInput: true, IsOutput: true}}}
						err := engine.runSubflow(child, definition, storage.FlowSubflow{
							Name: "CallChild", FlowName: child.Name,
							InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "stamp", SourceField: reference}},
							OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "stamp"}},
						}, source, definition, frame)
						if err != nil {
							t.Fatal(err)
						}
						got, exists := frame.scalars["returned"]
						if !exists || !storage.ValuesEqual(got, control.want) {
							t.Fatalf("subflow input=%#v present=%t; want %#v", got, exists, control.want)
						}
					})
				}
			}
		}
	}
}

func TestFlowAuditSystemTimestampLookupCriteriaRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous_interview", "istest_trigger"} {
			for _, reference := range []string{"$Record.LastModifiedDate", "LastModifiedDate"} {
				for _, control := range []struct {
					name   string
					fields map[string]storage.Value
					equals bool
				}{
					{name: "system_only", equals: false},
					{name: "explicit_value", fields: map[string]storage.Value{"LastModifiedDate": storage.DateTimeValue(flowAuditUncapturedStamp)}, equals: true},
				} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", api, route, reference, control.name), func(t *testing.T) {
						engine, source, sourceDefinition, frame := flowAuditUncapturedFixture(api, route == "anonymous_interview", control.fields)
						targetDefinition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Observed__c": {APIName: "Observed__c", Type: storage.FieldDateTime}}}
						target := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Observed__c": storage.DateTimeValue(flowAuditUncapturedStamp)}}
						for _, operation := range []string{"equals", "notequal"} {
							want := control.equals
							if operation == "notequal" {
								want = !want
							}
							matches, ok := engine.evaluateFlowLookupCriteria(storage.WorkflowCriteriaItem{Field: "Observed__c", Operation: operation, SourceField: reference}, target, targetDefinition, source, sourceDefinition, frame)
							if !ok || matches != want {
								t.Errorf("%s: matches=%t supported=%t; want %t", operation, matches, ok, want)
							}
						}
					})
				}
			}
		}
	}
}
