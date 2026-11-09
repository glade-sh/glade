package apextest

import "testing"

func TestApprovalProjectOwnerRoutingRecognized(t *testing.T) {
	org := OrgFromIndex(loadTestIndex(t, "testdata/conformance/approval_project_capture/project"))
	found := false
	for _, r := range org.Objects["ProcessDefinition"].Records {
		if r.Fields["DeveloperName"].String == "Owned_Active" {
			found = true
			if got := r.Fields["GladeApprovalRouting"].String; got != "owner" {
				t.Fatalf("captured owner routing = %q", got)
			}
		}
	}
	if !found {
		t.Fatal("definition absent")
	}
}
