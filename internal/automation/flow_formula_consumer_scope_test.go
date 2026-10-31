package automation

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The marker must survive the modeled graph and cache representation; the
// captured assignment and create fixtures each need it on their own inputs.
func TestFlowFormulaSelectedConsumersCarryRuntimeMarker(t *testing.T) {
	for _, name := range []string{"FFFrameAssign", "FFFrameCreate"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "apextest", "testdata", "conformance", "flow_formula_frame", "project", "force-app", "main", "default", "flows", name+".flow-meta.xml")
			flow, diagnostics, err := loadFlow(path)
			if err != nil || len(flow.Rules) != 1 {
				t.Fatalf("load: %v, diagnostics=%v", err, diagnostics)
			}
			encoded, err := json.Marshal(flow.Rules[0])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), `"flowFrameFormula":true`) {
				t.Fatalf("selected formula marker missing: %s", encoded)
			}
		})
	}
}
