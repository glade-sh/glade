package apextest

import (
	"strings"
	"testing"
)

// The fixture was deployed in the approval submission native capture. Salesforce
// exposes both active and inactive definitions by developer name.
func TestApprovalProjectMetadataDefinitions(t *testing.T) {
	index := loadTestIndex(t, "testdata/conformance/approval_submission/project")
	org := OrgFromIndex(index)
	definitions := org.Objects["ProcessDefinition"]
	got := map[string]string{}
	for _, record := range definitions.Records {
		name := record.Fields["DeveloperName"].String
		state := record.Fields["State"].String
		if strings.HasPrefix(name, "Owned_") {
			got[name] = state
		}
	}
	if got["Owned_Active"] != "Active" || got["Owned_Inactive"] != "Inactive" {
		t.Fatalf("project approval definitions = %v, want active and inactive definitions", got)
	}
}
