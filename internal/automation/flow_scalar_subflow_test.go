package automation

import (
	"path/filepath"
	"testing"
)

func TestFlowLegacyScalarAssignmentEntryDirectAndSubflow(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, name := range []string{"Bool", "Fraction", "Zero"} {
			t.Run("api"+api+"/"+name+"/direct_interview_and_trigger_subflow", func(t *testing.T) {
				path := filepath.Join("../apextest/testdata/conformance/flow_scalar_subflow", "api"+api, "force-app/main/default/flows", "CedarScalar"+name+".flow-meta.xml")
				flow, diagnostics, err := loadFlow(path)
				if err != nil || len(diagnostics) != 0 {
					t.Fatalf("load: %v %v", err, diagnostics)
				}
				if len(flow.Rules) != 1 || len(flow.Rules[0].Steps) != 1 || flow.Rules[0].Steps[0].Assignment.Target != "Result" {
					t.Fatalf("native scalar assignment must retain its executable entry: rules=%+v", flow.Rules)
				}
			})
		}
		t.Run("api"+api+"/unknown_target_rejection", func(t *testing.T) {
			path := filepath.Join("../apextest/testdata/conformance/flow_scalar_subflow", "reject"+api, "force-app/main/default/flows/CedarScalarUnknown.flow-meta.xml")
			flow, _, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range flow.Rules {
				if len(rule.Steps) != 0 {
					t.Fatal("native-rejected undeclared assignment must not gain an executable entry")
				}
			}
		})
	}
}
