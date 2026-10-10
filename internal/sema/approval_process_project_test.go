package sema

import "testing"

func TestProjectBodyResolvesApprovalProcessDespiteApprovalSObjectName(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ApprovalProcessCaller.cls": `
public class ApprovalProcessCaller {
    public static Approval.ProcessResult run() {
        Approval.ProcessWorkitemRequest request = new Approval.ProcessWorkitemRequest();
        return Approval.process(request);
    }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("project-body Approval.process call should resolve: %#v", result.Diagnostics)
	}
}

func TestProjectBodyInfersApprovalUnlockResultDespiteApprovalSObjectName(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ApprovalUnlockCaller.cls": `
public class ApprovalUnlockCaller {
    public static Approval.UnlockResult run(String recordId) {
        return Approval.unlock(recordId);
    }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("project-body Approval.unlock result should resolve: %#v", result.Diagnostics)
	}
}
