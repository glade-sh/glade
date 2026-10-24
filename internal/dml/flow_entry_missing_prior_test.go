package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowEntryIsChangedRequiresCreateOrPrior(t *testing.T) {
	definition := flowEntryTestOrg().Objects["Account"].Definition
	for _, phase := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		for _, mode := range []string{"Create", "Update", "CreateAndUpdate"} {
			for _, operand := range []string{"true", "false"} {
				for _, event := range []string{"missing_prior", "create", "update_with_prior"} {
					if event == "create" && mode == "Update" || event == "update_with_prior" && operand == "false" {
						continue
					}
					t.Run(fmt.Sprintf("%s/%s/%s/%s", phase, mode, operand, event), func(t *testing.T) {
						record := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Phone": storage.StringValue("Ready")}}
						var prior *storage.Record
						if event == "create" && phase == "RecordBeforeSave" {
							record.ID = ""
						}
						if event == "update_with_prior" {
							old := record.Clone()
							old.Fields["Phone"] = storage.StringValue("Draft")
							prior = &old
						}
						item := storage.WorkflowCriteriaItem{Field: "Phone", Operation: "isChanged", Value: operand}
						rule := storage.FlowRule{TriggerType: phase, RecordTriggerType: mode}
						matches, supported := evaluateFlowCoreEntryCriteria(item, record, definition, "", prior, rule, event == "create")
						if event == "missing_prior" {
							if supported {
								t.Fatalf("missing prior acquired unchanged semantics: matches=%t, supported=%t", matches, supported)
							}
							return
						}
						want := operand == "false"
						if prior != nil {
							want = true
						}
						if !supported || matches != want {
							t.Fatalf("matches=%t supported=%t, want %t/true", matches, supported, want)
						}
					})
				}
			}
		}
	}
}

func TestFlowEntryAfterSaveCreateProvenanceConsumed(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		for _, operand := range []string{"true", "false"} {
			t.Run(fmt.Sprintf("after_save/deferred=%t/operand=%s", deferred, operand), func(t *testing.T) {
				org := flowEntryTestOrg()
				state := org.Objects["Account"]
				state.Definition.FlowRules = []storage.FlowRule{{Name: "BirchEntry", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "CreateAndUpdate", Criteria: []storage.WorkflowCriteriaItem{{Field: "Phone", Operation: "isChanged", Value: operand}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Description", LiteralValue: "created"}}}}
				org.Objects["Account"] = state
				engine := NewEngine(&org)
				engine.DeferAutomation = deferred
				result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Birch"), "Phone": storage.StringValue("Ready")}}})
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("insert: %+v", result)
				}
				id := result[0].ID
				if deferred {
					if _, err := engine.ApplyAutomation("Account", id); err != nil {
						t.Fatal(err)
					}
				}
				want := ""
				if operand == "false" {
					want = "created"
				}
				if got := org.Objects["Account"].Records[id].Fields["Description"].String; got != want {
					t.Fatalf("creation effect=%q want %q", got, want)
				}
				traces := 0
				engine.AutomationTracer = func(event string, args map[string]any) {
					if event == "apex.flow.rule" {
						traces++
						if args["modeled"] != false || args["matched"] != false {
							t.Errorf("reapplied automation lost missing-prior boundary: %+v", args)
						}
					}
				}
				if _, err := engine.ApplyAutomation("Account", id); err != nil {
					t.Fatal(err)
				}
				if traces != 1 {
					t.Fatalf("reapply traces=%d, want 1", traces)
				}
			})
		}
	}
}

func flowEntryTestOrg() storage.OrgState {
	org := testOrg()
	state := org.Objects["Account"]
	for _, name := range []string{"Phone", "Description"} {
		state.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldString}
	}
	org.Objects["Account"] = state
	return org
}
