package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func scalarInterviewRule(name string) storage.FlowRule {
	return storage.FlowRule{
		Name: name, Active: true, ProcessType: "AutoLaunchedFlow",
		Variables: []storage.FlowVariable{
			{Name: "seed", DataType: "String", IsInput: true},
			{Name: "result", DataType: "String", IsOutput: true},
		},
		Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{
			Name: "Echo", Target: "result", Operator: "Assign", SourceField: "seed",
		}}},
	}
}

// The Interview reference requires start() to run the named Flow and expose
// its resulting variables through getVariableValue. automation.json R126/R128
// supplies the owned native String-input conversion controls at API 62/67/68.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_reference_guide.pdf
func TestFlowInterviewStartsRequestedFlow(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"dynamic", `Flow.Interview i = Flow.Interview.createInterview('SproutScalar', new Map<String,Object>{'seed'=>'leaf'}); i.start(); System.assertEquals('leaf', i.getVariableValue('result')); System.assertEquals(null, i.getVariableValue('absent'));`},
		{"namespaced_dynamic", `Flow.Interview i = Flow.Interview.createInterview('sprig', 'SproutScalar', new Map<String,Object>{'seed'=>'leaf'}); i.start(); System.assertEquals('namespace-leaf', i.getVariableValue('result'));`},
		{"subclass", `Flow.Interview i = new Flow.Interview.SproutScalar(new Map<String,Object>{'seed'=>42}); i.start(); System.assertEquals('42', i.getVariableValue('result'));`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := storage.NewOrgState()
			namespaced := scalarInterviewRule("sprig.SproutScalar")
			namespaced.Steps[0].Assignment.SourceField = ""
			namespaced.Steps[0].Assignment.LiteralValue = "namespace-leaf"
			org.Metadata.Flows = []storage.FlowRule{scalarInterviewRule("SproutScalar"), namespaced}
			machine.SetOrg(&org)
			for _, class := range []Class{{Name: "Flow.Interview"}, {Name: "Flow.Interview.SproutScalar", SuperClass: "Flow.Interview"}} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFlowInterviewCannotExposeUndeclaredVariable(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Metadata.Flows = []storage.FlowRule{scalarInterviewRule("SproutScalar")}
	machine.SetOrg(&org)
	interview := Object("Flow.Interview")
	interview.Fields["flowName"] = String("SproutScalar")
	interview.Fields["started"] = Bool(true)
	// An extra frame/input value is not a variable in the executed Flow. The
	// documented absent-variable result is null, independent of stored extras.
	extras := Map()
	extras.Map[mapKey(String("absent"))] = Int(42)
	extras.MapKeys[mapKey(String("absent"))] = String("absent")
	for _, field := range []string{"variables", "outputs"} {
		interview.Fields[field] = extras
	}
	value, _, _, handled, err := machine.callPlatformObjectMember(interview, "getVariableValue", []Value{String("absent")}, &Result{})
	if err != nil || !handled || value.Kind != ValueNull {
		t.Fatalf("absent variable = %#v, handled=%v, err=%v; want null", value, handled, err)
	}
}
