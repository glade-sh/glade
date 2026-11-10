package automation

import "testing"

func TestFlowAfterSaveErrorFormulaScope(t *testing.T) {
	direct := flowCustomErrorXML{Name: "Direct", Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!Value}"}}}
	other := flowCustomErrorXML{Name: "Other", Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!Value}"}}}
	raw := flowXML{
		Start:        flowStartXML{Object: "Account", TriggerType: "RecordAfterSave", RecordTriggerType: "Create", Connector: flowConnectorXML{TargetReference: "Direct"}},
		Variables:    []flowVariableXML{{Name: "Name", DataType: "String"}},
		Formulas:     []flowFormulaXML{{Name: "Value", Expression: `{!Name} & "|" & {!$Record.Name}`}},
		CustomErrors: []flowCustomErrorXML{direct, other},
	}
	raw.frameFormulas = flowFrameFormulaMap(raw, flowVariableMap(raw.Variables))
	if len(flowCustomErrorElementFrameFormulas(raw, direct)) == 0 {
		t.Fatal("direct captured formula missing")
	}
	if got := flowCustomErrorElementFrameFormulas(raw, other); len(got) != 0 {
		t.Fatalf("unobserved error branch gained bindings: %v", got)
	}
}
