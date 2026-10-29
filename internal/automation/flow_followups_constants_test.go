package automation

import "testing"

func TestFlowFollowupsZeroConstantErrorMerge(t *testing.T) {
	// flow_followups/org.tsv CE rows capture zero and negative-zero Number
	// constants alongside the record merge, at both API 62.0 and 67.0.
	for _, number := range []string{"0.00", "-0.00", "-0"} {
		t.Run(number, func(t *testing.T) {
			constants := []flowConstantXML{{Name: "value", Value: flowValueXML{NumberValue: number}}}
			if got := inlineFlowConstantMergeFields("value={!value}", constants); got != "value=0" {
				t.Fatalf("numeric constant %q: got %q; native value=0", number, got)
			}
		})
	}
	// Authored string constants must not be treated as numbers.
	constants := []flowConstantXML{{Name: "value", Value: flowValueXML{StringValue: "-0.00"}}}
	if got := inlineFlowConstantMergeFields("value={!value}", constants); got != "value=-0.00" {
		t.Fatalf("string constant changed: %q", got)
	}
}
