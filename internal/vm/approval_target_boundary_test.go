package vm

import (
	"strings"
	"testing"
)

func TestApprovalNamedTargetObjectBoundary(t *testing.T) {
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	machine := New(nil)
	machine.SetOrg(&org)
	request := Object("Approval.ProcessSubmitRequest")
	request.Fields["ProcessDefinitionNameOrId"] = String("Account_Local_Approval")
	_, _, err := machine.approvalProcessDefinitionForRequest(request, "Contact")
	if err == nil || !strings.Contains(err.Error(), "hosted approval engine routing") {
		t.Fatalf("foreign-object process error = %v", err)
	}
}
