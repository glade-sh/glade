package automation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestFlowValueBoundaryCustomParentUpdateLowering(t *testing.T) {
	root := filepath.Join("../dml/testdata/conformance/flow_value_boundaries", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, flow := range idx.Flows {
		if strings.TrimSuffix(filepath.Base(flow.File), ".flow-meta.xml") != "ValueParentRecordUpdateTrigger" {
			continue
		}
		if len(flow.Rules) != 1 || len(flow.Rules[0].Steps) != 1 {
			t.Fatalf("native parent update must lower: rules=%+v diagnostics=%v", flow.Rules, idx.Diagnostics)
		}
		step := flow.Rules[0].Steps[0]
		if step.Kind != "recordUpdate" || step.RecordUpdate.ObjectName != "Account" || step.RecordUpdate.InputReference != "$Record.Parent__r" {
			t.Fatalf("native custom parent target=Account: %+v", step)
		}
		return
	}
	t.Fatal("missing synthetic parent update")
}

func TestFlowValueBoundaryFormulaRecordResourceRoutes(t *testing.T) {
	for _, route := range []string{"interview", "action", "trigger", "subflow"} {
		t.Run(route, func(t *testing.T) {
			reference := "InputRecord"
			want := `InputRecord.Search__c & ":" & TEXT(InputRecord.When__c)`
			if route == "trigger" {
				reference = "$Record"
				want = `Search__c & ":" & TEXT(When__c)`
			}
			expression := `{!` + reference + `.Search__c} & ":" & TEXT({!` + reference + `.When__c})`
			if got := normalizeFlowFormula(expression); got != want {
				t.Fatalf("native formula retains its record resource: got %q, want %q", got, want)
			}
		})
	}
}

func TestFlowValueBoundaryBooleanAssignmentEntryValidation(t *testing.T) {
	p, err := project.Load(filepath.Join("../dml/testdata/conformance/flow_value_boundaries", "project"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, flow := range index.Flows {
		if filepath.Base(flow.File) == "ValueConstantsAssignTrigger.flow-meta.xml" {
			if len(flow.Rules) != 1 {
				t.Fatalf("native accepts Boolean resource assignments in a save Flow: %v", index.Diagnostics)
			}
			return
		}
	}
	t.Fatal("missing synthetic Boolean assignment save Flow")
}
