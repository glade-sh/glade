package dml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowTargetReferencesShareFrameAndExecuteSelectedPath(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	reference := storage.FlowStep{Kind: "reference", Target: "shared"}
	rule := storage.FlowRule{
		Name: "SharedInterview",
		Steps: []storage.FlowStep{
			{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "count", Formula: "0"}},
			{Kind: "decision", Branches: []storage.FlowBranch{
				{Name: "Selected", Steps: []storage.FlowStep{reference}},
				{Name: "Default", Default: true, Steps: []storage.FlowStep{reference}},
			}},
			reference,
		},
		StepTargets: map[string][]storage.FlowStep{
			"shared": {
				{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "count", Formula: "count + 1"}},
				{Kind: "action", Action: storage.FlowAction{Name: "Observe", ActionType: "apex"}},
			},
		},
	}
	var counts []int64
	engine.FlowActionInvokerWithContext = func(_ storage.FlowAction, _ storage.Record, context FlowActionContext) error {
		counts = append(counts, context.Scalars["count"].Integer)
		return nil
	}
	output, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(counts) != "[1 2]" || output.Scalars["count"].Integer != 2 {
		t.Fatalf("shared frame counts = %v, final = %#v", counts, output.Scalars["count"])
	}
}

func TestFlowTargetReferenceErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		targets map[string][]storage.FlowStep
		want    string
	}{
		{name: "missing", want: "unknown step target"},
		{name: "cyclic", targets: map[string][]storage.FlowStep{"entry": {{Kind: "reference", Target: "entry"}}}, want: "cyclic step target"},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			_, err := engine.RunAutolaunchedFlowWithOutput(storage.FlowRule{
				Name: "InvalidInterview", Steps: []storage.FlowStep{{Kind: "reference", Target: "entry"}}, StepTargets: test.targets,
			}, FlowInterviewInput{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
		})
	}
}

func TestFlowTargetReferencesBeforeSaveAndSubflow(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	rule := storage.FlowRule{
		Name: "SharedBeforeSave", Active: true, TriggerType: "RecordBeforeSave",
		Steps:       []storage.FlowStep{{Kind: "reference", Target: "setname"}},
		StepTargets: map[string][]storage.FlowStep{"setname": {{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "Updated"}}}}},
	}
	account.Definition.FlowRules = []storage.FlowRule{rule}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Original")}}}
	if err := engine.ApplyBeforeSaveFlows(records); err != nil {
		t.Fatal(err)
	}
	if got := records[0].Fields["Name"].String; got != "Updated" {
		t.Fatalf("referenced before-save name = %q", got)
	}
	rule.StepTargets["setname"] = []storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "Unsafe", ObjectName: "Account"}}}
	account.Definition.FlowRules = []storage.FlowRule{rule}
	org.Objects["Account"] = account
	if err := engine.ApplyBeforeSaveFlows(records); err == nil || !strings.Contains(err.Error(), "unsupported side-effect steps") {
		t.Fatalf("unsafe target error = %v", err)
	}

	parent := newFlowFrame()
	child := storage.FlowRule{
		Name: "ChildInterview", Steps: []storage.FlowStep{{Kind: "reference", Target: "output"}},
		StepTargets: map[string][]storage.FlowStep{"output": {{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "childValue", LiteralValue: "returned"}}}},
	}
	err := engine.runSubflow(child, storage.ObjectDefinition{}, storage.FlowSubflow{
		Name: "CallChild", OutputAssignments: []storage.FlowAssignment{{Target: "parentValue", SourceField: "childValue"}},
	}, storage.Record{}, storage.ObjectDefinition{}, parent)
	if err != nil || parent.scalars["parentvalue"].String != "returned" {
		t.Fatalf("referenced subflow output = %#v, error = %v", parent.scalars["parentvalue"], err)
	}
}

func TestFlowTargetReferenceStaticWalk(t *testing.T) {
	const count = 80
	targets := make(map[string][]storage.FlowStep, count)
	for i := 0; i < count; i++ {
		steps := []storage.FlowStep{{Kind: "assignment"}}
		if i+1 < count {
			ref := storage.FlowStep{Kind: "reference", Target: fmt.Sprintf("step%d", i+1)}
			steps = append(steps, storage.FlowStep{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{ref}}, {Default: true, Steps: []storage.FlowStep{ref}}}})
		}
		targets[fmt.Sprintf("step%d", i)] = steps
	}
	steps := []storage.FlowStep{{Kind: "reference", Target: "step0"}}
	visited := 0
	if !flowStepsSatisfy(steps, targets, func(storage.FlowStep, bool) bool { visited++; return true }) || visited > 2*count {
		t.Fatalf("DAG walk visits = %d, want at most %d", visited, 2*count)
	}
	if !flowStepsBeforeSaveSafeWithTargets(steps, false, false, targets) {
		t.Fatal("shared assignment/decision DAG rejected")
	}
	targets["step79"] = []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Record__Prior.Id"}}}}}}
	if !flowStepsReadPriorDecision(steps, targets) {
		t.Fatal("prior reference inside shared target not found")
	}
	targets["step79"] = append(targets["step79"], storage.FlowStep{Kind: "reference", Target: "step0"})
	if flowStepsReadPriorDecision(steps, targets) || flowStepsBeforeSaveSafeWithTargets(steps, false, false, targets) {
		t.Fatal("cyclic static model accepted")
	}
	delete(targets, "step79")
	if flowStepsReadPriorDecision(steps, targets) || flowStepsBeforeSaveSafeWithTargets(steps, false, false, targets) {
		t.Fatal("missing static target accepted")
	}
}

func TestFlowTargetReferenceSafetyPreservesLegacyRestrictions(t *testing.T) {
	create := storage.FlowStep{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{ObjectName: "Account"}}
	rule := storage.FlowRule{
		TriggerType: "RecordBeforeDelete", RecordTriggerType: "Delete",
		StepTargets: map[string][]storage.FlowStep{"create": {create}},
	}
	ref := storage.FlowStep{Kind: "reference", Target: "create"}
	if !flowCoreTriggeredStepsSafe(rule, []storage.FlowStep{ref}, "before-delete") {
		t.Fatal("explicit delete root create rejected")
	}
	if flowCoreTriggeredStepsSafe(rule, []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{ref}}}}}, "before-delete") {
		t.Fatal("nested create widened legacy delete safety")
	}
	if flowStepsBeforeSaveSafe([]storage.FlowStep{{Kind: "loop"}}, false) {
		t.Fatal("loop widened legacy before-save safety")
	}
	if !hasComplexActiveAutomation(storage.ObjectDefinition{FlowRules: []storage.FlowRule{{Active: true, Steps: []storage.FlowStep{ref}, StepTargets: rule.StepTargets}}}) {
		t.Fatal("referenced side effect classified as simple automation")
	}
}
