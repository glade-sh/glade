package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowValueBoundaryLookupRecordNullRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"interview", "action", "trigger", "subflow"} {
			for _, found := range []bool{false, true} {
				for _, null := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/found=%v/isnull=%v", api, route, found, null), func(t *testing.T) {
						org := storage.NewOrgState()
						org.APIVersion = api
						storage.EnsureStandardObject(&org, "Account")
						engine := NewEngine(&org)
						engine.InvocableFlow = route != "trigger"
						frame := newFlowFrame()
						frame.saveDecisions = route == "trigger" || route == "subflow"
						output := flowLookupOutput{definition: org.Objects["Account"].Definition, record: storage.Record{Object: "Account"}}
						if found {
							output.record.ID = "001000000000001AAA"
						}
						frame.lookupOutputs["rec"] = output
						branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "rec", Operation: "isnull", Value: fmt.Sprint(null)}}}
						match, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
						if !supported || match != (null != found) {
							t.Fatalf("match=%v supported=%v, native=%v", match, supported, null != found)
						}
					})
				}
			}
		}
	}
}

func TestFlowValueBoundaryParentSourceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"interview", "action", "trigger", "subflow"} {
			for _, found := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/parent=%v", api, route, found), func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Account")
					storage.EnsureStandardObject(&org, "Contact")
					engine := NewEngine(&org)
					engine.InvocableFlow = route != "trigger"
					parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("parent text")}}})[0]
					if !parent.Success {
						t.Fatal(parent)
					}
					source := storage.Record{Object: "Contact", Fields: map[string]storage.Value{}}
					if found {
						source.Fields["AccountId"] = storage.IDValue(parent.ID)
					}
					frame := newFlowFrame()
					frame.saveDecisions = route == "trigger" || route == "subflow"
					root := "InputRecord"
					if route == "trigger" {
						root = "$Record"
					}
					frame.records[flowFrameKey(root)] = source
					reference := root + ".Account.Name"
					if route == "trigger" {
						reference = "Account.Name"
					}
					got, ok := engine.flowRecordUpdateSourceValue(reference, source, org.Objects["Contact"].Definition, frame)
					if !ok || found && got.String != "parent text" || !found && got.Kind != storage.ValueNull {
						t.Fatalf("native parent=%v; Glade=%#v resolved=%v", found, got, ok)
					}
				})
			}
		}
	}
}
