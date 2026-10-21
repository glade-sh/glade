package dml

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowPathTrackedReferenceValidation(t *testing.T) {
	tracked := storage.FlowStep{Kind: "reference", Target: "entry", VisitKey: "entry"}
	untracked := storage.FlowStep{Kind: "reference", Target: "entry"}
	for _, test := range []struct {
		name      string
		entry     storage.FlowStep
		body      []storage.FlowStep
		wantSafe  bool
		wantPrior bool
	}{
		{
			name: "tracked cycle", entry: tracked, wantSafe: true, wantPrior: true,
			body: []storage.FlowStep{
				{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "count", Operator: "Assign", Formula: "1"}},
				{Kind: "decision", Branches: []storage.FlowBranch{{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Record__Prior.Id"}}, Steps: []storage.FlowStep{tracked}}}},
			},
		},
		{name: "untracked cycle", entry: untracked, body: []storage.FlowStep{untracked}},
		{name: "untracked closing edge", entry: tracked, body: []storage.FlowStep{untracked}},
		{name: "untracked entry with tracked closing edge", entry: untracked, body: []storage.FlowStep{tracked}},
		{name: "mismatched tracked closing edge", entry: tracked, body: []storage.FlowStep{{Kind: "reference", Target: "entry", VisitKey: "different"}}},
		{name: "untracked entry with nested tracked closing edge", entry: untracked, body: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{tracked}}}}}},
		{name: "missing tracked target", entry: storage.FlowStep{Kind: "reference", Target: "missing", VisitKey: "missing"}},
		{
			name: "unsupported after tracked backedge", entry: tracked,
			body: []storage.FlowStep{tracked, {Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{ObjectName: "Account"}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			targets := map[string][]storage.FlowStep{"entry": test.body}
			steps := []storage.FlowStep{test.entry}
			if got := flowStepsBeforeSaveSafeWithTargets(steps, false, false, targets); got != test.wantSafe {
				t.Fatalf("before-save validation = %t, want %t", got, test.wantSafe)
			}
			if got := flowStepsReadPriorDecision(steps, targets); got != test.wantPrior {
				t.Fatalf("prior-decision read = %t, want %t", got, test.wantPrior)
			}
		})
	}
}

func TestFlowPathVisitedKeyDoesNotHideMissingTarget(t *testing.T) {
	for _, target := range []string{"entry", "missing"} {
		t.Run(target, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			_, err := engine.RunAutolaunchedFlowWithOutput(storage.FlowRule{
				Name:        "VisitedTarget",
				Steps:       []storage.FlowStep{{Kind: "reference", Target: "entry", VisitKey: "entry"}},
				StepTargets: map[string][]storage.FlowStep{"entry": {{Kind: "reference", Target: target, VisitKey: "entry"}}},
			}, FlowInterviewInput{})
			if target == "entry" {
				if err != nil {
					t.Fatalf("tracked repeat = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "unknown step target") {
				t.Fatalf("missing target hidden by visited key: %v", err)
			}
		})
	}
}

func TestFlowPathPrimitiveLoopRestoresVisitedScope(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "complete"
		if fail {
			name = "action error"
		}
		t.Run(name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			frame := newFlowFrame()
			frame.variables["items"] = storage.FlowVariable{Name: "Items", DataType: "String", IsCollection: true}
			frame.scalars["items"] = storage.ListValue(storage.StringValue("First"), storage.StringValue("Second"))
			frame.scalars["item"] = storage.StringValue("Before")
			parent := map[string]bool{"parent": true}
			frame.pathVisited = parent
			ref := storage.FlowStep{Kind: "reference", Target: "body", VisitKey: "body"}
			frame.stepTargets = map[string][]storage.FlowStep{"body": {
				{Kind: "action", Action: storage.FlowAction{Name: "Notify", ActionType: "apex"}},
				ref,
			}}
			var calls []string
			engine.FlowActionInvokerWithContext = func(_ storage.FlowAction, _ storage.Record, context FlowActionContext) error {
				item := context.Scalars["item"].String
				calls = append(calls, item)
				if fail && item == "Second" {
					return errors.New("owned primitive failure")
				}
				return nil
			}
			record := storage.Record{}
			handled, _, err := engine.flowCorePrimitiveLoop("PrimitiveInterview", "", &record, storage.ObjectDefinition{}, storage.FlowLoop{
				Name: "Repeat", CollectionReference: "Items", CurrentItemReference: "Item", IterationOrder: "Asc", Steps: []storage.FlowStep{ref},
			}, frame)
			if !handled {
				t.Fatal("declared primitive loop was not handled")
			}
			if fail {
				if err == nil || !strings.Contains(err.Error(), "owned primitive failure") {
					t.Fatalf("action error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(calls, ","); got != "First,Second" {
				t.Fatalf("primitive loop item contexts = %q", got)
			}
			if len(frame.pathVisited) != 1 || !frame.pathVisited["parent"] || frame.pathVisited["body"] || len(parent) != 1 {
				t.Fatalf("body visits leaked: frame=%v parent=%v", frame.pathVisited, parent)
			}
			if got := frame.scalars["item"].String; got != "Before" {
				t.Fatalf("prior primitive item = %q", got)
			}
			if len(frame.activeTargets) != 0 {
				t.Fatalf("active body target leaked: %v", frame.activeTargets)
			}
		})
	}
}
