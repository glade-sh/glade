package automation

import "testing"

// Flow constants cannot change, so formulas and Custom Error text carry their
// literal value (native rows: apextest flow_gaps P01-P08).
func TestFlowFormulaMapInlinesConstants(t *testing.T) {
	formulas := []flowFormulaXML{{Name: "within", Expression: "TODAY() + {!lim} >= {!$Record.CloseDate}"}, {Name: "label", Expression: "{!tag} & {!flag}"}}
	constants := []flowConstantXML{
		{Name: "lim", Value: flowValueXML{NumberValue: "365.0"}},
		{Name: "tag", Value: flowValueXML{StringValue: "x"}},
		{Name: "flag", Value: flowValueXML{BooleanValue: "true"}},
		{Name: "skipped", Value: flowValueXML{DateValue: "2031-03-09"}},
	}
	got := flowFormulaMapWithConstants(formulas, constants)
	if want := "TODAY() + (365) >= CloseDate"; got["within"] != want {
		t.Fatalf("within = %q, want %q", got["within"], want)
	}
	if got["label"] != `("x") & (TRUE)` {
		t.Fatalf("label = %q", got["label"])
	}
	if plain := flowFormulaMap(formulas); plain["within"] == got["within"] {
		t.Fatalf("constants must only inline when declared: %q", plain["within"])
	}
}

func TestModeledFlowCustomErrorInlinesCapturedConstants(t *testing.T) {
	constants := []flowConstantXML{
		{Name: "lim", Value: flowValueXML{NumberValue: "365.0"}},
		{Name: "big", Value: flowValueXML{NumberValue: "123456"}},
		{Name: "tag", Value: flowValueXML{StringValue: "x"}},
	}
	custom := flowCustomErrorXML{Name: "E", Messages: []flowCustomErrorMessageXML{{ErrorMessage: "n={!lim} t={!tag} big={!big} d={!$Record.CloseDate} u={!unknown}"}}}
	got := modeledFlowCustomError(custom, constants).Messages[0].Message
	if want := "n=365 t=x big={!big} d={!$Record.CloseDate} u={!unknown}"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
