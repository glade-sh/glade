package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoctorOrgSetupRenderingRoutes(t *testing.T) {
	for _, route := range []string{"noEvidence", "fieldEvidence"} {
		t.Run(route, func(t *testing.T) {
			info := DoctorInfo{ParserOK: true, ToolchainOK: true}
			if route == "fieldEvidence" {
				info.OrgSetup = []DoctorOrgSetup{{Kind: "feature", Feature: "PersonAccounts", Reason: "source uses a platform field", Files: []string{"force-app/SetupUse.cls"}, YAML: "org:\n  features: [PersonAccounts]"}}
			}
			var output bytes.Buffer
			if err := WriteDoctor(&output, info); err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"Org setup the project uses", "features: [PersonAccounts]", "force-app/SetupUse.cls"} {
				if strings.Contains(output.String(), text) != (route == "fieldEvidence") {
					t.Fatalf("%s rendering mismatch for %q:\n%s", route, text, &output)
				}
			}
		})
	}
}
