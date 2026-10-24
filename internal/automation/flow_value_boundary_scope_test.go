package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowValueBoundaryUncapturedLegacyBooleanSourceRemainsUnsupported(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run("api"+api+"/direct_interview_and_trigger_subflow_entry", func(t *testing.T) {
			path := filepath.Join("../apextest/testdata/conformance/flow_scalar_subflow", "api"+api, "force-app/main/default/flows/CedarScalarBool.flow-meta.xml")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body = []byte(strings.ReplaceAll(string(body), "<booleanValue>true</booleanValue>", "<stringValue>text</stringValue>"))
			path = filepath.Join(t.TempDir(), "ScalarBoolean.flow-meta.xml")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			flow, _, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range flow.Rules {
				if len(rule.Steps) != 0 {
					t.Fatal("uncaptured string source must remain outside the legacy Boolean entry boundary")
				}
			}
		})
	}
}
