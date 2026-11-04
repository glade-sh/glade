package automation

import "testing"

// Uncaptured numeric spellings stay unresolved; see flow_merge_format/org.tsv.
func TestFlowMergeUncapturedNumericSpellings(t *testing.T) {
	for _, value := range []string{"001000.00", "+1000", "1e3"} {
		t.Run(value, func(t *testing.T) {
			constant := flowConstantXML{Name: "n", Value: flowValueXML{NumberValue: value}}
			node := flowCustomErrorXML{Messages: []flowCustomErrorMessageXML{{ErrorMessage: "v={!n}"}}}
			message := modeledFlowCustomError(node, []flowConstantXML{constant}).Messages[0]
			if message.Message != "v={!n}" || len(message.NumberConstants) != 0 {
				t.Fatalf("uncaptured value %q was modeled: %+v", value, message)
			}
		})
	}
}
