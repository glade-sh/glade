package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApprovalProjectUnsupportedRoutingMetadata(t *testing.T) {
	fixture := "testdata/conformance/approval_project_capture/project/force-app/main/default/approvalProcesses/Account.Owned_Active.approvalProcess-meta.xml"
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"other_submitter":   strings.Replace(string(raw), "<type>owner</type>", "<type>user</type>", 1),
		"other_approver":    strings.Replace(string(raw), "<name>Owner</name>", "<name>Manager</name>", 1),
		"editable_approver": strings.Replace(string(raw), "<recordEditability>AdminOnly</recordEditability>", "<recordEditability>AdminOrCurrentApprover</recordEditability>", 1),
		"delegation":        strings.Replace(string(raw), "<allowDelegate>false</allowDelegate>", "<allowDelegate>true</allowDelegate>", 1),
		"other_rejection":   strings.Replace(string(raw), "<type>RejectRequest</type>", "<type>ApproveRequest</type>", 1),
		"locked_final":      strings.Replace(string(raw), "<finalApprovalRecordLock>false</finalApprovalRecordLock>", "<finalApprovalRecordLock>true</finalApprovalRecordLock>", 1),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/Boundary.cls"), `public class Boundary {}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/approvalProcesses/Account.Boundary.approvalProcess-meta.xml"), body)
			org := OrgFromIndex(loadTestIndex(t, root))
			found := false
			for _, record := range org.Objects["ProcessDefinition"].Records {
				if record.Fields["DeveloperName"].String == "Boundary" {
					found = true
					if got := record.Fields["GladeApprovalRouting"].String; got != "unsupported" {
						t.Fatalf("routing=%q", got)
					}
				}
			}
			if !found {
				t.Fatal("definition missing")
			}
		})
	}
}
