package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowFollowupsDecisionFormulaBindings(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_followups/project/force-app/main/default/flows/FF_Shadow.flow-meta.xml"
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, replace, quoted string
		scalarOnly            bool
	}{
		{name: "native"},
		{name: "quoted_references", replace: `"FLOW_NAME"`, quoted: `"$Record.Name {!Name} Record.Name"`},
		{name: "scalar_only", scalarOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := string(body)
			if tc.replace != "" {
				source = strings.ReplaceAll(source, tc.replace, tc.quoted)
			}
			if tc.scalarOnly {
				source = strings.ReplaceAll(source, `AND({!Name} = "FLOW_NAME", {!$Record.Name} = "FF_SHADOW_PASS")`, `{!Name} = "FLOW_NAME"`)
				source = strings.ReplaceAll(source, `AND({!NumberOfEmployees} = 7, {!$Record.NumberOfEmployees} = 19)`, `{!NumberOfEmployees} = 7`)
			}
			path := filepath.Join(t.TempDir(), "FF_Shadow.flow-meta.xml")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			flow, diagnostics, err := loadFlow(path)
			if err != nil || len(flow.Rules) != 1 {
				t.Fatalf("load native flow: %v; rules=%d; diagnostics=%v", err, len(flow.Rules), diagnostics)
			}
			if len(flow.Rules[0].Steps) == 0 || len(flow.Rules[0].Branches) != 0 {
				t.Errorf("scalar decision must use a framed step graph: steps=%d, legacy branches=%d", len(flow.Rules[0].Steps), len(flow.Rules[0].Branches))
			}
			for _, name := range []string{"Name", "NumberOfEmployees"} {
				found := false
				for _, variable := range flow.Rules[0].Variables {
					found = found || variable.Name == name
				}
				if !found {
					t.Errorf("scalar decision lost interview variable %s", name)
				}
			}
			var expressions []string
			var visit func([]storage.FlowStep)
			visit = func(steps []storage.FlowStep) {
				for _, step := range steps {
					for _, branch := range step.Branches {
						expressions = append(expressions, branch.Formula)
						visit(branch.Steps)
					}
					visit(step.FaultBranch)
				}
			}
			visit(flow.Rules[0].Steps)
			for _, steps := range flow.Rules[0].StepTargets {
				visit(steps)
			}
			joined := strings.Join(expressions, "\n")
			// Native FV rows distinguish each scalar from its explicit record twin.
			operands := []string{`NumberOfEmployees = 7`}
			if !tc.scalarOnly {
				operands = append(operands, `$Record.Name = "FF_SHADOW_PASS"`, `$Record.NumberOfEmployees = 19`)
			}
			for _, operand := range operands {
				if !strings.Contains(joined, operand) {
					t.Errorf("lost decision operand %s in %s", operand, joined)
				}
			}
			quoted := `"FLOW_NAME"`
			if tc.quoted != "" {
				quoted = tc.quoted
			}
			if !strings.Contains(joined, "Name = "+quoted) {
				t.Errorf("scalar or quoted text changed in %s", joined)
			}
		})
	}
	// Record prefix lowering outside the decision frame keeps its existing
	// representation; changing decision bindings must not change assignments.
	ordinary := flowFormulaMap([]flowFormulaXML{{Name: "ordinary", Expression: `{!$Record.Name} & {!Name}`}})
	if ordinary["ordinary"] != `Name & Name` {
		t.Fatalf("ordinary formula lowering changed: %s", ordinary["ordinary"])
	}
}
