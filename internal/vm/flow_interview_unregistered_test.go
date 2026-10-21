package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// automation.json API 62/67 R116/R120 capture createInterview/start using
// deployed flow metadata. A platform class needs no synthetic user declaration.
func TestFlowInterviewCreateWithoutSyntheticClass(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Flow.Interview interview=Flow.Interview.createInterview('SproutScalar',new Map<String,Object>{'seed'=>'leaf'});
interview.start();
System.assertEquals('leaf',interview.getVariableValue('result'));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Metadata.Flows = []storage.FlowRule{scalarInterviewRule("SproutScalar")}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFlowInterviewGeneratedMethodsUseRuntime(t *testing.T) {
	runGeneratedFlowInterview(t, false)
}

func TestFlowInterviewGeneratedRootUsesRuntime(t *testing.T) {
	runGeneratedFlowInterview(t, true)
}

func runGeneratedFlowInterview(t *testing.T, includeRoot bool) {
	t.Helper()
	program, err := CompileAnonymous(`
Flow.Interview interview=Flow.Interview.createInterview('SproutScalar',new Map<String,Object>{'seed'=>'leaf'});
interview.start();
System.assertEquals('leaf',interview.getVariableValue('result'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.Flows = []storage.FlowRule{scalarInterviewRule("SproutScalar")}
	machine := New(nil)
	machine.SetOrg(&org)
	if includeRoot {
		if err = machine.RegisterClass(Class{Name: "Flow", PlatformType: true, Access: "global"}); err != nil {
			t.Fatal(err)
		}
	}
	if err = machine.RegisterClass(Class{Name: "Flow.Interview", PlatformType: true, Methods: map[string]Method{
		"createInterview":  {Name: "Flow.Interview.createInterview", ClassName: "Flow.Interview", IsStatic: true, Access: "global", ReturnType: "Flow.Interview", Modifiers: []string{"passive-generated"}, Params: []Param{{Name: "flowName", Type: "String"}, {Name: "initialValues", Type: "Map<String,Object>"}}},
		"start":            {Name: "Flow.Interview.start", ClassName: "Flow.Interview", Access: "global", ReturnType: "void", Modifiers: []string{"passive-generated"}},
		"getVariableValue": {Name: "Flow.Interview.getVariableValue", ClassName: "Flow.Interview", Access: "global", ReturnType: "Object", Modifiers: []string{"passive-generated"}, Params: []Param{{Name: "variableName", Type: "String"}}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
