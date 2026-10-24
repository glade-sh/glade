package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowIntegerInputBoundaryNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join("../dml/testdata/conformance/flow_value_boundaries", "api"+strings.TrimSuffix(api, ".0"), "org.tsv")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 3)
				if len(cells) != 3 {
					t.Fatal("invalid native row")
				}
				if cells[0] != "anonymous" || !strings.HasPrefix(cells[1], "Integer") {
					continue
				}
				t.Run(cells[1], func(t *testing.T) {
					var want struct {
						Error   bool   `json:"error"`
						Value   *int64 `json:"value"`
						Integer bool   `json:"integer"`
					}
					if err := json.Unmarshal([]byte(cells[2]), &want); err != nil {
						t.Fatal(err)
					}
					fraction := strings.HasPrefix(cells[1], "IntegerFraction")
					number := "7"
					if fraction {
						number = "7.75"
					}
					for suffix, text := range map[string]string{"Maximum": "2147483647", "Overflow": "2147483648", "Negative": "-7", "Integral": "7.00"} {
						if strings.HasSuffix(cells[1], suffix) {
							number = text
							if fraction && suffix != "Integral" {
								number += ".75"
							}
							if fraction && suffix == "Maximum" {
								number = "2147483647.49"
							}
						}
					}
					value := storage.DecimalValue(number)
					if strings.HasSuffix(cells[1], "Null") {
						value = storage.NullValue()
					}
					machine := New(nil)
					if err := machine.RegisterClass(Class{Name: "BoundaryInput", Fields: map[string]Field{"amount": {Name: "amount", Type: "Integer"}}}); err != nil {
						t.Fatal(err)
					}
					record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Amount__c": value}}
					root := "InputRecord"
					if strings.Contains(cells[1], "Trigger") {
						root = "$Record"
					}
					context := dml.FlowActionContext{Records: map[string]storage.Record{strings.ToLower(root): record}}
					action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Name: "amount", SourceField: root + ".Amount__c"}}}
					item, err := machine.flowActionInputObject(action, record, context, "BoundaryInput", "inputs", &Result{})
					if want.Error {
						if err == nil {
							t.Fatalf("native rejects numeric input; Glade accepted kind=%s text=%s", item.Fields["amount"].Kind, item.Fields["amount"].Text)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					got := item.Fields["amount"]
					if want.Value == nil {
						if got.Kind != ValueNull {
							t.Fatalf("native=null; Glade=%#v", got)
						}
						return
					}
					if got.Kind != ValueInt || got.Int != *want.Value {
						t.Fatalf("native Integer=%d; Glade kind=%s text=%s", *want.Value, got.Kind, got.Text)
					}
				})
			}
		})
	}
}

func TestFlowValueBoundaryInvocableApexStepRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, compact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/targets=%v", api, compact), func(t *testing.T) {
				rule := storage.FlowRule{APIVersion: api, ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "InputRecord", DataType: "SObject", ObjectType: "Account", IsInput: true}}, Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "apex", ClassName: "BoundaryIntegerAction"}}}}
				if compact {
					rule.StepTargets = map[string][]storage.FlowStep{"entry": rule.Steps}
					rule.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
				}
				if !invocableFlowActionSupported(rule) {
					t.Fatal("native Invocable.Action executes a Flow Apex action")
				}
			})
		}
	}
}

func TestFlowValueBoundaryInvocableAssignedBooleanRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, compact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/targets=%v", api, compact), func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				rule := storage.FlowRule{Name: "BoundaryAssignedFlag", Active: true, APIVersion: api, ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "Ready", DataType: "Boolean", IsInput: true, IsOutput: true}}, Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Ready", Operator: "Assign", Formula: "TRUE"}}}}
				if compact {
					rule.StepTargets = map[string][]storage.FlowStep{"entry": rule.Steps}
					rule.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
				}
				org.Metadata.Flows = []storage.FlowRule{rule}
				machine := New(nil)
				machine.SetOrg(&org)
				action := Object("Invocable.Action")
				action.Fields["name"] = String(rule.Name)
				output, success, err := machine.invokeInvocableFlowAction(action, typedMap("Map<String,Object>"), &Result{})
				if err != nil || !success {
					t.Fatalf("native permits an omitted Boolean assigned before use: success=%v err=%v", success, err)
				}
				if flag := output.Map[mapKey(String("Ready"))]; flag.Kind != ValueBool || !flag.Bool {
					t.Fatalf("native assigned flag=true; Glade=%v", flag)
				}
			})
		}
	}
}
